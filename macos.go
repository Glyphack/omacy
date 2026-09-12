package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"

	"github.com/charmbracelet/huh"
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

// confirmMacSettings asks before anything on the system is touched.
func confirmMacSettings() (bool, error) {
	apply := true
	err := huh.NewConfirm().
		Title("Apply the omacy macOS settings?").
		Description("The firewall, trackpad, screen, Finder and dock settings").
		Value(&apply).
		Run()
	if errors.Is(err, huh.ErrUserAborted) {
		return false, errStopped
	}
	return apply, err
}

// applyMacOSSettings runs every setting, printing what each one does, and keeps going when one of
// them fails.
func applyMacOSSettings(ctx context.Context) []settingFailure {
	var failures []settingFailure
	for _, group := range macOSSettingGroups() {
		fmt.Printf("\n%s\n", group.name)
		for _, s := range group.settings {
			failed := s.apply(ctx)
			state := "ok"
			if len(failed) > 0 {
				state = "failed"
			}
			fmt.Printf("  %-6s %s\n", state, s.explanation)
			failures = append(failures, failed...)
		}
	}
	fmt.Println()
	return failures
}

func reportSettingFailures(failures []settingFailure) {
	for _, failure := range failures {
		slog.Error("Setting failed", "setting", failure.setting, "command", failure.command, "output", failure.output, "err", failure.err)
	}
}
