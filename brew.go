package main

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

//go:embed config/Brewfile
var BrewFile string

const (
	brewPrefix       = "/opt/homebrew"
	brewInstallerURL = "https://raw.githubusercontent.com/Homebrew/install/b41c8e7b3588e2899974119faf3b2a897428648d/install.sh"
	brewBinary       = brewPrefix + "/bin/brew"
)

func installBrew(ctx context.Context) error {
	if _, err := os.Stat(brewBinary); err == nil {
		slog.Info("Homebrew is already installed, skipping", "prefix", brewPrefix)
		return nil
	}

	installer, err := run(exec.CommandContext(ctx, "curl", "-fsSL", brewInstallerURL))
	if err != nil {
		return fmt.Errorf("download the homebrew installer: %w", err)
	}

	cmd := exec.CommandContext(ctx, "bash")
	cmd.Env = append(os.Environ(), "NONINTERACTIVE=1")
	cmd.Stdin = strings.NewReader(installer)
	out, err := run(cmd)
	if err != nil {
		return fmt.Errorf("homebrew installer: %w", err)
	}
	slog.Debug("Brew installed", "stdout", out)

	if _, err := os.Stat(brewBinary); err != nil {
		return fmt.Errorf("the homebrew installer left no brew at %s: %w", brewBinary, err)
	}
	if err := os.Setenv("PATH", filepath.Join(brewPrefix, "bin")+":"+os.Getenv("PATH")); err != nil {
		return fmt.Errorf("set PATH: %w", err)
	}

	return nil
}

func brewfile() string {
	if path := os.Getenv("BREWFILE_PATH"); path != "" {
		return path
	}
	return filepath.Join(omacyDir(), "Brewfile")
}

func brewBundle(ctx context.Context, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, brewBinary, append([]string{"bundle"}, args...)...)
	cmd.Env = append(os.Environ(),
		"HOMEBREW_NO_AUTO_UPDATE=1",
		"HOMEBREW_NO_INSTALL_CLEANUP=1",
		"HOMEBREW_NO_INSTALL_UPGRADE=1",
		"HOMEBREW_NO_ANALYTICS=1",
		"HOMEBREW_NO_ENV_HINTS=1",
	)
	return cmd
}

func installBundle(ctx context.Context) error {
	install := brewBundle(ctx, "install", "--file=-", "--no-upgrade")
	install.Stdin = strings.NewReader(BrewFile)
	out, err := run(install)
	if err != nil {
		return err
	}
	slog.Debug("brew bundle install", "stdout", out)

	out, err = run(brewBundle(ctx, "dump", "--file="+brewfile(), "--force", "--no-go", "--no-cargo", "--no-uv", "--no-npm"))
	if err != nil {
		return err
	}
	slog.Debug("brew bundle dump", "stdout", out)
	return nil
}

func runAttached(cmd *exec.Cmd) error {
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w", cmd, err)
	}
	return nil
}

func runBrewCommand(ctx context.Context, name string) error {
	switch name {
	case "edit":
		return brewEdit()
	case "sync":
		return runAttached(brewBundle(ctx, "install", "--file="+brewfile(), "--force-cleanup", "--no-upgrade"))
	case "dump":
		if err := runAttached(brewBundle(ctx, "dump", "--file="+brewfile(), "--force")); err != nil {
			return err
		}
		fmt.Printf("Wrote %s\n", brewfile())
		return nil
	default:
		return errors.New("usage: omacy brew edit|sync|dump")
	}
}

// brewEdit opens the Brewfile in the user's editor and waits for them to finish.
func brewEdit() error {
	editor := strings.Fields(os.Getenv("EDITOR"))
	if len(editor) == 0 {
		return errors.New("set EDITOR and try again")
	}
	if err := runAttached(exec.Command(editor[0], append(editor[1:], brewfile())...)); err != nil {
		return err
	}
	fmt.Println("Run omacy brew sync to install and remove packages so they match the file.")
	return nil
}
