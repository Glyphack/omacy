package main

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
)

type setting struct {
	option
	commands []string
}

type settingGroup struct {
	name     string
	settings []setting
}

type settingGroups []settingGroup

func (groups settingGroups) anyOn() bool {
	for _, group := range groups {
		for _, s := range group.settings {
			if s.on {
				return true
			}
		}
	}
	return false
}

type settingFailure struct {
	setting string
	command string
	// output is kept since systemsetup and a few other tools print their errors there.
	output string
	err    error
}

func (s setting) apply(ctx context.Context) []settingFailure {
	var failures []settingFailure
	for _, line := range s.commands {
		out, err := run(exec.CommandContext(ctx, "sh", "-c", line))
		if err == nil {
			continue
		}
		failures = append(failures, settingFailure{
			setting: s.help,
			command: line,
			output:  strings.TrimSpace(out),
			err:     err,
		})
	}
	return failures
}

func (groups settingGroups) apply(ctx context.Context) error {
	var failures []settingFailure
	for _, group := range groups {
		for _, s := range group.settings {
			if !s.on {
				continue
			}
			failed := s.apply(ctx)
			if len(failed) > 0 {
				failures = append(failures, failed...)
				continue
			}
			slog.Info(s.help, "group", group.name)
		}
	}
	for _, failure := range failures {
		slog.Error("Setting failed", "setting", failure.setting, "command", failure.command, "output", failure.output, "err", failure.err)
	}
	if len(failures) > 0 {
		return fmt.Errorf("%d macOS settings did not apply", len(failures))
	}
	return nil
}

func restartSettingReaders(ctx context.Context) {
	for _, app := range []string{"Finder", "SystemUIServer", "Dock", "ControlCenter"} {
		_, _ = run(exec.CommandContext(ctx, "killall", app))
	}
}
