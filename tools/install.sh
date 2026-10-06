#!/bin/sh
# Installs or updates canon from a GitHub release (DECISIONS 276 and 325, CLI.md section 7). Linux and macOS;
# Windows users download the .zip from the release page.
#   install.sh [vX.Y.Z]    default: $CANON_VERSION, else the latest release
# Environment: CANON_INSTALL_DIR (default ~/.local/bin); CANON_VERSION; CANON_RELEASE_BASE
# (default https://github.com/Fantasim/canon/releases: files under download/vX.Y.Z/, and
# latest/download/checksums.txt names the latest version; a mirror or a local test release may replace it).
set -eu

base="${CANON_RELEASE_BASE:-https://github.com/Fantasim/canon/releases}"
dir="${CANON_INSTALL_DIR:-${HOME:?HOME is not set}/.local/bin}"
ver='[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.]+)?'
semver="^v$ver\$"

die() {
	echo "install.sh: $*" >&2
	exit 1
}

fetch() { # fetch URL DEST
	if command -v curl >/dev/null 2>&1; then
		case "$1" in
		https://*) curl -fsSL --proto '=https' --proto-redir '=https' -o "$2" "$1" ;;
		*) curl -fsSL -o "$2" "$1" ;;
		esac
	elif command -v wget >/dev/null 2>&1; then
		wget -q -O "$2" "$1"
	else
		die "neither curl nor wget is installed"
	fi
}

case "$(uname -s)" in
Linux) os=linux ;;
Darwin) os=darwin ;;
*) die "unsupported system $(uname -s): Linux and macOS only; on Windows download the canon_<version>_windows_amd64.zip from $base" ;;
esac
case "$(uname -m)" in
x86_64 | amd64) arch=amd64 ;;
aarch64 | arm64) arch=arm64 ;;
*) die "unsupported architecture $(uname -m): amd64 and arm64 only" ;;
esac
if command -v sha256sum >/dev/null 2>&1; then
	sum() { sha256sum "$1"; }
elif command -v shasum >/dev/null 2>&1; then
	sum() { shasum -a 256 "$1"; }
else
	die "neither sha256sum nor shasum is installed"
fi

tmp="$(mktemp -d)"
staged=""
trap 'rm -rf "$tmp"; [ -z "$staged" ] || rm -f "$staged"' EXIT
trap 'exit 130' INT TERM

tag="${1:-${CANON_VERSION:-}}"
if [ -n "$tag" ]; then
	case "$tag" in [0-9]*) tag="v$tag" ;; esac
	[ "$(printf '%s\n' "$tag" | wc -l)" -eq 1 ] || die "bad version: want one line vX.Y.Z"
	printf '%s\n' "$tag" | grep -Eq "$semver" || die "bad version $tag: want vX.Y.Z"
else
	# The latest release's version is read from its archive name, then everything comes from
	# download/v<version>/, so a release published meanwhile cannot mix two versions.
	url="$base/latest/download/checksums.txt"
	fetch "$url" "$tmp/latest.txt" || die "cannot download $url (is there a release yet?); give a version: install.sh vX.Y.Z"
	name="$(awk '{ print $2 }' "$tmp/latest.txt" | grep -E "^canon_${ver}_${os}_${arch}\.tar\.gz\$" | head -n 1)" || true
	[ -n "$name" ] || die "$url lists no canon_<version>_${os}_$arch.tar.gz archive"
	tag="v${name#canon_}"
	tag="${tag%_"${os}"_"$arch".tar.gz}"
fi
where="$base/download/$tag"
fetch "$where/checksums.txt" "$tmp/checksums.txt" || die "cannot download $where/checksums.txt (no such release?)"
archive="canon_${tag#v}_${os}_${arch}.tar.gz"

echo "installing $archive into $dir"
fetch "$where/$archive" "$tmp/$archive" || die "cannot download $where/$archive"

want="$(awk -v f="$archive" '$2 == f { print $1 }' "$tmp/checksums.txt")"
[ -n "$want" ] || die "$archive is not listed in checksums.txt"
got="$(sum "$tmp/$archive" | awk '{ print $1 }')"
[ "$want" = "$got" ] || die "checksum mismatch for $archive: want $want, got $got"

mkdir "$tmp/x"
tar -xzf "$tmp/$archive" -C "$tmp/x" 2>/dev/null && [ -f "$tmp/x/canon" ] || die "$archive holds no canon binary"
mkdir -p "$dir"
staged="$(mktemp "$dir/.canon.new.XXXXXX")"
cp "$tmp/x/canon" "$staged"
chmod 755 "$staged"
"$staged" version >/dev/null 2>&1 || die "the downloaded canon does not run on this machine; nothing installed"
mv -f "$staged" "$dir/canon"
staged=""

"$dir/canon" version | head -n 1
case ":${PATH:-}:" in
*":$dir:"*) ;;
*) echo "warning: $dir is not on your PATH; add it, e.g. export PATH=\"$dir:\$PATH\"" >&2 ;;
esac
