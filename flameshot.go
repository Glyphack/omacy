package main

import (
	"context"
	_ "embed"
	"fmt"
	"log/slog"
	"os/exec"
	"path/filepath"
)

//go:embed config/flameshot/flameshot.ini
var flameshotConfig string

func flameshotConfigPath() string {
	return filepath.Join(ConfigDir, "flameshot", "flameshot.ini")
}

// setupFlameshot writes the omacy flameshot config before flameshot is installed, so the shortcut
// is already in place the first time flameshot runs instead of the app's own defaults. It leaves
// an existing config alone rather than overwriting whatever the user has changed since.
func setupFlameshot(ctx context.Context) error {
	path := flameshotConfigPath()
	missing, err := pathMissing(path)
	if err != nil {
		return err
	}
	if missing {
		if err := writeFile(path, []byte(flameshotConfig), 0o644); err != nil {
			return err
		}
	} else {
		slog.Debug("flameshot config already exists, leaving it alone", "file", path)
	}

	out, err := run(exec.CommandContext(ctx, "brew", "install", "--cask", "flameshot"))
	if err != nil {
		return fmt.Errorf("brew install flameshot: %w", err)
	}
	slog.Debug("flameshot installed", "stdout", out)
	return nil
}
