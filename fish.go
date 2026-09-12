package main

import (
	"context"
	"embed"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
)

//go:embed all:config/fish/omacy
var fishOmacyLibrary embed.FS

//go:embed config/fish/conf.d/omacy.fish
var fishOmacyLoader []byte

func fishConfigDir() string {
	return filepath.Join(ConfigDir, "fish")
}

func fishOmacyLibPath() string {
	return filepath.Join(fishConfigDir(), "omacy")
}

func fishOmacyConfigPath() string {
	return filepath.Join(fishConfigDir(), "conf.d", "omacy.fish")
}

func loginShell() (string, error) {
	u, err := user.Current()
	if err != nil {
		return "", err
	}
	out, err := exec.Command("dscl", ".", "-read", "/Users/"+u.Username, "UserShell").Output()
	if err != nil {
		return "", fmt.Errorf("read login shell: %w", err)
	}
	fields := strings.Fields(string(out))
	if len(fields) < 2 {
		return "", fmt.Errorf("unexpected dscl output: %q", out)
	}
	return fields[len(fields)-1], nil
}

func setFishAsDefault(ctx context.Context) error {
	fishPath := filepath.Join(brewPrefix, "bin", "fish")

	current, err := loginShell()
	if err != nil {
		return err
	}
	if current == fishPath {
		slog.Debug("Shell is already fish")
		return nil
	}

	if _, err := os.Stat(fishPath); err != nil {
		return fmt.Errorf("fish not found at %s: %w", fishPath, err)
	}

	u, err := user.Current()
	if err != nil {
		return err
	}
	shellScript := fmt.Sprintf(
		`grep -qxF %[1]q /etc/shells || echo %[1]q >> /etc/shells; chsh -s %[1]q %[2]q`,
		fishPath, u.Username)

	cmd := exec.CommandContext(ctx, "sudo", "sh", "-c", shellScript)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("set fish as login shell: %w", err)
	}
	return nil
}

func setupFish(context.Context) error {
	if err := copyFS(fishOmacyLibrary, fishOmacyLibPath()); err != nil {
		return err
	}

	return writeFile(fishOmacyConfigPath(), fishOmacyLoader, 0o644)
}
