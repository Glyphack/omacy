package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

type VM struct {
	Name     string
	Image    string
	User     string
	Password string
	LogPath  string
	Window   bool
	// Base names a local VM that already carries the slow part of the setup. A fresh VM is cloned
	// from it when it exists, and from Image when it does not.
	Base string
}

type State string

const (
	StateMissing State = "missing"
	StateRunning State = "running"
	StateStopped State = "stopped"
)

// exitCode carries the exit status of a command that already printed its own errors.
type exitCode int

func (e exitCode) Error() string {
	return fmt.Sprintf("exit status %d", int(e))
}

var sshOptions = []string{
	"-o", "StrictHostKeyChecking=no",
	"-o", "UserKnownHostsFile=/dev/null",
	"-o", "LogLevel=ERROR",
	"-o", "ConnectTimeout=5",
	"-o", "PubkeyAuthentication=no",
	"-o", "PreferredAuthentications=password",
	"-o", "NumberOfPasswordPrompts=1",
}

// passwordVar is the name of the environment variable the askpass helper reads the password from.
const passwordVar = "OMACY_VM_PASSWORD"

// askpassScript prints the password when ssh asks for it.
const askpassScript = "#!/bin/sh\nprintf '%s\\n' \"$" + passwordVar + "\"\n"

// sshLogin is what every ssh and scp call to a VM needs: the user and address to connect to, and
// the environment that answers the password prompt.
type sshLogin struct {
	address string
	env     []string
}

func (vm VM) login() (sshLogin, error) {
	ip, err := vm.ip()
	if err != nil {
		return sshLogin{}, err
	}
	helper := filepath.Join(os.TempDir(), "omacy-e2e-askpass")
	if err := os.WriteFile(helper, []byte(askpassScript), 0o700); err != nil {
		return sshLogin{}, fmt.Errorf("write the askpass helper %s: %w", helper, err)
	}
	env := append(os.Environ(),
		"SSH_ASKPASS="+helper,
		"SSH_ASKPASS_REQUIRE=force",
		passwordVar+"="+vm.Password,
	)
	return sshLogin{address: vm.User + "@" + ip, env: env}, nil
}

// ssh builds the command that runs command in the VM, or opens a login shell when no command is given.
func (l sshLogin) ssh(terminal bool, command ...string) *exec.Cmd {
	args := append([]string{}, sshOptions...)
	if terminal {
		args = append(args, "-t")
	}
	args = append(args, l.address)
	args = append(args, remoteCommand(command)...)
	cmd := exec.Command("ssh", args...)
	cmd.Env = l.env
	return cmd
}

func (l sshLogin) scp(local, remote string) *exec.Cmd {
	args := append([]string{}, sshOptions...)
	args = append(args, local, l.address+":"+remote)
	cmd := exec.Command("scp", args...)
	cmd.Env = l.env
	return cmd
}

// remoteCommand wraps a command line so the VM always runs it with bash. ssh hands the line to the
// login shell of the user, which is fish once omacy has run, so the line travels encoded and bash
// on the other side gets it back exactly as it was written.
func remoteCommand(command []string) []string {
	if len(command) == 0 {
		return nil
	}
	line := base64.StdEncoding.EncodeToString([]byte(strings.Join(command, " ")))
	return []string{"bash", "-lc", `"$(echo ` + line + ` | base64 -d)"`}
}

type tartEntry struct {
	Name    string `json:"Name"`
	State   string `json:"State"`
	Running bool   `json:"Running"`
}

func tartList(source string) ([]tartEntry, error) {
	out, err := tartOutput("list", "--source", source, "--format", "json")
	if err != nil {
		return nil, err
	}
	var entries []tartEntry
	if err := json.Unmarshal([]byte(out), &entries); err != nil {
		return nil, fmt.Errorf("parse tart list: %w", err)
	}
	return entries, nil
}

func vmState(name string) (State, error) {
	entries, err := tartList("local")
	if err != nil {
		return "", err
	}
	for _, entry := range entries {
		if entry.Name != name {
			continue
		}
		if entry.State != "" {
			return State(entry.State), nil
		}
		if entry.Running {
			return StateRunning, nil
		}
		return StateStopped, nil
	}
	return StateMissing, nil
}

func (vm VM) State() (State, error) {
	return vmState(vm.Name)
}

func (vm VM) ImageDownloaded() (bool, error) {
	entries, err := tartList("oci")
	if err != nil {
		return false, err
	}
	for _, entry := range entries {
		if entry.Name == vm.Image {
			return true, nil
		}
	}
	return false, nil
}

