package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const usage = `usage: go run ./e2e <command> [flags] [args...]

  run [args]       replace the VM with a fresh one and start omacy in it
  continue [args]  start omacy again in the existing VM (boots or creates it when needed)
  base             build the base VM: a machine with Homebrew and the packages already installed,
                   which run then clones instead of waiting for those to install again
  ssh [command]    open a login shell in the VM, or run one command there with bash.
                   The command always runs in bash, not in the login shell of the VM, which is
                   fish once omacy has run. Write it as bash. To see what the login shell does
                   with it, ask for that shell: ssh "fish -lc 'which omacy'"
  clean            delete the VM, the base VM and the downloaded macOS image

flags for run and continue:
  -via build       build omacy on this machine and copy the binary into the VM (default)
  -via release     copy install.sh into the VM and let it download and start the dev release
  -window          boot the VM with a visible window instead of headless

flags for base:
  -window          boot the VM with a visible window instead of headless
`

// Installer gets omacy into the VM. Prepare does the local work before the VM is touched,
// Install copies omacy into the running VM and returns the command that starts it there.
type Installer interface {
	Prepare() error
	Install(vm VM) ([]string, error)
}

type buildInstaller struct {
	binary string
}

func (b *buildInstaller) Prepare() error {
	root, err := moduleRoot()
	if err != nil {
		return err
	}
	b.binary = filepath.Join(root, "bin", "omacy")

	fmt.Printf("building %s\n", b.binary)
	cmd := exec.Command("go", "build", "-o", b.binary, ".")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("build failed: %w", err)
	}
	return nil
}

func (b *buildInstaller) Install(vm VM) ([]string, error) {
	if err := vm.Copy(b.binary, "omacy"); err != nil {
		return nil, err
	}
	return []string{"./omacy"}, nil
}

type releaseInstaller struct {
	script string
}

func (r *releaseInstaller) Prepare() error {
	root, err := moduleRoot()
	if err != nil {
		return err
	}
	r.script = filepath.Join(root, "install.sh")
	if _, err := os.Stat(r.script); err != nil {
		return fmt.Errorf("install script: %w", err)
	}
	return nil
}

func (r *releaseInstaller) Install(vm VM) ([]string, error) {
	if err := vm.Copy(r.script, "install.sh"); err != nil {
		return nil, err
	}
	return []string{"bash", "install.sh"}, nil
}

const (
	devVMName  = "omacy"
	baseVMName = "omacy-base"
)

