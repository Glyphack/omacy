package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/cirruslabs/echelon"
	"github.com/cirruslabs/echelon/renderers"
)

var HomeDir string
var ConfigDir string
var OmacyDir string

var Verbose bool

var LogFile *os.File

func runStep(ctx context.Context, title string, action func(context.Context) error) error {
	slog.Info(title)
	if Verbose {
		return action(ctx)
	}

	renderer := renderers.NewInteractiveRenderer(os.Stdout, nil)
	go renderer.StartDrawing()
	root := echelon.NewLogger(echelon.InfoLevel, renderer)
	step := root.Scoped(title)

	err := action(ctx)

	step.Finish(err == nil)
	root.Finish(err == nil)
	renderer.StopDrawing()
	return err
}

func mustRunStep(ctx context.Context, title string, action func(context.Context) error) {
	if err := runStep(ctx, title, action); err != nil {
		fail(title+" failed", err)
	}
}

func pause(message string) {
	fmt.Println(message)
	_, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if errors.Is(err, io.EOF) {
		stopped()
	}
	if err != nil {
		fail("Cannot read from the terminal", err)
	}
}

const sudoRefreshInterval = 60 * time.Second

func becomeSudo(ctx context.Context, prompt string) error {
	cmd := exec.CommandContext(ctx, "sudo", "-v", "-p", prompt)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}

func keepSudoAlive() {
	for range time.Tick(sudoRefreshInterval) {
		_ = exec.Command("sudo", "-n", "-v").Run()
	}
}

// run returns standard output even when cmd fails, since some tools print their errors there.
func run(cmd *exec.Cmd) (string, error) {
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr

	err := cmd.Run()
	if err == nil {
		return stdout.String(), nil
	}
	if complaint := strings.TrimSpace(stderr.String()); complaint != "" {
		return stdout.String(), fmt.Errorf("%s: %w\n%s", cmd, err, complaint)
	}
	return stdout.String(), fmt.Errorf("%s: %w", cmd, err)
}

func runOpen(ctx context.Context, args ...string) {
	if _, err := run(exec.CommandContext(ctx, "open", args...)); err != nil {
		slog.Warn("Could not open "+strings.Join(args, " "), "err", err)
	}
}

func openLogFile() (*os.File, error) {
	logsDir := filepath.Join(OmacyDir, "logs")
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		return nil, fmt.Errorf("create the omacy logs directory %s: %w", logsDir, err)
	}
	return os.Create(filepath.Join(logsDir, time.Now().Format("20060102-150405")+".log"))
}

func fail(message string, err error) {
	slog.Error(message, "err", err)
	fmt.Fprintf(os.Stderr, "Something went wrong. The installation log is in %s\n", LogFile.Name())
	os.Exit(1)
}

func stopped() {
	fmt.Fprintln(os.Stderr, "Stopped, run omacy again whenever you want to go on.")
	os.Exit(130)
}

const enableAutoWrap = "\x1b[?7h"

func exitOnInterrupt() {
	interrupts := make(chan os.Signal, 1)
	signal.Notify(interrupts, os.Interrupt)
	go func() {
		<-interrupts
		fmt.Print(enableAutoWrap + "\n")
		stopped()
	}()
}

const levelNameWidth = 5

// logStyles shows full level names, since the default styles cut them to four letters.
func logStyles() *log.Styles {
	styles := log.DefaultStyles()
	for level, style := range styles.Levels {
		styles.Levels[level] = style.Width(levelNameWidth).MaxWidth(levelNameWidth)
	}
	return styles
}