// EnsureRunning creates the VM from its clone source when it is missing, boots it when it is not running,
// restarts it when it is running in the wrong window mode, makes sure sudo inside it never asks
// for a password, and turns Remote Login on.
func (vm VM) EnsureRunning() error {
	state, err := vm.State()
	if err != nil {
		return err
	}
	if state == StateMissing {
		source, err := vm.cloneSource()
		if err != nil {
			return err
		}
		if source == vm.Image {
			fmt.Printf("cloning %s into %s (downloads the image on first use)\n", source, vm.Name)
		} else {
			fmt.Printf("cloning %s into %s, Homebrew and the packages come with it\n", source, vm.Name)
		}
		if err := runVisible("tart", "clone", source, vm.Name); err != nil {
			return err
		}
		state = StateStopped
	}
	if state == StateRunning && !vm.windowModeMatches() {
		fmt.Printf("stopping %s to switch to running %s\n", vm.Name, windowModeLabel(vm.Window))
		if err := runVisible("tart", "stop", vm.Name); err != nil {
			return err
		}
		state = StateStopped
	}
	if state != StateRunning {
		if err := vm.start(); err != nil {
			return err
		}
	}
	if err := vm.disableSudoPassword(); err != nil {
		return err
	}
	return vm.enableRemoteLogin()
}

// enableRemoteLogin turns Remote Login on so the VM keeps taking ssh connections, which is how
// everything here reaches it and how a real mac is reached while omacy runs.
func (vm VM) enableRemoteLogin() error {
	if err := vm.exec("sudo -n systemsetup -f -setremotelogin on"); err != nil {
		fmt.Printf("could not turn Remote Login on in %s: %v\n", vm.Name, err)
	}
	return nil
}

// cloneSource picks what a fresh copy of this VM is made from. The base VM is preferred because the
// slow steps are already done in it, and it has to be shut down first so the copy gets a whole disk.
func (vm VM) cloneSource() (string, error) {
	if vm.Base == "" {
		return vm.Image, nil
	}
	state, err := vmState(vm.Base)
	if err != nil {
		return "", err
	}
	if state == StateMissing {
		return vm.Image, nil
	}
	if state == StateRunning {
		fmt.Printf("%s is running, shutting it down so the clone gets a settled disk\n", vm.Base)
		base := vm
		base.Name = vm.Base
		if err := base.Stop(); err != nil {
			return "", err
		}
	}
	return vm.Base, nil
}

// Stop shuts the VM down with everything it did safely on its disk. The guest is told to flush
// first, because tart kills the machine once its timeout runs out, and a machine killed while it
// still holds unwritten work loses that work. A base image built from such a disk is missing
// packages that brew already recorded as installed, and brew never installs them again.
func (vm VM) Stop() error {
	state, err := vm.State()
	if err != nil {
		return err
	}
	if state != StateRunning {
		return nil
	}
	if err := vm.exec("sync"); err != nil {
		return err
	}
	fmt.Printf("stopping %s\n", vm.Name)
	return runVisible("tart", "stop", vm.Name, "--timeout", "120")
}

// Remove stops and deletes the VM when it exists.
func (vm VM) Remove() error {
	state, err := vm.State()
	if err != nil {
		return err
	}
	if state == StateMissing {
		return nil
	}
	if state == StateRunning {
		fmt.Printf("stopping %s\n", vm.Name)
		if err := runVisible("tart", "stop", vm.Name); err != nil {
			return err
		}
	}
	fmt.Printf("deleting %s\n", vm.Name)
	return runVisible("tart", "delete", vm.Name)
}

func (vm VM) RemoveImage() error {
	downloaded, err := vm.ImageDownloaded()
	if err != nil {
		return err
	}
	if !downloaded {
		return nil
	}
	fmt.Printf("deleting downloaded image %s\n", vm.Image)
	return runVisible("tart", "delete", vm.Image)
}

func (vm VM) Copy(local, remote string) error {
	login, err := vm.login()
	if err != nil {
		return err
	}
	return runCommand(login.scp(local, remote))
}

// Shell runs a command in the VM with the local terminal attached, or opens a login shell when no command is given.
func (vm VM) Shell(command ...string) error {
	state, err := vm.State()
	if err != nil {
		return err
	}
	if state != StateRunning {
		return fmt.Errorf("%s is not running, start it with: go run ./e2e continue", vm.Name)
	}
	login, err := vm.login()
	if err != nil {
		return err
	}
	cmd := login.ssh(true, command...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	err = cmd.Run()
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exitCode(exit.ExitCode())
	}
	return err
}

