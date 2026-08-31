#!/bin/sh
set -eu

err() {
	printf '%s\n' "$*" >&2
	exit 1
}

need() {
	command -v "$1" >/dev/null 2>&1 || err "need $1"
}

need curl
need tar
need awk
need mktemp

sha256() {
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum "$1" | awk '{print $1}'
	elif command -v shasum >/dev/null 2>&1; then
		shasum -a 256 "$1" | awk '{print $1}'
	else
		err "need sha256sum or shasum"
	fi
}

if [ -n "${GOLEM_OS:-}" ]; then
	os=$GOLEM_OS
else
	case "$(uname -s)" in
	Darwin) os=darwin ;;
	Linux) os=linux ;;
	*) err "unsupported OS $(uname -s); Golem supports macOS and Linux" ;;
	esac
fi

if [ -n "${GOLEM_ARCH:-}" ]; then
	arch=$GOLEM_ARCH
else
	case "$(uname -m)" in
	x86_64 | amd64) arch=amd64 ;;
	arm64 | aarch64) arch=arm64 ;;
	*) err "unsupported architecture $(uname -m); Golem supports amd64 and arm64" ;;
	esac
fi

case "$os" in
darwin | linux) ;;
*) err "unsupported OS $os; Golem supports macOS and Linux" ;;
esac

case "$arch" in
amd64 | arm64) ;;
*) err "unsupported architecture $arch; Golem supports amd64 and arm64" ;;
esac

github="${GOLEM_GITHUB_URL:-https://github.com}"
github=${github%/}
repo="${GOLEM_GITHUB_REPO:-terracotta4u/golem}"

if [ -n "${GOLEM_VERSION:-}" ]; then
	tag=$GOLEM_VERSION
	case "$tag" in
	v*) ;;
	*) tag="v$tag" ;;
	esac
	base="$github/$repo/releases/download/$tag"
else
	base="$github/$repo/releases/latest/download"
fi

bin_dir="${GOLEM_BIN_DIR:-$HOME/.local/bin}"

tmpdir=$(mktemp -d "${TMPDIR:-/tmp}/golem-install.XXXXXX")
trap 'rm -rf "$tmpdir"' EXIT INT TERM

printf 'Downloading Golem (%s/%s)...\n' "$os" "$arch"

curl -fsSL "$base/checksums.txt" -o "$tmpdir/checksums.txt" || err "failed to download checksums from $base/checksums.txt"

asset=""
want_hash=""
while IFS= read -r line || [ -n "$line" ]; do
	[ -n "$line" ] || continue
	hash=$(printf '%s\n' "$line" | awk '{print $1}')
	name=$(printf '%s\n' "$line" | awk '{print $2}')
	[ -n "$name" ] || continue
	case "$name" in
	golem_*_"${os}_${arch}.tar.gz") ;;
	*) continue ;;
	esac
	if [ -n "${GOLEM_VERSION:-}" ]; then
		[ "$name" = "golem_${GOLEM_VERSION#v}_${os}_${arch}.tar.gz" ] || continue
	fi
	asset=$name
	want_hash=$hash
	break
done <"$tmpdir/checksums.txt"

[ -n "$asset" ] || err "no Golem release for $os/$arch"

rest=${asset#golem_}
ver=${rest%_"${os}_${arch}.tar.gz"}

curl -fsSL "$base/$asset" -o "$tmpdir/$asset" || err "failed to download $base/$asset"

got=$(sha256 "$tmpdir/$asset")
[ "$got" = "$want_hash" ] || err "checksum mismatch for $asset"

tar -xzf "$tmpdir/$asset" -C "$tmpdir"
binary="$tmpdir/golem_${ver}_${os}_${arch}/golem"
[ -f "$binary" ] || err "archive did not contain golem"

mkdir -p "$bin_dir"
cp "$binary" "$bin_dir/golem"
chmod 755 "$bin_dir/golem"

printf 'Installed Golem %s to %s/golem\n' "$ver" "$bin_dir"
"$bin_dir/golem" version || true

case ":$PATH:" in
*":$bin_dir:"*) ;;
*)
	printf '\n%s is not on PATH. Add:\n\n  export PATH="%s:$PATH"\n\n' "$bin_dir" "$bin_dir"
	;;
esac
