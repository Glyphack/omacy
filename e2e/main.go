package main

import (
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
		deleteVM(devVMName)
		deleteVM(baseVMName)
	default:
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	if err != nil {
		fail(err)
	}
}

// windowFlag takes a leading -window off the arguments.
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

// buildBase makes the VM that run clones from and shuts it down so its disk can be copied.
func buildBase(window bool) error {
	if err := build(); err != nil {
		return err
	}
	vm, err := startVM(baseVMName, image, window)
	if err != nil {
		return err
	}
	if err := vm.configure(); err != nil {
		return err
	}
	runErr := vm.runOmacy([]string{"-installAppsOnly"})
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

// startVM makes a fresh VM called name from source and boots it. A VM with that name is replaced.
func startVM(name, source string, window bool) (VM, error) {
	deleteVM(name)
	vm := VM{Name: name}
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
	return vm.shell(fmt.Sprintf("echo '%s' | sudo -n tee %s >/dev/null && sudo -n chmod 0440 %s", rule, file, file))
}

// runOmacy copies the built binary into the VM and starts it there with this terminal attached.
func (vm VM) runOmacy(args []string) error {
	if err := vm.copy(binary, "omacy"); err != nil {
		return err
	}
	return vm.shell(append([]string{"./omacy"}, args...)...)
}

const binary = "bin/omacy"

// build compiles omacy for the VM the way the release does. It expects to run from the repository root.
func build() error {
	fmt.Printf("building %s\n", binary)
	cmd := exec.Command("go", "build", "-o", binary, ".")
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
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
