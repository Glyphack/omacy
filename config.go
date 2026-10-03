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
			brew: option{flag: "brew", title: "apps", help: "Installs Homebrew and the apps the rest of the setup needs: Brave Browser, Karabiner, Hammerspoon, WezTerm, Raycast, Zed, and IINA. You can later add your app choices there too."},
			mise: option{flag: "mise", title: "developer tools", help: "Installs developer tools like fzf, ripgrep and Go with mise. You can later add your own tools too."},
		},
		configure: configureChoices{
			defaultShell: option{flag: "default-shell", title: "Fish as login shell", help: "Super productive shell with autocompletion."},
			fish:         option{flag: "fish", title: "Fish config", help: "Setup your prompt, and fzf search. And creates default functions "},
			wezterm:      option{flag: "wezterm", title: "WezTerm config", help: "WezTerm follows system appearance for light/dark mode, the Hack Nerd Font, and cmd+shift+c to copy the output of the last command."},
			karabiner: appWithPermissions{
				config:          option{flag: "karabiner", title: "Karabiner config", help: "Caps Lock becomes Escape when you tap it and Hyper key when you hold it. cmd+shift+v pastes with no formatting."},
				permissionSetup: option{flag: "karabiner-permissions", title: "Karabiner permissions", help: "Guide you through granting permissions to Karabiner"},
			},
			hammerspoon: appWithPermissions{
				config:          option{flag: "hammerspoon", title: "Hammerspoon config", help: "Contains various mac automations and customizations."},
				permissionSetup: option{flag: "hammerspoon-permissions", title: "Hammerspoon permissions", help: "Guide you through granting permissions to Hammerspoon"},
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
		c.configure.hammerspoon.permissionSetup.on = false
		c.configure.karabiner.permissionSetup.on = false
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
