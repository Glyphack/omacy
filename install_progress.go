package main

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/term"
)

type bundleApplication struct{ name, entry string }

// The embedded Brewfile contains one declarative application per line.
func bundleApplications(file string) []bundleApplication {
	var apps []bundleApplication
	for line := range strings.SplitSeq(file, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "\"", 3)
		name := line
		if len(parts) == 3 {
			name = parts[1]
		}
		apps = append(apps, bundleApplication{name, line})
	}
	return apps
}

type installProgress struct {
	mu               sync.Mutex
	total, completed int
	label            string
	started          time.Time
	interactive      bool
	finished         bool
	done, stopped    chan struct{}
}

func newInstallProgress(total int) *installProgress {
	p := &installProgress{total: total, label: "Preparing applications", started: time.Now(),
		interactive: !Verbose && term.IsTerminal(os.Stdout.Fd()),
		done:        make(chan struct{}), stopped: make(chan struct{})}
	go func() {
		defer close(p.stopped)
		ticker := time.NewTicker(150 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				p.draw()
			case <-p.done:
				p.draw()
				if p.interactive {
					fmt.Println()
				}
				return
			}
		}
	}()
	return p
}

func (p *installProgress) update(completed int, label string) {
	p.mu.Lock()
	p.completed, p.label = completed, label
	p.mu.Unlock()
	slog.Info(label, "completed", completed, "total", p.total)
	if !p.interactive && !Verbose {
		if p.total == 0 {
			fmt.Println(label)
		} else {
			fmt.Printf("[%d/%d] %s\n", completed, p.total, label)
		}
	}
}

func (p *installProgress) draw() {
	if !p.interactive {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	width, _, err := term.GetSize(os.Stdout.Fd())
	if err != nil {
		width = 80
	}
	barWidth := min(24, max(4, width/4))
	filled, percent := 0, 0
	if p.total > 0 {
		filled = barWidth * p.completed / p.total
		percent = 100 * p.completed / p.total
	}
	bar := lipgloss.NewStyle().Foreground(lipgloss.Color("#9F9CFF")).Render(strings.Repeat("━", filled)) +
		lipgloss.NewStyle().Foreground(lipgloss.Color("#555555")).Render(strings.Repeat("─", barWidth-filled))
	frames := []string{"🕛", "🕐", "🕑", "🕒", "🕓", "🕔", "🕕", "🕖", "🕗", "🕘", "🕙", "🕚"}
	elapsed := time.Since(p.started)
	icon := frames[int(elapsed/(150*time.Millisecond))%len(frames)]
	if p.finished || p.label == "Applications ready" {
		icon = "✅"
	}
	if strings.HasPrefix(p.label, "Failed") {
		icon = "❌"
	}
	line := fmt.Sprintf("%s %s %3d%% %d/%d · %s · %s", icon, bar, percent, p.completed, p.total, p.label, elapsed.Round(time.Second))
	if p.total == 0 {
		line = fmt.Sprintf("%s %s · %s", icon, p.label, elapsed.Round(time.Second))
	}
	fmt.Print("\r\x1b[2K" + lipgloss.NewStyle().MaxWidth(max(1, width-1)).Render(line))
}

func (p *installProgress) stop() {
	close(p.done)
	<-p.stopped
}
