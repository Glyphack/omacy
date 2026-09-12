package main

import (
	"context"
	"embed"
	"path/filepath"
)

//go:embed all:config/wezterm/omacy
var weztermOmacyModule embed.FS

func weztermOmacyModulePath() string {
	return filepath.Join(ConfigDir, "wezterm", "omacy")
}

func setupWezterm(context.Context) error {
	return copyFS(weztermOmacyModule, weztermOmacyModulePath())
}
