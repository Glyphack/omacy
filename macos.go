package main

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
)

// setting is one preference of the system, the sentence printed while it is applied and the
// commands that set it.
type setting struct {
	explanation string
	commands    []string
	// optional keeps the failures of commands that fail when there is nothing left to do.
	optional bool
}

// settingGroup holds the settings of one part of the system.
type settingGroup struct {
	name     string
	settings []setting
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
			setting: s.explanation,
			command: line,
			output:  strings.TrimSpace(out),
			err:     err,
		})
	}
	return failures
}

// applyMacOSSettings runs every setting, logging what each one changes, and keeps going when one of
// them fails. The error says how many did not apply, the failures themselves are logged one by one.
func applyMacOSSettings(ctx context.Context) error {
	var failures []settingFailure
	for _, group := range macOSSettingGroups() {
		for _, s := range group.settings {
			failed := s.apply(ctx)
			if len(failed) > 0 {
				failures = append(failures, failed...)
				continue
			}
			slog.Info(s.explanation, "group", group.name)
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
