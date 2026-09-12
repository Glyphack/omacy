package main

import (
	"context"
	_ "embed"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

//go:embed config/mise/config.toml
var miseConfig string

func miseConfigPath() string {
	return filepath.Join(ConfigDir, "mise", "conf.d", "omacy.toml")
}

func setupMise(ctx context.Context) error {
	path := miseConfigPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create mise conf.d directory: %w", err)
	}
	if err := os.WriteFile(path, []byte(miseConfig), 0o644); err != nil {
		return fmt.Errorf("write mise config: %w", err)
	}
	slog.Debug("Wrote mise config", "file", path)
	out, err := run(exec.CommandContext(ctx, "mise", "install"))
	if err != nil {
		return fmt.Errorf("mise install: %w", err)
	}
	slog.Debug("mise tools ready", "stdout", out)

	return addMiseToPath(ctx)
}

// addMiseToPath puts the bin directories of the tools mise manages in front of PATH, so the steps
// that follow find them the way a shell that activated mise would.
func addMiseToPath(ctx context.Context) error {
	out, err := run(exec.CommandContext(ctx, "mise", "bin-paths"))
	if err != nil {
		return fmt.Errorf("mise bin-paths: %w", err)
	}

	var dirs []string
	for line := range strings.SplitSeq(out, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			dirs = append(dirs, line)
		}
	}
	if len(dirs) == 0 {
		return nil
	}

	os.Setenv("PATH", strings.Join(dirs, ":")+":"+os.Getenv("PATH"))
	slog.Debug("mise tools are on PATH", "dirs", dirs)
	return nil
}
