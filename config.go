package main

import (
	"flag"
	"fmt"
	"os"
	"slices"
	"strings"
)

type option struct {
	flag  string
	title string
	help  string
	on    bool
}

type Config struct {
	profile   string
	install   installChoices
	configure configureChoices
	macOS     settingGroups
}

type installChoices struct {
	brew option
	mise option
}

type configureChoices struct {
	defaultShell option
	fish         option
	wezterm      option
	karabiner    appWithPermissions
	hammerspoon  appWithPermissions
}

type appWithPermissions struct {
	config          option
	permissionSetup option
}

type optionGroup struct {
	name    string
	options []*option
}

func newConfig() Config {
	return Config{
		install: installChoices{
			brew: option{flag: "brew", title: "Homebrew apps", help: "Installs Homebrew and the apps the rest of the setup needs, like WezTerm, Karabiner and Hammerspoon. Also adds Brave, Raycast, Zed and IINA."},
			mise: option{flag: "mise", title: "mise tools", help: "Installs developer tools like fzf, ripgrep and Go with mise. Add a tool to the mise config and every Mac you set up gets it."},
		},
		configure: configureChoices{
			defaultShell: option{flag: "default-shell", title: "Fish as login shell", help: "Makes fish the shell every new terminal opens. fish suggests commands as you type and completes them with tab, with no plugins."},
			fish:         option{flag: "fish", title: "Fish config", help: "Adds a prompt with the folder and git branch, and fzf search through past commands. Also adds ,cp and ,pf to copy and paste files, and ntfy to send a notification to your phone."},
			wezterm:      option{flag: "wezterm", title: "WezTerm config", help: "Gives WezTerm colors that follow light and dark mode, the Hack Nerd Font, and cmd+shift+c to copy the output of the last command. You turn it on with one line in wezterm.lua."},
			karabiner: appWithPermissions{
				config:          option{flag: "karabiner", title: "Karabiner config", help: "Caps Lock becomes Escape when you tap it and Hyper (cmd+ctrl+option) when you hold it, a key no app uses. cmd+shift+v pastes with no formatting in Notes and Outlook."},
				permissionSetup: option{flag: "karabiner-permissions", title: "Karabiner permissions", help: "Opens Karabiner and waits while you allow its keyboard driver and input monitoring. Without them the Caps Lock and paste keys do nothing."},
			},
			hammerspoon: appWithPermissions{
				config:          option{flag: "hammerspoon", title: "Hammerspoon config", help: "Hold Caps Lock and press a key to snap windows to half the screen, move them between screens or open Brave and WezTerm. It also adds a window picker, a bookmark picker and a mic mute key."},
				permissionSetup: option{flag: "hammerspoon-permissions", title: "Hammerspoon permissions", help: "Opens the accessibility settings and waits while you turn Hammerspoon on. Without it the window shortcuts cannot move windows."},
			},
		},
		macOS: macOSSettingGroups(),
	}
}

func (c *Config) groups() []optionGroup {
	groups := []optionGroup{
		{name: "Install", options: []*option{&c.install.brew, &c.install.mise}},
		{name: "Configure", options: []*option{
			&c.configure.defaultShell,
			&c.configure.fish,
			&c.configure.wezterm,
			&c.configure.karabiner.config,
			&c.configure.karabiner.permissionSetup,
			&c.configure.hammerspoon.config,
			&c.configure.hammerspoon.permissionSetup,
		}},
	}
	for i := range c.macOS {
		group := optionGroup{name: c.macOS[i].name}
		for j := range c.macOS[i].settings {
			group.options = append(group.options, &c.macOS[i].settings[j].option)
		}
		groups = append(groups, group)
	}
	return groups
}

func (c *Config) options() []*option {
	var options []*option
	for _, group := range c.groups() {
		options = append(options, group.options...)
	}
	return options
}

func (c *Config) option(flag string) *option {
	for _, o := range c.options() {
		if o.flag == flag {
			return o
		}
	}
	return nil
}