// exec runs a command in the VM without a terminal and returns its stderr in the error when it fails.
func (vm VM) exec(command string) error {
	_, err := vm.execOutput(command)
	return err
}

// execOutput runs a command in the VM without a terminal and returns what it printed. It fails the
// same way exec does, with its output folded into the error.
func (vm VM) execOutput(command string) (string, error) {
	login, err := vm.login()
	if err != nil {
		return "", err
	}
	out, err := login.ssh(false, command).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("in %s: %s: %w: %s", vm.Name, command, err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

// disableSudoPassword lets the VM user run any sudo command, including sudo -v, without a password.
// The VM has just booted, so the first connection right after waitForSSH can still land while macOS
// is busy starting up and get its login rejected; retry a few times rather than fail outright.
func (vm VM) disableSudoPassword() error {
	rule := fmt.Sprintf("Defaults:%s !authenticate", vm.User)
	file := "/etc/sudoers.d/no-password"
	command := fmt.Sprintf(
		"printf '%%s\\n' '%s' | sudo -n tee %s >/dev/null && sudo -n chmod 0440 %s && sudo -n visudo -c -q -f %s",
		rule, file, file, file,
	)
	var err error
	for attempt := 0; attempt < 5; attempt++ {
		if err = vm.exec(command); err == nil {
			return nil
		}
		time.Sleep(2 * time.Second)
	}
	return err
}

func (vm VM) start() error {
	logFile, err := os.Create(vm.LogPath)
	if err != nil {
		return err
	}
	defer func() { _ = logFile.Close() }()

	args := []string{"run", vm.Name}
	if !vm.Window {
		args = append(args, "--no-graphics")
	}
	fmt.Printf("booting %s %s, log: %s\n", vm.Name, windowModeLabel(vm.Window), vm.LogPath)
	cmd := exec.Command("tart", args...)
	cmd.Stdout, cmd.Stderr = logFile, logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("tart run: %w", err)
	}
	if err := vm.recordWindowMode(); err != nil {
		return err
	}
	if err := cmd.Process.Release(); err != nil {
		return err
	}
	return vm.waitForSSH(5 * time.Minute)
}

func windowModeLabel(window bool) string {
	if window {
		return "with a window"
	}
	return "without a window"
}

// windowMarkerPath is where we remember which window mode the VM was last booted with, since tart
// itself does not report that.
func (vm VM) windowMarkerPath() string {
	return vm.LogPath + ".window"
}

func (vm VM) recordWindowMode() error {
	value := "0"
	if vm.Window {
		value = "1"
	}
	return os.WriteFile(vm.windowMarkerPath(), []byte(value), 0o644)
}

// windowModeMatches reports whether the running VM was last booted in the window mode vm now wants.
// When we have no record of it, we assume the default headless mode.
func (vm VM) windowModeMatches() bool {
	data, err := os.ReadFile(vm.windowMarkerPath())
	if err != nil {
		return !vm.Window
	}
	want := "0"
	if vm.Window {
		want = "1"
	}
	return strings.TrimSpace(string(data)) == want
}

func (vm VM) waitForSSH(timeout time.Duration) error {
	login, err := vm.login()
	if err != nil {
		return err
	}
	deadline := time.Now().Add(timeout)
	for {
		if login.ssh(false, "true").Run() == nil {
			fmt.Printf("%s is up at %s\n", vm.Name, login.address)
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("%s did not accept ssh within %s, see %s", vm.Name, timeout, vm.LogPath)
		}
		time.Sleep(2 * time.Second)
	}
}

func (vm VM) ip() (string, error) {
	return tartOutput("ip", "--wait", "120", vm.Name)
}

func tartOutput(args ...string) (string, error) {
	out, err := exec.Command("tart", args...).Output()
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return "", fmt.Errorf("tart %s: %s", strings.Join(args, " "), strings.TrimSpace(string(exit.Stderr)))
	}
	if err != nil {
		return "", fmt.Errorf("tart %s: %w", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(out)), nil
}

func runVisible(name string, args ...string) error {
	return runCommand(exec.Command(name, args...))
}

// runCommand runs a command with the local terminal attached.
func runCommand(cmd *exec.Cmd) error {
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w", strings.Join(cmd.Args, " "), err)
	}
	return nil
}
