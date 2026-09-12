package main

import (
	"context"
	_ "embed"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/huh/spinner"
	"github.com/charmbracelet/log"
	"golang.org/x/term"
)

//go:embed config/Brewfile
var BrewFile string

const (
	brewPrefix       = "/opt/homebrew"
	brewInstallerURL = "https://raw.githubusercontent.com/Homebrew/install/b41c8e7b3588e2899974119faf3b2a897428648d/install.sh"
	brewBinary       = brewPrefix + "/bin/brew"
	levelNameWidth   = 5
)

var HomeDir string
var ConfigDir string

var Verbose bool

func runStep(ctx context.Context, title string, action func(context.Context) error) error {
	var err error
	if Verbose {
		slog.Info(title)
		err = action(ctx)
	} else {
		err = spinner.New().Title(title).Context(ctx).ActionWithErr(action).Run()
	}
	if err != nil {
		return err
	}
	slog.Info(title + " completed")
	return nil
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
	defer term.Restore(fd, state)

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

func startSudoKeepAlive(ctx context.Context) (*sudoKeepAlive, error) {
	ctx, cancel := context.WithCancel(ctx)
	keepAlive := &sudoKeepAlive{cancel: cancel, done: make(chan struct{})}
	if os.Geteuid() == 0 {
		return keepAlive, nil
	}
	cmd := exec.Command("sudo", "-v")
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	err := cmd.Run()
	if err != nil {
		return nil, err
	}
	go keepAlive.run(ctx)
	return keepAlive, nil
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

// run waits for cmd and returns what it printed on standard output. A command that fails carries
// its command line and what it printed on standard error in the error, and its standard output is
// still returned for the callers that read the complaints tools leave there.
func run(cmd *exec.Cmd) (string, error) {
	out, err := cmd.Output()
	if err == nil {
		return string(out), nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return string(out), fmt.Errorf("%s: %w\n%s", cmd, err, strings.TrimSpace(string(exitErr.Stderr)))
	}
	return string(out), fmt.Errorf("%s: %w", cmd, err)
}

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
	os.Setenv("PATH", filepath.Join(brewPrefix, "bin")+":"+os.Getenv("PATH"))

	return nil
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

func brewfilePath() string {
	return filepath.Join(omacyDir(), "Brewfile.base")
}

func installBundle(ctx context.Context) error {
	path := brewfilePath()
	if err := writeFile(path, []byte(BrewFile), 0o644); err != nil {
		return err
	}
	env := append(os.Environ(),
		"HOMEBREW_NO_AUTO_UPDATE=1",
		"HOMEBREW_NO_INSTALL_CLEANUP=1",
		"HOMEBREW_NO_INSTALL_UPGRADE=1",
		"HOMEBREW_NO_ANALYTICS=1",
		"HOMEBREW_NO_ENV_HINTS=1",
	)

	cmd := exec.CommandContext(ctx, "brew", "bundle", "install", "--file="+path, "--no-upgrade")
	cmd.Env = env
	out, err := run(cmd)
	if err != nil {
		return fmt.Errorf("brew bundle: %w", err)
	}
	slog.Debug("bundle installed", "stdout", out)
	return nil
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
	nonInteractive := flag.Bool("nonInteractive", false, "Skip actions that needs user interaction")
	noPermissionCheck := flag.Bool("noPermissionCheck", false, "Apply the macOS settings without asking and skip the Karabiner and Hammerspoon permission checks, so omacy never waits for a key press")
	verbose := flag.Bool("verbose", false, "Print every step as it runs instead of showing a spinner")
	flag.Parse()
	Verbose = *verbose
	skipPermissionChecks := *nonInteractive || *noPermissionCheck

	// The log shares stdout with the questions omacy asks, so the two stay in the order they happen.
	h := log.NewWithOptions(os.Stdout, log.Options{Level: log.DebugLevel})
	h.SetStyles(logStyles())
	slog.SetDefault(slog.New(h))

	var err error
	HomeDir, err = os.UserHomeDir()
	if err != nil {
		h.Fatal("find home directory", "error", err)
	}
	ConfigDir = filepath.Join(HomeDir, ".config")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if err := makeOmacyDir(); err != nil {
		h.Fatal("Cannot create the omacy directory", "err", err)
	}

	if !*nonInteractive {
		slog.Info("Configuring your system requires sudo. Enter your password below")
		keepAlive, err := startSudoKeepAlive(ctx)
		if err != nil {
			h.Fatal("Could not become sudo user.", "err", err)
		}
		defer keepAlive.stop()

		if err := runStep(ctx, "Installing Homebrew", installBrew); err != nil {
			h.Fatal("Installing Homebrew failed", "err", err)
		}
	}

	if err := runStep(ctx, "Installing Packages", installBundle); err != nil {
		h.Fatal("Installing Packages failed", "err", err)
	}

	if err := runStep(ctx, "Mise Configure & Install", setupMise); err != nil {
		h.Fatal("Configuring mise failed", "err", err)
	}

	if *installAppsOnly {
		return
	}

	if !*nonInteractive {
		if err := setFishAsDefault(ctx); err != nil {
			h.Fatal("Set fish as default failed", "err", err)
		}
		slog.Info("Set fish shell as default shell")
	} else {
		slog.Info("Skip setting fish as default shell because it requires sudo")
	}

	if err := runStep(ctx, "Configuring fish", setupFish); err != nil {
		h.Fatal("Configuring fish failed", "err", err)
	}

	applySettings := *noPermissionCheck
	if !skipPermissionChecks {
		applySettings, err = confirmMacSettings()
		if err != nil {
			h.Fatal("Stopped", "err", err)
		}
	}
	if applySettings {
		failures := applyMacOSSettings(ctx)
		reportSettingFailures(failures)
		slog.Info("Applied macOS settings", "failed", len(failures))
	} else {
		slog.Info("Skipped the macOS settings")
	}

	karabiner, err := karabinerPermission(ctx)
	switch {
	case karabiner == permissionGranted:
		slog.Info("Karabiner already has all its permissions, skipping setup")
	case skipPermissionChecks:
		slog.Warn("Skipping the Karabiner permission step, its shortcuts start working once the permissions are enabled")
		fmt.Println("Open Karabiner-Elements and enable the permissions it asks for")
	default:
		if karabiner == permissionUnknown {
			slog.Warn("Cannot ask Karabiner about its permissions", "err", err)
			fmt.Println("omacy cannot tell whether Karabiner has its permissions, so make sure you enable them")
		}
		fmt.Println("Press any key and Karabiner will open and prompt you to enable some permissions. Do that and then come back here to continue")
		if err := waitForAnyKey(); err != nil {
			h.Fatal("Stopped", "err", err)
		}
		runOpen(ctx, "-a", "Karabiner-Elements")

		for {
			fmt.Println("Press any key once the Karabiner permissions are enabled")
			if err := waitForAnyKey(); err != nil {
				h.Fatal("Stopped", "err", err)
			}
			karabiner, err = karabinerPermission(ctx)
			if karabiner == permissionGranted {
				slog.Info("Karabiner has all its permissions")
				break
			}
			if karabiner == permissionUnknown {
				slog.Warn("Cannot ask Karabiner about its permissions", "err", err)
				fmt.Println("omacy cannot tell whether Karabiner has its permissions, going on without an answer")
				break
			}
			fmt.Println("Karabiner still reports missing permissions or an inactive driver. Enable the ones it asks for in Karabiner-Elements")
		}
	}

	if err := runStep(ctx, "Configuring Karabiner", setupKarabiner); err != nil {
		h.Fatal("Configuring Karabiner failed", "err", err)
	}

	if err := runStep(ctx, "Configuring Hammerspoon", setupHammerspoon); err != nil {
		h.Fatal("Configuring Hammerspoon failed", "err", err)
	}

	if err := restartHammerspoon(ctx); err != nil {
		slog.Warn("Hammerspoon is not running the config omacy wrote, so its shortcuts and the hs command stay off until it starts", "err", err)
	}

	hammerspoon, err := checkHammerspoonPermission(ctx)
	switch {
	case hammerspoon == permissionGranted:
		slog.Info("Hammerspoon already has accessibility permission, skipping setup")
	case skipPermissionChecks:
		slog.Warn("Skipping the Hammerspoon permission step, its shortcuts start working once accessibility is allowed")
		fmt.Println("Start Hammerspoon and allow it in System Settings under Privacy & Security, Accessibility")
	default:
		if hammerspoon == permissionUnknown {
			slog.Warn("Cannot ask Hammerspoon about accessibility", "err", err)
			fmt.Println("omacy cannot tell whether Hammerspoon has accessibility permission, so make sure you turn it on")
		}
		fmt.Println("omacy is about to open the accessibility settings")
		fmt.Println("Turn Hammerspoon on in that list, then come back here")
		fmt.Println("Press any key to go on")
		if err := waitForAnyKey(); err != nil {
			h.Fatal("Stopped", "err", err)
		}

		runOpen(ctx, "x-apple.systempreferences:com.apple.preference.security?Privacy_Accessibility")

		for {
			fmt.Println("Press any key once Hammerspoon is turned on in the accessibility list")
			if err := waitForAnyKey(); err != nil {
				h.Fatal("Stopped", "err", err)
			}
			// Hammerspoon picks the permission up while it starts, so the one running now was
			// started before the list was ticked and still believes it has nothing.
			if err := restartHammerspoon(ctx); err != nil {
				slog.Warn("Hammerspoon is not running the config omacy wrote, so its shortcuts and the hs command stay off until it starts", "err", err)
			}
			hammerspoon, err = checkHammerspoonPermission(ctx)
			if hammerspoon == permissionGranted {
				slog.Info("Hammerspoon has accessibility permission")
				break
			}
			if hammerspoon == permissionUnknown {
				slog.Warn("Cannot ask Hammerspoon about accessibility", "err", err)
				fmt.Println("omacy cannot tell whether Hammerspoon has accessibility permission, going on without an answer")
				break
			}
			fmt.Println("Hammerspoon still says it has no accessibility permission. Turn it on in System Settings under Privacy & Security, Accessibility")
		}
	}
}
