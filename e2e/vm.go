package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const (
	image    = "ghcr.io/cirruslabs/macos-tahoe-vanilla:26.6.2"
	user     = "admin"
	password = "admin"
	sshWait  = 5 * time.Minute
)

type VM struct {
	Name string
	IP   string
}

var sshOptions = []string{
	"-o", "StrictHostKeyChecking=no",
	"-o", "UserKnownHostsFile=/dev/null",
	"-o", "LogLevel=ERROR",
	"-o", "ConnectTimeout=5",
	"-o", "PreferredAuthentications=password",
	"-o", "NumberOfPasswordPrompts=1",
}

// sshEnv makes ssh and scp take the VM password from a small script instead of asking on the terminal.
var sshEnv []string

func writeAskpass() error {
	askpass := filepath.Join(os.TempDir(), "omacy-e2e-askpass")
	if err := os.WriteFile(askpass, []byte("#!/bin/sh\necho "+password+"\n"), 0o700); err != nil {
		return fmt.Errorf("write the ssh password script: %w", err)
	}
	sshEnv = append(os.Environ(), "SSH_ASKPASS="+askpass, "SSH_ASKPASS_REQUIRE=force")
	return nil
}

// boot starts the VM in its own session, so it outlives this program.
func (vm *VM) boot(window bool) error {
	args := []string{"run", vm.Name}
	if !window {
		args = append(args, "--no-graphics")
	}
	logPath := filepath.Join(os.TempDir(), "omacy-e2e-"+vm.Name+".log")
	logFile, err := os.Create(logPath)
	if err != nil {
		return fmt.Errorf("create the tart run log: %w", err)
	}
	defer func() { _ = logFile.Close() }()

	fmt.Printf("booting %s\n", vm.Name)
	cmd := exec.Command("tart", args...)
	cmd.Stdout, cmd.Stderr = logFile, logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("tart run: %w", err)
	}
	if err := cmd.Process.Release(); err != nil {
		return err
	}

	ip, err := tart("ip", "--wait", "120", vm.Name)
	if err != nil {
		printed, _ := os.ReadFile(logPath)
		return fmt.Errorf("%w\ntart run printed:\n%s", err, strings.TrimSpace(string(printed)))
	}
	vm.IP = ip
	deadline := time.Now().Add(sshWait)
	for vm.sshCmd("true").Run() != nil {
		if time.Now().After(deadline) {
			return fmt.Errorf("%s did not take ssh logins within %s", vm.Name, sshWait)
		}
		time.Sleep(2 * time.Second)
	}
	fmt.Printf("%s is up at %s\n", vm.Name, vm.IP)
	return nil
}

func deleteVM(name string) error {
	if _, err := tart("get", name); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return err
		}
		return nil
	}
	_, _ = tart("stop", name)
	if _, err := tart("delete", name); err != nil {
		return err
	}
	fmt.Printf("deleted %s\n", name)
	return nil
}

func (vm VM) sshCmd(command ...string) *exec.Cmd {
	args := append(append([]string{}, sshOptions...), "-t", user+"@"+vm.IP)
	cmd := exec.Command("ssh", append(args, command...)...)
	cmd.Env = sshEnv
	return cmd
}

// shell runs command through the login shell of the VM, since a plain ssh command does not get the
// PATH a person gets in a terminal.
func (vm VM) shell(command ...string) error {
	if len(command) > 0 {
		command = []string{"$SHELL", "-lc", quote(strings.Join(command, " "))}
	}
	cmd := vm.sshCmd(command...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}

// quote wraps s in single quotes the way zsh, bash and fish all read them.
func quote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func (vm VM) copy(local, remote string) error {
	args := append(append([]string{}, sshOptions...), local, user+"@"+vm.IP+":"+remote)
	cmd := exec.Command("scp", args...)
	cmd.Env = sshEnv
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("copy %s into %s: %w", local, vm.Name, err)
	}
	return nil
}

func tart(args ...string) (string, error) {
	out, err := exec.Command("tart", args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("tart %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

func tartRun(args ...string) error {
	cmd := exec.Command("tart", args...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("tart %s: %w", strings.Join(args, " "), err)
	}
	return nil
}
