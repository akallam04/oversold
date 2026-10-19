#!/bin/sh

fail=0

row() {
	printf '%-16s %s\n' "$1" "$2"
}

missing() {
	row "$1" "MISSING ($2)"
	fail=1
}

if command -v go >/dev/null 2>&1; then
	v=$(go env GOVERSION)
	row go "$v"
	case "$v" in
	go1.2[7-9]* | go1.[3-9][0-9]* | go[2-9]*) ;;
	*) missing go "need 1.27 or newer" ;;
	esac
else
	missing go "need 1.27 or newer"
fi

if command -v docker >/dev/null 2>&1; then
	row docker "$(docker version --format '{{.Client.Version}}' 2>/dev/null)"
	if docker compose version >/dev/null 2>&1; then
		row "docker compose" "$(docker compose version --short)"
	else
		missing "docker compose" "need Compose v2"
	fi
	info=$(docker info --format '{{.NCPU}} {{.MemTotal}}' 2>/dev/null)
	if [ -n "$info" ]; then
		cpus=${info% *}
		mem=${info#* }
		mem_mb=$((mem / 1024 / 1024))
		row "docker daemon" "running, $cpus CPUs, $mem_mb MiB memory"
		if [ "$mem_mb" -lt 4096 ]; then
			row "" "WARNING: Docker has less than 4 GB of memory"
		fi
	else
		missing "docker daemon" "not running"
	fi
else
	missing docker "not installed"
fi

if command -v git >/dev/null 2>&1; then
	row git "$(git --version | sed 's/^git version //')"
else
	missing git "not installed"
fi

if command -v make >/dev/null 2>&1; then
	row make "$(make --version | head -n 1)"
else
	missing make "not installed"
fi

gitleaks_bin=$(command -v gitleaks 2>/dev/null)
if [ -z "$gitleaks_bin" ] && [ -x "$HOME/.local/bin/gitleaks" ]; then
	gitleaks_bin="$HOME/.local/bin/gitleaks"
fi
if [ -n "$gitleaks_bin" ]; then
	row gitleaks "$("$gitleaks_bin" version)"
else
	missing gitleaks "not installed"
fi

if command -v gh >/dev/null 2>&1; then
	row gh "$(gh --version | head -n 1 | sed 's/^gh version //')"
	account=$(gh api user --jq .login 2>/dev/null)
	if [ -n "$account" ]; then
		row "gh account" "$account"
	else
		row "gh account" "not logged in (run gh auth login)"
	fi
else
	row gh "not installed (optional for local work)"
fi

if command -v node >/dev/null 2>&1; then
	row node "$(node --version) (optional)"
else
	row node "not installed (optional)"
fi

if [ "$fail" -ne 0 ]; then
	echo "required tools are missing or too old" >&2
	exit 1
fi
