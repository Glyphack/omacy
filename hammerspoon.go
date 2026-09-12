package main

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

//go:embed all:config/hammerspoon/Spoons/Omacy.spoon
var hammerspoonOmacySpoon embed.FS

const hammerspoonLoadLua = `local omacy = nil
if hs.spoons.isInstalled("Omacy") then
	hs.loadSpoon("Omacy")
	omacy = spoon.Omacy
else
	print("omacy is not installed on this machine, its Hammerspoon shortcuts stay off")
end
`

const hammerspoonApplyLua = `if omacy then
	omacy:start()
end
`

const (
	hsApp         = "Hammerspoon"
	hsCLI         = "/Applications/Hammerspoon.app/Contents/Frameworks/hs/hs"
	hsCallTimeout = time.Second
	hsQuitWait    = 2 * time.Second
	hsStartWait   = 5 * time.Second
	hsPollDelay   = 500 * time.Millisecond

	loadMarker  = "-- Omacy load DO NOT EDIT"
	applyMarker = "-- Omacy apply DO NOT EDIT"
)

func putBlock(content string, marker string, body string, top bool) string {
	block := marker + "\n" + body + marker

	start := strings.Index(content, marker)
	if start >= 0 {
		end := strings.LastIndex(content, marker)
		return content[:start] + block + content[end+len(marker):]
	}

	if content == "" {
		return block + "\n"
	}
	if top {
		return block + "\n\n" + content
	}
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	return content + "\n" + block + "\n"
}

func hammerspoonDir() string {
	return filepath.Join(HomeDir, ".hammerspoon")
}

func hammerspoonOmacySpoonPath() string {
	return filepath.Join(hammerspoonDir(), "Spoons", "Omacy.spoon")
}

func hammerspoonInitPath() string {
	return filepath.Join(hammerspoonDir(), "init.lua")
}

func runHammerspoonLua(ctx context.Context, lua string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, hsCallTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, hsCLI, "-c", lua)
	cmd.WaitDelay = hsCallTimeout

	out, err := cmd.Output()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return "", fmt.Errorf("%s gave no answer within %s", hsCLI, hsCallTimeout)
	}
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return "", fmt.Errorf("%s failed: %w: %s", hsCLI, err, strings.TrimSpace(string(exitErr.Stderr)))
		}
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func checkHammerspoonPermission(ctx context.Context) (permissionStatus, error) {
	out, err := runHammerspoonLua(ctx, "print(hs.accessibilityState())")
	if err != nil {
		return permissionUnknown, err
	}
	if out == "true" {
		return permissionGranted, nil
	}
	if out == "false" {
		return permissionDenied, nil
	}
	return permissionUnknown, fmt.Errorf("hammerspoon answered %q, which is neither true nor false", out)
}

func enableHammerspoonLibrary() error {
	path := hammerspoonInitPath()
	missing, err := pathMissing(path)
	if err != nil {
		return err
	}

	content := ""
	if !missing {
		existing, readErr := os.ReadFile(path)
		if readErr != nil {
			return fmt.Errorf("error reading %s: %w", path, readErr)
		}
		content = string(existing)
	}

	content = putBlock(content, loadMarker, hammerspoonLoadLua, true)
	content = putBlock(content, applyMarker, hammerspoonApplyLua, false)

	return writeFile(path, []byte(content), 0o644)
}

func hammerspoonRunning(ctx context.Context) bool {
	_, err := run(exec.CommandContext(ctx, "pgrep", "-x", hsApp))
	return err == nil
}

func restartHammerspoon(ctx context.Context) error {
	slog.Debug("Restarting Hammerspoon")

	if hammerspoonRunning(ctx) {
		if _, err := run(exec.CommandContext(ctx, "pkill", "-x", hsApp)); err != nil {
			return fmt.Errorf("quit Hammerspoon: %w", err)
		}

		deadline := time.Now().Add(hsQuitWait)
		for hammerspoonRunning(ctx) {
			if time.Now().After(deadline) {
				return fmt.Errorf("Hammerspoon was still running %s after it was told to quit", hsQuitWait)
			}
			time.Sleep(hsPollDelay)
		}
	}

	if _, err := run(exec.CommandContext(ctx, "open", "-a", hsApp)); err != nil {
		return fmt.Errorf("start Hammerspoon: %w", err)
	}

	deadline := time.Now().Add(hsStartWait)
	var err error
	for {
		if _, err = runHammerspoonLua(ctx, "print('ready')"); err == nil {
			break
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("Hammerspoon did not answer %s within %s: %w", hsCLI, hsStartWait, err)
		}
		time.Sleep(hsPollDelay)
	}

	out, err := runHammerspoonLua(ctx, "print(spoon.Omacy and spoon.Omacy.version or \"missing\")")
	if err != nil {
		return err
	}
	if out == "missing" {
		return fmt.Errorf("Hammerspoon is running but never loaded the Omacy spoon, its console window says what the config tripped over")
	}
	return nil
}

func setupHammerspoon(context.Context) error {
	if err := copyFS(hammerspoonOmacySpoon, hammerspoonOmacySpoonPath()); err != nil {
		return err
	}
	return enableHammerspoonLibrary()
}
