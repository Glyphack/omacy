package main

import (
	"context"
	"embed"
	"fmt"
	"os"
	"path/filepath"
)

//go:embed all:config/wezterm/omacy
var weztermOmacyModule embed.FS

const weztermLoadLua = `local omacy_config = require("omacy").load()
if omacy_config then
	return omacy_config
end
`

const weztermStarterLua = `local wezterm = require("wezterm")

local config = wezterm.config_builder()

return config
`

func weztermOmacyModulePath() string {
	return filepath.Join(ConfigDir, "wezterm", "omacy")
}

// weztermConfigPath returns the config file WezTerm reads. It reads ~/.wezterm.lua before
// ~/.config/wezterm/wezterm.lua.
func weztermConfigPath() (string, error) {
	homeConfig := filepath.Join(HomeDir, ".wezterm.lua")
	missing, err := pathMissing(homeConfig)
	if err != nil {
		return "", err
	}
	if !missing {
		return homeConfig, nil
	}
	return filepath.Join(ConfigDir, "wezterm", "wezterm.lua"), nil
}

func enableWeztermModule() error {
	path, err := weztermConfigPath()
	if err != nil {
		return err
	}
	missing, err := pathMissing(path)
	if err != nil {
		return err
	}

	content := weztermStarterLua
	if !missing {
		existing, readErr := os.ReadFile(path)
		if readErr != nil {
			return fmt.Errorf("error reading %s: %w", path, readErr)
		}
		content = string(existing)
	}

	content = putBlock(content, loadMarker, weztermLoadLua, true)

	return writeFile(path, []byte(content), 0o644)
}

func setupWezterm(context.Context) error {
	if err := copyFS(weztermOmacyModule, weztermOmacyModulePath()); err != nil {
		return err
	}
	return enableWeztermModule()
}
