package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/log"
	"github.com/cirruslabs/echelon"
	"github.com/cirruslabs/echelon/renderers"
	"golang.org/x/term"
)

const levelNameWidth = 5

var HomeDir string
var ConfigDir string

var Verbose bool

// LogFile is the run's log file, or nil if it could not be opened. Everything omacy logs while it
// runs is written here.
var LogFile *os.File

// newProgressRenderer draws the step on a terminal and falls back to plain lines anywhere else. The
// second return value stops the drawing.
func newProgressRenderer() (echelon.LogRendered, func()) {
	if !term.IsTerminal(int(os.Stdout.Fd())) {
		return renderers.NewSimpleRenderer(os.Stdout, nil), func() {}
	}
	renderer := renderers.NewInteractiveRenderer(os.Stdout, nil)
	go renderer.StartDrawing()
	return renderer, renderer.StopDrawing
}

func runStep(ctx context.Context, title string, action func(context.Context) error) error {
	slog.Info(title)
	if Verbose {
		return action(ctx)
	}

	renderer, stopDrawing := newProgressRenderer()
	root := echelon.NewLogger(echelon.InfoLevel, renderer)
	step := root.Scoped(title)

	err := action(ctx)

	step.Finish(err == nil)
	root.Finish(err == nil)
	stopDrawing()
	return err
}

const (
	keyCtrlC = 0x03
	keyCtrlD = 0x04
)

var errStopped = errors.New("stopped by the user, run omacy again whenever you want to go on")

func waitForAnyKey() error {
	fd := int(os.Stdin.Fd())
	state, err := term.MakeRaw(fd)
	if err != nil {
		return err
	}
	defer func() { _ = term.Restore(fd, state) }()

	key := make([]byte, 1)
	if _, err := os.Stdin.Read(key); err != nil {
		return err
	}
	if key[0] == keyCtrlC || key[0] == keyCtrlD {
		return errStopped
	}
	return nil
}

const sudoRefreshInterval = 60 * time.Second

type sudoKeepAlive struct {
	cancel context.CancelFunc
	done   chan struct{}
}

func becomeSudo(ctx context.Context, prompt string) error {
	if os.Geteuid() == 0 {
		return nil
	}
	args := []string{"-v", "-p", prompt, "-v"}
	cmd := exec.CommandContext(ctx, "sudo", args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}

// startSudoKeepAlive refreshes the sudo timestamp in the background, so it does not expire while a
// step is running. It does nothing when already root.
func startSudoKeepAlive(ctx context.Context) *sudoKeepAlive {
	if os.Geteuid() == 0 {
		return nil
	}
	ctx, cancel := context.WithCancel(ctx)
	keepAlive := &sudoKeepAlive{cancel: cancel, done: make(chan struct{})}
	go keepAlive.run(ctx)
	return keepAlive
}

func (k *sudoKeepAlive) run(ctx context.Context) {
	defer close(k.done)

	ticker := time.NewTicker(sudoRefreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = exec.CommandContext(ctx, "sudo", "-n", "-v").Run()
		}
	}
}

// stop ends the renewal and waits for it to finish.
func (k *sudoKeepAlive) stop() {
	if k == nil {
		return
	}
	k.cancel()
	<-k.done
}

// run waits for cmd and returns what it printed on standard output. The command prints nothing on
// the terminal. A command that fails carries its command line and what it printed on standard error
// in the error, and its standard output is still returned for the callers that read the complaints
// tools leave there.
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

func omacyDir() string {
	return filepath.Join(HomeDir, ".omacy")
}

func makeOmacyDir() error {
	if err := os.MkdirAll(omacyDir(), 0o755); err != nil {
		return fmt.Errorf("create the omacy directory %s: %w", omacyDir(), err)
	}
	return nil
}

// openLogFile creates a fresh log file under ~/.omacy/logs for this run, named after the time it
// started, and returns it opened for writing.
func openLogFile() (*os.File, error) {
	logsDir := filepath.Join(omacyDir(), "logs")
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		return nil, fmt.Errorf("create the omacy logs directory %s: %w", logsDir, err)
	}
	path := filepath.Join(logsDir, time.Now().Format("20060102-150405")+".log")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("create the log file %s: %w", path, err)
	}
	return file, nil
}

// exit tells the user something went wrong and where to find the installation log, then exits with
// status 1.
func exit() {
	if LogFile != nil {
		fmt.Fprintf(os.Stderr, "Something went wrong. The installation log is in %s\n", LogFile.Name())
	}
	os.Exit(1)
}

// logStyles spells the level names out in full and pads them to the same width, where the default
// styles cut every name down to four letters.
func logStyles() *log.Styles {
	styles := log.DefaultStyles()
	for level, style := range styles.Levels {
		styles.Levels[level] = style.Width(levelNameWidth).MaxWidth(levelNameWidth)
	}
	return styles
}

