#!/bin/sh
set -eu

fail() {
	echo "add-medusa-version: $*" >&2
	exit 1
}

if [ $# -ne 1 ]; then
	echo "usage: scripts/add-medusa-version.sh X.Y.Z" >&2
	exit 2
fi
version=$1
if ! printf '%s\n' "$version" | grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+$'; then
	echo "add-medusa-version: $version is not a version like 2.22.0" >&2
	exit 2
fi

root=$(cd "$(dirname "$0")/.." && pwd)
cd "$root"
versions=targets/medusa/versions
dest=$versions/$version
[ ! -e "$dest" ] || fail "$dest already exists"

newest=$(ls "$versions" | grep -E '^[0-9]+\.[0-9]+\.[0-9]+$' | sort -t. -k1,1n -k2,2n -k3,3n | tail -n 1)
[ -n "$newest" ] || fail "no version folder under $versions to copy from"

image=$(sed -n 's/^FROM \([^ ]*\).*/\1/p' targets/medusa/Dockerfile | head -n 1)
[ -n "$image" ] || fail "no FROM line in targets/medusa/Dockerfile"

run() {
	docker run --rm --user "$(id -u):$(id -g)" -e HOME=/tmp -e npm_config_update_notifier=false "$@"
}

found=$(run "$image" npm view "@medusajs/medusa@$version" version) || fail "could not ask npm for @medusajs/medusa@$version"
[ "$found" = "$version" ] || fail "@medusajs/medusa@$version is not on npm"
engines=$(run "$image" npm view "@medusajs/medusa@$version" engines.node)
[ -n "$engines" ] || fail "@medusajs/medusa@$version has no engines.node"
node=$(run "$image" node -p process.version)
if ! run -e RANGE="$engines" "$image" node -e 'const semver = require("/usr/local/lib/node_modules/npm/node_modules/semver"); process.exit(semver.satisfies(process.version, process.env.RANGE) ? 0 : 1)'; then
	fail "Medusa $version needs Node $engines, but targets/medusa/Dockerfile uses $image with Node $node. Update the Dockerfile first."
fi

mkdir "$dest"
trap 'rm -rf "$root/$dest"' EXIT

run -e VERSION="$version" -e RANGE="$engines" -v "$root/$versions/$newest:/src:ro" -v "$root/$dest:/w" "$image" node -e '
const fs = require("fs");
const v = process.env.VERSION;
const p = JSON.parse(fs.readFileSync("/src/package.json", "utf8"));
p.version = v;
for (const section of ["dependencies", "devDependencies"]) {
  for (const name of Object.keys(p[section] || {})) {
    if (name.startsWith("@medusajs/")) p[section][name] = v;
  }
}
p.engines = Object.assign({}, p.engines, { node: process.env.RANGE });
fs.writeFileSync("/w/package.json", JSON.stringify(p, null, 2) + "\n");
'

echo "making the lockfile for medusa $version in $image"
run -v "$root/$dest:/w" "$image" sh -c 'set -e; mkdir /tmp/build; cp /w/package.json /tmp/build/; cd /tmp/build; npm install --ignore-scripts --no-audit --no-fund --loglevel=error; cp package-lock.json /w/package-lock.json'

make --no-print-directory build >/dev/null
./bin/oversold versions | grep -qx "$version" || fail "oversold versions does not list $version, see the error above"

ui=$(run -v "$root/$dest:/w:ro" "$image" node -p 'const p = require("/w/package-lock.json").packages["node_modules/@medusajs/ui"]; p ? p.version : "none"')
trap - EXIT
echo "added $dest from $newest: every @medusajs package at $version, engines.node $engines, @medusajs/ui $ui in the lockfile"
