#!/usr/bin/env bash
set -euo pipefail

REPO="${OMACY_REPO:-Glyphack/omacy}"
TAG="${OMACY_TAG:-dev}"
INSTALL_DIR="${OMACY_INSTALL_DIR:-$HOME/.local/bin}"

die() {
	echo "omacy install: $*" >&2
	exit 1
}

path_note() {
	echo "note: $INSTALL_DIR is not on the PATH of this shell"
	if [ "$INSTALL_DIR" = "$HOME/.local/bin" ]; then
		echo "omacy makes fish your login shell and puts $INSTALL_DIR on the PATH of fish"
		echo "so once omacy has finished, open a new terminal and type: omacy"
	fi
	case "${SHELL##*/}" in
	zsh) echo "to get it in zsh as well, add this line to ~/.zshrc: export PATH=\"$INSTALL_DIR:\$PATH\"" ;;
	bash) echo "to get it in bash as well, add this line to ~/.bash_profile: export PATH=\"$INSTALL_DIR:\$PATH\"" ;;
	esac
	echo "until then, start it with: $INSTALL_DIR/omacy"
}

main() {
	[ "$(uname -s)" = Darwin ] || die "this only runs on macOS, found $(uname -s)"

	[ "$(id -u)" -ne 0 ] || die "run this as yourself, not as root or with sudo. omacy asks for your password when it needs it"

	[ "$(sysctl -n hw.optional.arm64 2>/dev/null)" = 1 ] || die "omacy only runs on Apple silicon Macs"

	asset="omacy-darwin-arm64"
	url="https://github.com/$REPO/releases/download/$TAG/$asset"
	tmp="$(mktemp -d)"
	trap 'rm -rf "$tmp"' EXIT

	echo "downloading omacy $TAG for darwin/arm64"
	curl -fL --progress-bar -o "$tmp/$asset" "$url" || die "download failed from $url"
	curl -fsSL -o "$tmp/$asset.sha256" "$url.sha256" || die "download failed from $url.sha256"
	(cd "$tmp" && shasum -a 256 -c "$asset.sha256" >/dev/null) || die "the downloaded omacy does not match its checksum, try again"
	chmod +x "$tmp/$asset"

	mkdir -p "$INSTALL_DIR"
	mv "$tmp/$asset" "$INSTALL_DIR/omacy"
	rm -rf "$tmp"
	trap - EXIT
	echo "installed $INSTALL_DIR/omacy"

	case ":$PATH:" in
	*":$INSTALL_DIR:"*) ;;
	*) path_note ;;
	esac

	if [ -t 0 ]; then
		exec "$INSTALL_DIR/omacy"
	fi
	if (exec </dev/tty) 2>/dev/null; then
		exec "$INSTALL_DIR/omacy" </dev/tty
	fi
	echo "no terminal available, run it yourself with: $INSTALL_DIR/omacy"
}

main
