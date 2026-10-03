# Omacy

Omacy is a customizable and extensible setup that turns your Mac into a keyboard driven environment for programming.

Install:

```
curl -fsSL https://raw.githubusercontent.com/Glyphack/omacy/HEAD/install.sh | bash
```

It works both on a brand new Mac and if you already have things installed.


## Features

- Sets sane defaults for settings, for example holding down a key repeats faster.
- Installs apps and tools using Mise and Homebrew so no manual installation is needed.
- Creates keyboard shortcuts for managing windows, launching apps, and more.
- You can customize anything.

## Manual

This section explains how to use your computer after installing Omacy.
If you'd like to know what is installed and how things are configured read [under the hood](#under-the-hood).

### Navigation

Everything can be done with the keyboard. And customizing it can be done with Hammerspoon.

After installation you have a new caps lock key. The new behavior is:

- When held it's mapped to holding `CMD + Option + Ctrl`. This is called the hyper key.
- When pressed it acts as escape key, a must have for any vim user.

This is an unutilized combination that no apps have hotkeys for.
So you can set hotkeys on `caps lock + any other key` without conflict with other apps.
(If you're wondering why shift key is left out from the combination [read this](https://www.reddit.com/r/osx/comments/4ooxu0/how_do_i_disable_sysdiagnose_generation_shortcut/).)

#### Opening Apps

Let's start with opening apps:

| Hotkey | Function |
| --- | --- |
| `Hyper + J` | Open Brave Browser|
| `Hyper + K` | Open WezTerm |
| `Hyper + M` | Show a list of open windows and focus the selected one |

Pressing the hotkey launches the application or focuses the app if it's already running.
If an app has multiple windows pressing repeatedly will cycle through different windows of the app.

You can override or add new hotkeys to launch apps[^hammerspoon-config]:

```lua
omacy:map(omacy.HYPER, "o", function() omacy.focus.launchOrFocusOrRotate({ app = "md.obsidian" }) end, "Obsidian")
```

The app name is bundle ID of the application.
To find the bundle ID of an app run `osascript -e 'id of app "Obsidian"'`.

For other apps that do not have a hotkey use `Hyper + M` for the window switcher.

#### Browser Tabs

| Hotkey | Function |
| --- | --- |
| `Hyper + B` | Pick a Brave Browser bookmark and open it |

You can also launch or jump to a tab[^hammerspoon-config]:

```lua
omacy:map(omacy.HYPER, "1", function()
	omacy.focus.launchOrFocusOrRotate({
		app = "com.brave.Browser",
		url = "https://music.youtube.com",
	})
end, "YouTube Music")
```

### Windows

You can also resize and move windows with the keyboard.

| Hotkey | Function |
| --- | --- |
| `Hyper + A` | Move current window to left half of the screen |
| `Hyper + D` | Move current window to right half of the screen |
| `Hyper + W` | Move current window to top half of the screen |
| `Hyper + S` | Move current window to bottom half of the screen |
| `Hyper + C` | Center the current window |
| `Hyper + I` | Maximize the current window |
| `Hyper + G` | Partitions your screen into a grid and select where you want the app to be. |

### Screens

If you have multiple screens you can move the current window between screens.

| Hotkey | Function |
| --- | --- |
| `Hyper + ]` | Move the current window to the next screen |
| `Hyper + [` | Move the current window to the previous screen |

### Brightness

Display brightness of connected monitor can be done with the `f1` & `f2` keys.
This is done by [MonitorControl](https://github.com/MonitorControl/MonitorControl).

### Audio

You can mute/unmute your mic globally.
This is not tied to a specific app so works in everything.
When the mic is muted an indicator is shown in the menu bar.

| Hotkey | Function |
| --- | --- |
| `Hyper + T` | Mute or unmute the microphone |

You can define a priority list of microphones and speakers and the top available one is always active.
So if you connect or disconnect a device only the ones you like stay activated[^hammerspoon-config].

```lua
omacy.audio.prefer({
	output = { "WH-1000XM5", "MacBook Pro Speakers" },
	input = { "Yeti Stereo Microphone", "MacBook Pro Microphone" },
})
```

### Apps

Other than the set of apps described these apps are installed.

IINA is a good media player. It's also set as your default app to open video files.

Brave Browser can replace Google Chrome with built-in ad-blocker.

Zed is a good editor to replace Xcode and TextEdit for previewing files.

Raycast is a very good tool. It can actually replace a lot of custom built functionalities but it does not allow configuration through code. So just installed for app launcher (Spotlight does anything except finding apps) and commands.

Suggested hotkeys to set:

- [Clipboard history](https://manual.raycast.com/clipboard-history)
- [Notes](https://manual.raycast.com/notes)
- [Commands](https://manual.raycast.com/script-commands)

#### Quiet Start on Login

You can launch and hide apps on start[^hammerspoon-config].
This was removed from macOS a few years ago but we have it!

```lua
omacy.autostart.launch({ "com.brave.Browser", "com.raycast.macos" })
```

#### Installing Apps

Omacy uses [bundle file](https://docs.brew.sh/Brew-Bundle-and-Brewfile) for managing macOS apps.
It's a list of all the apps you want installed on your Mac.
This allows you to replicate the setup on other machines by copying the file.

To install other apps:

- If temporarily, use `brew install` followed by the app name.
- If persistent, use `omacy brew edit` and add the app. Then run `omacy brew sync`.

Syncing always installs all the listed apps and removes the ones that are not listed but installed with brew.
So this allows you to try apps and remove them later with a sync.

### Keyboard & Mouse

#### Mouse Auto Scrolling

Auto scrolling is a feature that Windows & Linux have where you can scroll by holding the mouse wheel instead of turning it.
This is available and scrolls horizontally and vertically by holding the mouse middle button and moving your mouse around.

#### Paste and Match Formatting

Have you noticed when you copy something across apps it is pasted with different font and formatting?
To disable this you usually have to use `CMD + Shift + V` but this is not universal, some apps require `CMD + Option + Shift + V`.

I wish this could become the default behavior for pasting but it breaks for images.
But at least it's unified now, `CMD + Shift + V` works in any app.

### Terminal & Shell

WezTerm is installed and the shortcut `Hyper + K` opens it.
Its default theme is [flexoki](https://stephango.com/flexoki) and it changes to light/dark mode based on system appearance setting.
After running any command `CMD + Shift + C` will copy its output to clipboard.

[Fish](https://fishshell.com/) is the default shell because it has way better default functionality than bash and zsh.
You have hotkeys and autocompletion out of the box.
This means you don't need to install thousands of plugins to get basic functionalities.

Fish is configured with vi-style bindings so after pressing escape you can use vim motions to navigate the prompt.

[fzf](https://github.com/junegunn/fzf) adds these hotkeys to fish:

| Hotkey | Function |
| --- | --- |
| `Ctrl + R` | Search the command history |
| `Ctrl + T` | Find a file and add its path to the command |
| `Option + C` | Find a folder and go into it |

A set of fish functions ships with the default fish config.

- `,cp` to copy any file within the terminal to clipboard. You can copy from the terminal then you can paste it to the browser.
- `,pf` paste the file that you have copied into current directory. It also works with screenshots saved into clipboard.

### Notifications

The `ntfy` command in the terminal uses [ntfy](https://docs.ntfy.sh) to send a push notification to your phone or laptop.
Simply type `ntfy 10min tea` and you will receive this in 10 minutes.

To set up `ntfy` you first need to run `set -U ntfy <topic>` and set your topic.

### Development Tools

By default only a number of dev tools and CLI apps are installed.

- Go compiler
- Fzf for fuzzy finding in the terminal.
- ripgrep as a more user friendly grep. Check its [guide](https://github.com/BurntSushi/ripgrep/blob/master/GUIDE.md).
- [fd](https://github.com/sharkdp/fd) as a more user friendly find command.
- `yt-dlp` for downloading videos from the web.

These are all installed with mise. You can edit your [config](https://mise.jdx.dev/configuration.html) and run `mise i` to install more tools.

### Mac Settings

Omacy sets sane defaults in macOS settings app.

The full list of these settings is:

Trackpad, mouse and keyboard
- Speeds up pointer movement for both mouse and trackpad.
- Turns natural scrolling off, so the page moves down when you scroll down.
- Repeats the letter when you hold a key, instead of showing the accent menu.
- Sets the fastest key repeat speed and a short wait before repeating starts.
- Turns off automatic spelling correction while you type.

Security
- Turns the firewall on, so only apps you allow can accept connections from outside.
- Stops the Mac from waking up when network traffic arrives for it.

Menu bar
- Removes the WiFi icon from the menu bar.
- Removes the Spotlight search icon from the menu bar.

Dock and Mission Control
- Removes every pinned app, folder and file from the dock. So you can pin what you want.
- Highlights the icon under the pointer when a stack opens as a grid.
- Sets dock icon size to 36 pixels.
- Minimizes windows into their own app icon using the scale effect instead of the genie curve.
- Opens an app when you hold a dragged file over its dock icon.
- Stops the icon from bouncing while an app is starting.
- Hides the dock and brings it back with no delay and no animation.
- Keeps recent and suggested apps out of the dock.
- Turns off the bottom right hot corner, so moving the pointer there no longer opens Quick Note.

Screen
- Takes window screenshots with no shadow around the window.

Accessibility
- Turns on reduce motion, so switching spaces and opening windows fade instead of slide.

Finder
- Shows file extensions.
- Sorts folders above files in every list.
- Sorts lists by the date a file was last changed.
- Lets you quit Finder with `CMD + Q`, which also hides the desktop icons.
- Removes window and get info animations.
- Opens every new window in your home directory.
- Shows hidden files, the ones whose name starts with a dot.
- Shows the status bar with item count and free space, and the path bar with parent folders.
- Puts the full folder path in the window title.
- Makes search start in the current folder instead of the whole Mac.
- Removes the warning when you rename a file extension.
- Opens a folder instantly when you hold a dragged file over it (spring loading).
- Stops Finder from writing `.DS_Store` files on shared network drives.
- Opens a window when a disk image is mounted.
- Makes list view the default view for new windows.
- Removes the confirmation before emptying the trash.
- Expands the general, open with and permissions sections of the get info window.

Default apps
- Makes IINA the default app for video files.

## Under the Hood

Running Omacy installs and configures the following apps:

[Homebrew](https://brew.sh/) is installed and more apps are installed with it.

Brew file path: `~/.config/omacy/Brewfile`

[Fish](https://fishshell.com/) is installed and chosen as default shell.

Config: `~/.config/fish/conf.d/omacy.fish` and `~/.config/fish/omacy`

[WezTerm](https://github.com/wezterm/wezterm) is installed as terminal emulator.

Config: `~/.config/wezterm/omacy`

[Hammerspoon](https://www.hammerspoon.org/) lets you write Lua code to interact with macOS API.

Spoon path: `~/.hammerspoon/Spoons/Omacy.spoon`

[Karabiner-Elements](http://karabiner-elements.pqrs.org/) is a keyboard customizer for macOS.

Config is written to your normal configuration.

[Mise](https://mise.jdx.dev/) to install developer tools.

Config: `~/.config/mise/conf.d/omacy.toml`

[Raycast](https://www.raycast.com/) is installed but not configured.

[^hammerspoon-config]: Put this code in `~/.hammerspoon/init.lua` between the `Omacy load` and `Omacy apply` blocks, then reload Hammerspoon from its menu bar icon.
