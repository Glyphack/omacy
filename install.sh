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

	[ "$(uname -m)" = arm64 ] || die "omacy only runs on Apple silicon Macs, found $(uname -m)"

	url="https://github.com/$REPO/releases/download/$TAG/omacy-darwin-arm64"
	tmp="$(mktemp -d)"
	trap 'rm -rf "$tmp"' EXIT

	echo "downloading omacy $TAG for darwin/arm64"
	curl -fL --progress-bar -o "$tmp/omacy" "$url" || die "download failed from $url"
	chmod +x "$tmp/omacy"

	mkdir -p "$INSTALL_DIR"
	mv "$tmp/omacy" "$INSTALL_DIR/omacy"
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