func main() {
	exitOnInterrupt()

	args := parseArgs(os.Args[1:])
	Verbose = args.verbose

	var err error
	HomeDir, err = os.UserHomeDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, "find home directory:", err)
		os.Exit(1)
	}
	ConfigDir = filepath.Join(HomeDir, ".config")
	OmacyDir = filepath.Join(ConfigDir, "omacy")

	LogFile, err = openLogFile()
	if err != nil {
		fmt.Fprintln(os.Stderr, "open the log file:", err)
		os.Exit(1)
	}
	defer func() { _ = LogFile.Close() }()

	logOutput := io.Writer(LogFile)
	if Verbose {
		logOutput = io.MultiWriter(os.Stdout, LogFile)
	}
	h := log.NewWithOptions(logOutput, log.Options{Level: log.DebugLevel})
	h.SetStyles(logStyles())
	slog.SetDefault(slog.New(h))

	ctx := context.Background()

	if args.command == "brew" {
		if err := runBrewCommand(ctx, args.brewCommand); err != nil {
			fail("omacy brew failed", err)
		}
		return
	}

	var config Config
	if args.noInteractive {
		config = args.config(configProfiles[args.profile])
	} else {
		config = runConfigForm(args)
	}
	install(ctx, config)
}

func install(ctx context.Context, config Config) {
	slog.Info("Setting up", "profile", config.profile)
	defer fmt.Printf("The installation log is in %s\n", LogFile.Name())

	if err := addBrewToPath(); err != nil {
		fail("Cannot put Homebrew on PATH", err)
	}

	if err := becomeSudo(ctx, "Enter your password:"); err != nil {
		fail("Could not become sudo user.", err)
	}
	go keepSudoAlive()

	if config.install.brew.on {
		mustRunStep(ctx, "Installing Homebrew", installBrew)

		// sudo only asks again when the Homebrew installer made it forget the password.
		if err := becomeSudo(ctx, "Enter your password again:"); err != nil {
			fail("Could not become sudo user.", err)
		}

		mustRunStep(ctx, "Installing applications", installBundle)
	}
	if config.install.mise.on {
		mustRunStep(ctx, "Mise Configure & Install", setupMise)
	}

	if config.configure.defaultShell.on {
		if err := setFishAsDefault(ctx); err != nil {
			fail("Set fish as default failed", err)
		}
		slog.Info("Set fish shell as default shell")
	}
	if config.configure.fish.on {
		mustRunStep(ctx, "Configuring fish", setupFish)
	}
	if config.configure.wezterm.on {
		mustRunStep(ctx, "Configuring WezTerm", setupWezterm)
	}

	if config.macOS.anyOn() {
		var report settingsReport
		mustRunStep(ctx, "Applying macOS settings", func(ctx context.Context) error {
			report = config.macOS.apply(ctx)
			return nil
		})
		report.print()
		restartSettingReaders(ctx)
		fmt.Println("Some settings only take effect after you log out or restart.")
	}

	if config.configure.karabiner.config.on {
		mustRunStep(ctx, "Configuring Karabiner", setupKarabiner)
	}
	permissionRequest{
		app:         "Karabiner",
		appPath:     karabinerAppPath,
		wait:        config.configure.karabiner.permissionSetup.on,
		check:       karabinerPermission,
		openArgs:    []string{"-a", "Karabiner-Elements"},
		openMessage: "Press Enter to open Karabiner. Follow the instructions in the Karabiner app to give it permissions, then come back.",
		doneMessage: "Press Enter once Karabiner is set up",
	}.ask(ctx)

	if config.configure.hammerspoon.config.on {
		mustRunStep(ctx, "Configuring Hammerspoon", setupHammerspoon)
	}
	permissionRequest{
		app:         "Hammerspoon",
		appPath:     hsAppPath,
		wait:        config.configure.hammerspoon.permissionSetup.on,
		check:       hammerspoonPermission,
		openArgs:    []string{"x-apple.systempreferences:com.apple.preference.security?Privacy_Accessibility"},
		openMessage: "Press Enter to open the accessibility settings. Turn Hammerspoon on in that list and then come back here.",
		doneMessage: "Press Enter once Hammerspoon is turned on in the accessibility list",
	}.ask(ctx)

	runOpen(ctx, "-a", "MonitorControl")
}
