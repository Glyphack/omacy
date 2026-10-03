package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
)

const usage = `usage: go run ./e2e <command>

  run [-window] [omacy args]  make a fresh VM, start omacy in it and leave the VM running
  base [-window]              make the VM that run clones from, with only the apps installed
  ssh [command]               open a shell in the VM, or run one command in its login shell
  clean                       delete the VMs made by run and base

-window boots the VM with a visible screen instead of headless.
To start omacy again in the same VM: go run ./e2e ssh ./omacy
`

const (
	devVMName  = "omacy"
	baseVMName = "omacy-base"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	if err := writeAskpass(); err != nil {
		fail(err)
	}

	command, args := os.Args[1], os.Args[2:]
	var err error
	switch command {
	case "run":
		window, omacyArgs := windowFlag(args)
		err = run(window, omacyArgs)
	case "base":
		window, _ := windowFlag(args)
		err = buildBase(window)
	case "ssh":
		err = ssh(args)
	case "clean":
		err = errors.Join(deleteVM(devVMName), deleteVM(baseVMName))
	default:
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	if err != nil {
		fail(err)
	}
}

func windowFlag(args []string) (bool, []string) {
	if len(args) > 0 && args[0] == "-window" {
		return true, args[1:]
	}
	return false, args
}

func run(window bool, omacyArgs []string) error {
	if err := build(); err != nil {
		return err
	}
	source := image
	if _, err := tart("get", baseVMName); err == nil {
		source = baseVMName
	}
	vm, err := startVM(devVMName, source, window)
	if err != nil {
		return err
	}
	if err := vm.configure(); err != nil {
		return err
	}
	return vm.runOmacy(omacyArgs)
}

// buildBase deletes a base that fails half way, so run does not clone it.
func buildBase(window bool) error {
	if err := build(); err != nil {
		return err
	}
	if err := makeBase(window); err != nil {
		return errors.Join(err, deleteVM(baseVMName))
	}
	return nil
}

func makeBase(window bool) error {
	vm, err := startVM(baseVMName, image, window)
	if err != nil {
		return err
	}
	if err := vm.configure(); err != nil {
		return err
	}
	runErr := vm.runOmacy([]string{"--profile=install-apps", "--no-interactive"})
	if err := tartRun("stop", "--timeout", "120", vm.Name); err != nil {
		return err
	}
	return runErr
}

func ssh(command []string) error {
	ip, err := tart("ip", devVMName)
	if err != nil {
		return err
	}
	return VM{Name: devVMName, IP: ip}.shell(command...)
}

func startVM(name, source string, window bool) (VM, error) {
	vm := VM{Name: name}
	if err := deleteVM(name); err != nil {
		return vm, err
	}
	fmt.Printf("cloning %s into %s\n", source, name)
	if err := tartRun("clone", source, name); err != nil {
		return vm, err
	}
	if err := vm.boot(window); err != nil {
		return vm, err
	}
	return vm, nil
}

// configure lets every sudo call in the VM pass without a password, since nobody is there to type one.
func (vm VM) configure() error {
	file := "/etc/sudoers.d/no-password"
	rule := "Defaults:" + user + " !authenticate"
	if err := vm.shell(fmt.Sprintf("echo '%s' | sudo -n tee %s >/dev/null && sudo -n chmod 0440 %s", rule, file, file)); err != nil {
		return fmt.Errorf("let sudo pass without a password in %s: %w", vm.Name, err)
	}
	return nil
}

func (vm VM) runOmacy(args []string) error {
	if err := vm.copy(binary, "omacy"); err != nil {
		return err
	}
	if err := vm.shell(append([]string{"./omacy"}, args...)...); err != nil {
		return fmt.Errorf("run omacy in %s: %w", vm.Name, err)
	}
	return nil
}

const binary = "bin/omacy"

// build expects to run from the repository root.
func build() error {
	fmt.Printf("building %s\n", binary)
	cmd := exec.Command("go", "build", "-o", binary, ".")
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS=darwin", "GOARCH=arm64")
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("build omacy: %w", err)
	}
	return nil
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "e2e:", err)
	os.Exit(1)
}
