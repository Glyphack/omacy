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

type settingsReport struct {
	applied []string
	failed  []string
}

func (groups settingGroups) apply(ctx context.Context) settingsReport {
	var report settingsReport
	for _, group := range groups {
		for _, s := range group.settings {
			if !s.on {
				continue
			}
			failures := s.apply(ctx)
			for _, failure := range failures {
				slog.Error("Setting failed", "setting", failure.setting, "command", failure.command, "output", failure.output, "err", failure.err)
			}
			if len(failures) > 0 {
				report.failed = append(report.failed, s.title)
				continue
			}
			slog.Info(s.help, "group", group.name)
			report.applied = append(report.applied, s.title)
		}
	}
	return report
}

func (r settingsReport) print() {
	for _, title := range r.applied {
		fmt.Println("Applied: " + title)
	}
	for _, title := range r.failed {
		fmt.Println("Failed: " + title)
	}
	if len(r.failed) > 0 {
		fmt.Println("Some macOS settings failed to apply. The installation log has the details.")
	}
}

func restartSettingReaders(ctx context.Context) {
	for _, app := range []string{"Finder", "SystemUIServer", "Dock", "ControlCenter"} {
		_, _ = run(exec.CommandContext(ctx, "killall", app))
	}
}
