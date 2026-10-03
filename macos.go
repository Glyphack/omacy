package main

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
)

// setting is one preference of the system, the option that turns it on and the commands that set
// it. The help of the option is logged while the setting is applied.
type setting struct {
	option
	commands []string
	// optional keeps the failures of commands that fail when there is nothing left to do.
	optional bool
}

// settingGroup holds the settings of one part of the system.
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

// settingFailure is a command that did not apply its setting. The error carries what the command
// printed on standard error, output what it printed on standard output, where systemsetup and a
// few other tools put their complaints.
type settingFailure struct {
	setting string
	command string
	output  string
	err     error
}

func (s setting) apply(ctx context.Context) []settingFailure {
	var failures []settingFailure
	for _, line := range s.commands {
		out, err := run(exec.CommandContext(ctx, "sh", "-c", line))
		if err == nil || s.optional {
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

// apply runs every setting that is on, logging what each one changes, and keeps going when one of
// them fails. The error says how many did not apply, the failures themselves are logged one by one.
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

// restartSettingReaders restarts Finder, the menu bar and the dock so they read the new settings.
// An app that is not running is left alone.
func restartSettingReaders(ctx context.Context) {
	for _, app := range []string{"Finder", "SystemUIServer", "Dock", "ControlCenter"} {
		_, _ = run(exec.CommandContext(ctx, "killall", app))
	}
}