func newVM(name string) VM {
	return VM{
		Name:     name,
		Image:    "ghcr.io/cirruslabs/macos-tahoe-vanilla:26.6.2",
		User:     "admin",
		Password: "admin",
		LogPath:  filepath.Join(os.TempDir(), name+"-vm.log"),
	}
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	if err := requireTools(requiredTools...); err != nil {
		fail(err)
	}

	vm := newVM(devVMName)
	vm.Base = baseVMName
	base := newVM(baseVMName)

	command, args := os.Args[1], os.Args[2:]
	var err error
	switch command {
	case "run", "continue":
		var opts options
		opts, err = parseOptions(command, args)
		if err != nil {
			break
		}
		vm.Window = opts.window
		if command == "run" {
			err = runFresh(vm, opts.installer, opts.omacyArgs)
		} else {
			err = runAgain(vm, opts.installer, opts.omacyArgs)
		}
	case "base":
		var window bool
		window, err = parseBaseOptions(args)
		if err != nil {
			break
		}
		base.Window = window
		err = buildBase(base)
	case "ssh":
		err = vm.Shell(args...)
	case "clean":
		err = clean(vm, base)
	default:
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	if err != nil {
		fail(err)
	}
}

// options holds what the run and continue commands were asked to do.
type options struct {
	installer Installer
	window    bool
	omacyArgs []string
}

func parseOptions(command string, args []string) (options, error) {
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.Usage = func() { fmt.Fprint(os.Stderr, usage) }
	via := flags.String("via", "build", "build or release")
	window := flags.Bool("window", false, "boot the VM with a visible window")
	if err := flags.Parse(args); err != nil {
		return options{}, err
	}
	opts := options{window: *window, omacyArgs: flags.Args()}

	switch *via {
	case "build":
		opts.installer = &buildInstaller{}
	case "release":
		if len(opts.omacyArgs) > 0 {
			return options{}, errors.New("install.sh does not pass arguments on to omacy, drop them or use -via build")
		}
		opts.installer = &releaseInstaller{}
	default:
		return options{}, fmt.Errorf("unknown -via value %q, use build or release", *via)
	}
	return opts, nil
}

func runFresh(vm VM, installer Installer, omacyArgs []string) error {
	if err := installer.Prepare(); err != nil {
		return err
	}
	if err := vm.Remove(); err != nil {
		return err
	}
	return installAndRun(vm, installer, omacyArgs)
}

func runAgain(vm VM, installer Installer, omacyArgs []string) error {
	if err := installer.Prepare(); err != nil {
		return err
	}
	return installAndRun(vm, installer, omacyArgs)
}

func installAndRun(vm VM, installer Installer, omacyArgs []string) error {
	if err := vm.EnsureRunning(); err != nil {
		return err
	}
	command, err := installer.Install(vm)
	if err != nil {
		return err
	}
	if err := vm.Shell(append(command, omacyArgs...)...); err != nil {
		return err
	}
	return verifyHammerspoon(vm)
}

// hammerspoonCheckCommand asks the hs command line tool for the version of the Omacy spoon, the way
// a real user's shortcuts depend on it being there.
const hammerspoonCheckCommand = `/Applications/Hammerspoon.app/Contents/Frameworks/hs/hs -a -t 2 -c 'print(spoon.Omacy and spoon.Omacy.version or "missing")'`

// verifyHammerspoon checks that Hammerspoon is up and has the Omacy spoon loaded once omacy has run.
func verifyHammerspoon(vm VM) error {
	out, err := vm.execOutput(hammerspoonCheckCommand)
	if err != nil {
		return fmt.Errorf("could not ask Hammerspoon for the Omacy spoon version: %w", err)
	}
	if out == "" || out == "missing" {
		return fmt.Errorf("no Omacy spoon version reported, Hammerspoon answered %q", out)
	}
	fmt.Printf("Hammerspoon answers, Omacy spoon version %s\n", out)
	return nil
}

func parseBaseOptions(args []string) (bool, error) {
	flags := flag.NewFlagSet("base", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.Usage = func() { fmt.Fprint(os.Stderr, usage) }
	window := flags.Bool("window", false, "boot the VM with a visible window")
	if err := flags.Parse(args); err != nil {
		return false, err
	}
	if flags.NArg() > 0 {
		return false, fmt.Errorf("base takes no arguments, got %q", strings.Join(flags.Args(), " "))
	}
	return *window, nil
}

// buildBase makes the VM that run clones from. It starts over from the macOS image every time, runs
// only the omacy steps that are the same on any mac, and shuts the machine down so its disk can be
// copied whole.
func buildBase(base VM) error {
	installer := &buildInstaller{}
	if err := installer.Prepare(); err != nil {
		return err
	}
	if err := base.Remove(); err != nil {
		return err
	}
	if err := base.EnsureRunning(); err != nil {
		return err
	}
	command, err := installer.Install(base)
	if err != nil {
		return err
	}
	if err := base.Shell(append(command, "--installAppsOnly")...); err != nil {
		// A half installed base is worse than none: run would clone it and brew would report every
		// package as already there, so the missing ones never get installed. Throw it away instead.
		fmt.Printf("the install failed, deleting %s so run starts from the macOS image again\n", base.Name)
		if removeErr := base.Remove(); removeErr != nil {
			return removeErr
		}
		return err
	}
	if err := base.Stop(); err != nil {
		return err
	}
	fmt.Printf("%s is ready, go run ./e2e run now starts from it\n", base.Name)
	return nil
}

func clean(vm, base VM) error {
	for _, target := range []VM{vm, base} {
		if err := target.Remove(); err != nil {
			return err
		}
		_ = os.Remove(target.LogPath)
		_ = os.Remove(target.LogPath + ".window")
	}
	return vm.RemoveImage()
}

func moduleRoot() (string, error) {
	out, err := exec.Command("go", "list", "-m", "-f", "{{.Dir}}").Output()
	if err != nil {
		return "", fmt.Errorf("find module root: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// tool is a program the e2e commands need, together with the command that installs it.
type tool struct {
	name    string
	install string
}

var requiredTools = []tool{
	{name: "tart", install: "brew install cirruslabs/cli/tart"},
}

func requireTools(tools ...tool) error {
	for _, t := range tools {
		if _, err := exec.LookPath(t.name); err != nil {
			return fmt.Errorf("missing tool %q, install it with: %s", t.name, t.install)
		}
	}
	return nil
}

func fail(err error) {
	var code exitCode
	if errors.As(err, &code) {
		os.Exit(int(code))
	}
	fmt.Fprintln(os.Stderr, "e2e:", err)
	os.Exit(1)
}
