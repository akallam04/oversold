#!/bin/sh
set -eu

usage() {
	echo "usage: scripts/build-variant.sh <variant> [--write]" >&2
	exit 2
}

[ $# -ge 1 ] && [ $# -le 2 ] || usage
name=$1
write=no
if [ $# -eq 2 ]; then
	[ "$2" = "--write" ] || usage
	write=yes
fi

cd "$(dirname "$0")/.."
root=$(pwd)
for tool in docker git go; do
	command -v "$tool" >/dev/null || { echo "$tool is required" >&2; exit 1; }
done
docker info >/dev/null 2>&1 || { echo "docker is not running" >&2; exit 1; }
make build >/dev/null

status=0
info=$(./bin/oversold variant "$name") || status=$?
if [ "$status" -ne 0 ] && [ "$write" = no ]; then
	exit "$status"
fi
field() {
	printf '%s\n' "$info" | awk -v k="$1" '$1 == k { print $2 }'
}
base=$(field base_version)
image=$(field base_image)
pr=$(field pr)
head=$(field head_sha)
parent=$(field base_sha)
sources=$(printf '%s\n' "$info" | awk '$1 == "source_file" { print $2 }')
files=$(printf '%s\n' "$info" | awk '$1 == "file" { print $2 }')
for v in "$base" "$image" "$pr" "$head" "$parent" "$sources" "$files"; do
	case "$v" in
	"" | -) echo "variant $name: variant.yaml is missing base_version, source, source_files or files" >&2; exit 2 ;;
	esac
done
docker image inspect "$image" >/dev/null 2>&1 || {
	echo "image $image not found; run any scenario with --version $base once to build it" >&2
	exit 1
}

repo=${pr#https://github.com/}
repo=${repo%/pull/*}
cache=${OVERSOLD_CACHE:-$root/.cache}/src/$(printf '%s' "$repo" | tr / _)
if [ ! -d "$cache/.git" ]; then
	mkdir -p "$cache"
	git -C "$cache" init -q
	git -C "$cache" remote add origin "https://github.com/$repo.git"
fi
echo "fetching $repo v$base, $parent and $head into $cache"
git -C "$cache" fetch -q --depth 1 --filter=blob:none origin "refs/tags/v$base:refs/tags/v$base"
git -C "$cache" fetch -q --depth 1 --filter=blob:none origin "$parent" "$head"
git -C "$cache" cat-file -e "$head^{commit}"
git -C "$cache" cat-file -e "$parent^{commit}"

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

packages=$(printf '%s\n' "$sources" | sed 's#/src/.*##' | sort -u)
for tree in unpatched patched; do
	mkdir -p "$work/$tree" "$work/out-$tree"
	git -C "$cache" archive "v$base" package.json yarn.lock _tsconfig.base.json $packages | tar -x -C "$work/$tree"
done
git -C "$cache" diff "$parent" "$head" -- $sources > "$work/pr.diff"
[ -s "$work/pr.diff" ] || { echo "the pull request does not change $sources" >&2; exit 1; }
if ! git -C "$work/patched" apply --check "$work/pr.diff" 2>"$work/apply.err"; then
	echo "the pull request's source change does not apply cleanly to v$base:" >&2
	cat "$work/apply.err" >&2
	exit 1
fi
git -C "$work/patched" apply "$work/pr.diff"
echo "applied $(grep -c '^diff --git' "$work/pr.diff") files from $parent..$head to v$base"

for tree in unpatched patched; do
	echo "compiling the $tree packages"
	docker run --rm --user "$(id -u):$(id -g)" -e HOME=/tmp -e npm_config_update_notifier=false \
		-v "$work/$tree:/src:ro" -v "$work/out-$tree:/out" -v "$root/scripts/variant-compile.js:/variant-compile.js:ro" \
		--entrypoint node "$image" /variant-compile.js /src /out $packages >"$work/compile-$tree.log" 2>&1 || {
		cat "$work/compile-$tree.log" >&2
		exit 1
	}
done
grep '^versions from' "$work/compile-unpatched.log"

docker run --rm -v "$work:/work:ro" --entrypoint sh "$image" -c '
	cd /work/out-unpatched
	total=0; differ=0; missing=0
	for f in $(find . -name "*.js" | sort); do
		published=/server/node_modules/${f#./}
		if [ ! -f "$published" ]; then
			missing=$((missing + 1))
			continue
		fi
		total=$((total + 1))
		cmp -s "$f" "$published" || { differ=$((differ + 1)); echo "differs from the published file: ${f#./}"; }
	done
	echo "method check: $total compiled .js files have a published counterpart, $differ differ; $missing compiled files are not published"
	for pkg in $(ls -d */* 2>/dev/null); do
		count=$(cd "/server/node_modules/$pkg/dist" && find . -name "*.js" | wc -l)
		mine=$(cd "$pkg/dist" && find . -name "*.js" | while read -r f; do [ -f "/server/node_modules/$pkg/dist/$f" ] && echo "$f"; done | wc -l)
		[ "$count" -eq "$mine" ] || { echo "published $pkg has $count .js files, only $mine were compiled"; differ=$((differ + 1)); }
	done
	[ "$differ" -eq 0 ]
' || { echo "method check failed: the unpatched compile does not reproduce the published files in $image" >&2; exit 1; }

changed=$(cd "$work" && for f in $(cd out-patched && find . -name '*.js' | sort); do cmp -s "out-patched/$f" "out-unpatched/$f" || echo "${f#./}"; done)
listed=$(printf '%s\n' "$files" | sort)
if [ "$(printf '%s\n' "$changed" | sort)" != "$listed" ]; then
	echo "the patch changes these compiled files:" >&2
	printf '  %s\n' $changed >&2
	echo "variant.yaml lists:" >&2
	printf '  %s\n' $listed >&2
	exit 1
fi

fail=0
for f in $files; do
	got=$(sha256sum "$work/out-patched/$f" 2>/dev/null || shasum -a 256 "$work/out-patched/$f")
	got=${got%% *}
	replaces=$(docker run --rm --entrypoint sha256sum "$image" "/server/node_modules/$f")
	replaces=${replaces%% *}
	want=$(printf '%s\n' "$info" | awk -v p="$f" '$1 == "file" && $2 == p { print $3 }')
	want_replaces=$(printf '%s\n' "$info" | awk -v p="$f" '$1 == "file" && $2 == p { print $4 }')
	dest="targets/medusa/variants/$name/files/$f"
	if [ "$write" = yes ]; then
		mkdir -p "$(dirname "$dest")"
		cp "$work/out-patched/$f" "$dest"
		echo "wrote $dest"
		echo "  - path: \"$f\""
		echo "    replaces_sha256: $replaces"
		echo "    sha256: $got"
		continue
	fi
	if [ "$got" = "$want" ] && [ "$replaces" = "$want_replaces" ] && cmp -s "$work/out-patched/$f" "$dest"; then
		echo "ok $f sha256 $got, replaces $replaces"
	else
		echo "MISMATCH $f: built sha256 $got (variant.yaml $want), replaces $replaces (variant.yaml $want_replaces)" >&2
		fail=1
	fi
done
[ "$write" = yes ] && echo "update the files entries in targets/medusa/variants/$name/variant.yaml with the lines above"
exit "$fail"