func main() {
	installAppsOnly := flag.Bool("installAppsOnly", false, "Only install apps without configuration")
	verbose := flag.Bool("verbose", false, "Print every step as it runs instead of showing a spinner")
	flag.Parse()
	Verbose = *verbose

	var err error
	HomeDir, err = os.UserHomeDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, "find home directory:", err)
		os.Exit(1)
	}
	ConfigDir = filepath.Join(HomeDir, ".config")

	logOutput := io.Writer(os.Stdout)
	if LogFile, err = openLogFile(); err != nil {
		fmt.Fprintln(os.Stderr, "could not open a log file, continuing without one:", err)
	} else {
		defer LogFile.Close()
		logOutput = LogFile
		if Verbose {
			logOutput = io.MultiWriter(os.Stdout, LogFile)
		}
	}

	h := log.NewWithOptions(logOutput, log.Options{Level: log.DebugLevel})
	h.SetStyles(logStyles())
	slog.SetDefault(slog.New(h))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	switch flag.Arg(0) {
	case "":
	case "brew":
		if err := runBrewCommand(ctx, flag.Arg(1)); err != nil {
			h.Error("omacy brew failed", "err", err)
			exit()
		}
		return
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", flag.Arg(0))
		flag.Usage()
		os.Exit(2)
	}

	if err := makeOmacyDir(); err != nil {
		h.Error("Cannot create the omacy directory", "err", err)
		exit()
	}

	if err := becomeSudo(ctx, "Enter your password:"); err != nil {
		h.Error("Could not become sudo user.", "err", err)
		exit()
	}

	if err := runStep(ctx, "Installing Homebrew", installBrew); err != nil {
		h.Error("Installing Homebrew failed", "err", err)
		exit()
	}

	if err := runStep(ctx, "Installing applications", installBundle); err != nil {
		h.Error("Installing applications failed", "err", err)
		exit()
	}

	if err := runStep(ctx, "Mise Configure & Install", setupMise); err != nil {
		h.Error("Configuring mise failed", "err", err)
		exit()
	}

	if *installAppsOnly {
		return
	}

	if err := becomeSudo(ctx, "Enter your password again: "); err != nil {
		h.Error("Could not become sudo user.", "err", err)
		exit()
	}
	keepAlive := startSudoKeepAlive(ctx)
	defer keepAlive.stop()

	if err := setFishAsDefault(ctx); err != nil {
		h.Error("Set fish as default failed", "err", err)
		exit()
	}
	slog.Info("Set fish shell as default shell")

	if err := runStep(ctx, "Configuring fish", setupFish); err != nil {
		h.Error("Configuring fish failed", "err", err)
		exit()
	}

	if err := runStep(ctx, "Configuring WezTerm", setupWezterm); err != nil {
		h.Error("Configuring WezTerm failed", "err", err)
		exit()
	}

	applySettings := true
	err = huh.NewConfirm().
		Title("Apply the omacy macOS settings?").
		Description("The firewall, trackpad, screen, Finder and dock settings").
		Value(&applySettings).
		Run()
	if err != nil {
		h.Error("Stopped", "err", err)
		exit()
	}
	if applySettings {
		if err := runStep(ctx, "Applying macOS settings", applyMacOSSettings); err != nil {
			slog.Warn("Some macOS settings did not apply", "err", err)
		}
	} else {
		slog.Info("Skipped the macOS settings")
	}

	if err := runStep(ctx, "Configuring Karabiner", setupKarabiner); err != nil {
		h.Error("Configuring Karabiner failed", "err", err)
		exit()
	}

	karabinerStatus, karabinerErr := karabinerPermission(ctx)
	t := "Karabiner Permissions are granted"
	if karabinerStatus == permissionGranted {
		slog.Info(t)
	} else {
		fmt.Println("Press any key to open Karabiner. Follow the instruction in Karabiner app and give it permissions then come back.")

		if err := waitForAnyKey(); err != nil {
			h.Error("Stopped", "err", err)
			exit()
		}
		runOpen(ctx, "-a", "Karabiner-Elements")
		karabinerStatus, karabinerErr = karabinerPermission(ctx)
		if karabinerStatus == permissionGranted {
			slog.Info("Karabiner has all its permissions")
			fmt.Println(t)
		}
		if karabinerStatus == permissionUnknown {
			slog.Warn("Cannot ask Karabiner about its permissions", "err", karabinerErr)
			fmt.Println("Karabiner permissions are not granted. Open the app and enable it later yourself.")
		}

	}

	if err := runStep(ctx, "Configuring Hammerspoon", setupHammerspoon); err != nil {
		h.Error("Configuring Hammerspoon failed", "err", err)
		exit()
	}

	if err := restartHammerspoon(ctx); err != nil {
		slog.Warn("Hammerspoon is not running the config omacy wrote, so its shortcuts and the hs command stay off until it starts", "err", err)
	}

	hammerspoonStatus, hammerspoonErr := checkHammerspoonPermission(ctx)
	hammerspoonGranted := "Hammerspoon Permissions are granted"
	if hammerspoonStatus == permissionGranted {
		slog.Info(hammerspoonGranted)
	} else {
		fmt.Println("Press any key to open the accessibility settings. Turn Hammerspoon on in that list and then come back here.")

		if err := waitForAnyKey(); err != nil {
			h.Error("Stopped", "err", err)
			exit()
		}
		runOpen(ctx, "x-apple.systempreferences:com.apple.preference.security?Privacy_Accessibility")

		fmt.Println("Press any key once Hammerspoon is turned on in the accessibility list")
		if err := waitForAnyKey(); err != nil {
			h.Error("Stopped", "err", err)
			exit()
		}
		if err := restartHammerspoon(ctx); err != nil {
			slog.Warn("Could not restart hammerspoon", "err", err)
		}
		hammerspoonStatus, hammerspoonErr = checkHammerspoonPermission(ctx)
		if hammerspoonStatus == permissionGranted {
			slog.Info(hammerspoonGranted)
			fmt.Println(hammerspoonGranted)
		}
		if hammerspoonStatus == permissionUnknown {
			slog.Warn("Cannot ask Hammerspoon about its permissions", "err", hammerspoonErr)
			fmt.Println("Hammerspoon permissions are not granted. Open the app and enable it later yourself.")
		}
	}
}