type configProfile struct {
	name  string
	title string
	apply func(*Config)
}

var configProfiles = []configProfile{
	{name: "new-mac", title: "New Mac", apply: newMacProfile},
	{name: "current-mac", title: "Current Mac", apply: currentMacProfile},
	{name: "install-apps", title: "Install apps only", apply: func(c *Config) {
		c.install.brew.on = true
		c.install.mise.on = true
	}},
	{name: "configure", title: "Configure only", apply: func(c *Config) {
		currentMacProfile(c)
		c.install.brew.on = false
		c.install.mise.on = false
	}},
}

func newMacProfile(c *Config) {
	for _, o := range c.options() {
		o.on = true
	}
}

func currentMacProfile(c *Config) {
	newMacProfile(c)
	c.option("macos-dock-clear").on = false
}

type arguments struct {
	profile       int // index in configProfiles
	noInteractive bool
	verbose       bool
	given         map[string]bool // only the option flags set on the command line, by flag name
	command       string
	brewCommand   string
}

func (a arguments) config(profile configProfile) Config {
	c := newConfig()
	c.profile = profile.name
	profile.apply(&c)
	for _, o := range c.options() {
		if on, ok := a.given[o.flag]; ok {
			o.on = on
		}
	}
	return c
}

func parseArgs(args []string) arguments {
	var names []string
	for _, p := range configProfiles {
		names = append(names, p.name)
	}

	flags := flag.NewFlagSet("omacy", flag.ExitOnError)
	parsed := newConfig()
	for _, o := range parsed.options() {
		flags.BoolVar(&o.on, o.flag, false, o.help)
	}
	a := arguments{given: map[string]bool{}}
	profile := flags.String("profile", configProfiles[0].name, "The profile to start from: "+strings.Join(names, ", ")+". The default is "+names[0]+".")
	flags.BoolVar(&a.noInteractive, "no-interactive", false, "Skips the form and sets up the profile with the flags given.")
	flags.BoolVar(&a.verbose, "verbose", false, "Prints every step as it runs instead of showing a spinner.")
	flags.Usage = func() { printUsage(flags) }
	_ = flags.Parse(args)

	a.profile = slices.Index(names, *profile)
	if a.profile < 0 {
		fmt.Fprintf(os.Stderr, "unknown profile %q, pick one of %s\n", *profile, strings.Join(names, ", "))
		os.Exit(2)
	}

	flags.Visit(func(fl *flag.Flag) {
		if o := parsed.option(fl.Name); o != nil {
			a.given[fl.Name] = o.on
		}
	})

	a.command, a.brewCommand = flags.Arg(0), flags.Arg(1)
	if a.command != "" && a.command != "brew" {
		fmt.Fprintf(os.Stderr, "unknown command %q\n", a.command)
		flags.Usage()
		os.Exit(2)
	}
	return a
}

// printUsage prints the flags with two dashes, under the name of their group.
func printUsage(flags *flag.FlagSet) {
	config := newConfig()
	groups := append([]optionGroup{{name: "General", options: []*option{
		{flag: "profile <name>", help: flags.Lookup("profile").Usage},
		{flag: "no-interactive", help: flags.Lookup("no-interactive").Usage},
		{flag: "verbose", help: flags.Lookup("verbose").Usage},
	}}}, config.groups()...)

	var usage strings.Builder
	usage.WriteString("Usage: omacy [flags]\n       omacy brew edit|sync|dump\n\n")
	usage.WriteString("Option flags turn a part of the setup on, like --brew, or off, like --brew=false.\n")
	for _, group := range groups {
		fmt.Fprintf(&usage, "\n%s\n", group.name)
		for _, o := range group.options {
			fmt.Fprintf(&usage, "  --%s\n        %s\n", o.flag, o.help)
		}
	}
	fmt.Fprint(os.Stderr, usage.String())
}
