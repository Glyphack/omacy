# Omacy

Omacy is a MacOS setup to make it easy to customize your computer.

Install with:

```
curl -fsSL https://raw.githubusercontent.com/Glyphack/omacy/HEAD/install.sh | bash
```

It works both on a brand new mac and if you already have things installed.

## What It Does?

Running Omacy installs and configures the following apps:

### Homebrew

[Homebrew](https://docs.brew.sh/) is MacOS package manager.
You can install apps from command line and no longer have to drag things into Applications folder.

Installed along with a set of software to install:

```
cask "iina"
cask "monitorcontrol"
cask "brave-browser"
cask "zed"
```

### Fish

[Fish](https://fishshell.com/) is my default shell.
It works better than bash and zsh out of the box.
You have vim mode, hotkeys, autocompletion without the need to install any plugins.

It is installed and chosen as default shell.

### Wezterm

[Wezterm](https://github.com/wezterm/wezterm) is installed as terminal emulator.
It's customizable, you can add your hotkeys and customize the look and feel of the terminal emulator.

### Hammerspoon

[Hammerspoon](https://www.hammerspoon.org/) Lets you write Lua code to interact with MacOS API.
With Hammerspoon you can write your own hotkeys to perform actions or change system settings.

For example Omacy ships with a hotkey to mute your microphone globally.


### Karabiner-Elements

[Karabiner-Elements](http://karabiner-elements.pqrs.org/) is a keyboard customizer for MacOS.
You can remap keys and change what they do.

### Mise

[Mise](https://mise.jdx.dev/) installs all your developer tools like compilers and command-line tools.

### Raycast

Raycast is a nice tool but it's not configurable from the command line. It's installed but you have to configure it yourself.

Suggested hotkeys to set:

- Clipboard history
- Raycast notes

These two features are so well designed that I prefer using the tool just for these.
I also use it to open asks given the unusable state of MacOS's Spotlight.

## Mac Settings

Omacy sets sane defaults in MacOS settings app.

The full list of these settings are:

Trackpad, mouse and keyboard
- Counts a light tap as a click, both after login and on the login screen.
- Makes the bottom right corner of the trackpad act as a right click.
- Speeds up pointer movement for both mouse and trackpad.
- Turns natural scrolling off, so the page moves down when you scroll down.
- Lets the tab key reach every control, including buttons inside dialog boxes.
- Repeats the letter when you hold a key, instead of showing the accent menu.
- Sets the fastest key repeat speed and a short wait before repeating starts.
- Turns off automatic spelling correction while you type.
- Stops the offer to enable dictation when you press fn twice.

Security
- Turns the firewall on, so only apps you allow can accept connections from outside.
- Turns on stealth mode, so the mac stays silent when someone pings it and does not show up in network scans.
- Stops other macs from sending AppleScript commands to this one (remote Apple events).
- Stops the mac from waking up when network traffic arrives for it. Marked optional.
- Removes the guest user from the login screen.

Menu bar
- Puts the WiFi icon in the menu bar.
- Removes the Spotlight search icon from the menu bar.

Dock and Mission Control
- Removes every pinned app, folder and file from the dock. So you can pin what you want.
- Highlights the icon under the pointer when a stack opens as a grid.
- Sets dock icon size to 36 pixels.
- Minimizes windows into their own app icon using the scale effect instead of the genie curve.
- Opens an app when you hold a dragged file over its dock icon.
- Shows the small dot under every running app.
- Stops the icon from bouncing while an app is starting.
- Makes the Mission Control animation almost instant and shows each window on its own instead of grouped by app.
- Hides the dock and brings it back with no delay and no animation.
- Makes icons of hidden apps half transparent.
- Keeps recent and suggested apps out of the dock.

Screen
- Asks for your password right away when the screen saver or sleep starts, with no grace period.
- Saves screenshots to the Desktop as png files with no window shadow.
- Enables extra sharp display resolutions in display settings, visible after a restart.

Finder
- Shows file extensions.
- Sorts folders above files in every list.
- Sorts lists by the date a file was last changed. Marked optional.
- Lets you quit Finder with command q, which also hides the desktop icons.
- Removes window and get info animations.
- Opens every new window in your home directory.
- Shows hidden files, the ones whose name starts with a dot.
- Shows the status bar with item count and free space, and the path bar with parent folders.
- Puts the full folder path in the window title.
- Makes search start in the current folder instead of the whole mac.
- Removes the warning when you rename a file extension.
- Opens a folder instantly when you hold a dragged file over it (spring loading).
- Stops Finder from writing .DS_Store files on shared network drives.
- Opens a window when a disk or usb drive is plugged in.
- Makes list view the default view for new windows.
- Removes the confirmation before emptying the trash.
- Expands the general, open with and permissions sections of the get info window.

Default apps
- Makes IINA the default app for video files. Marked optional.


## Usage

All of the default behavior can be configured from config files of the tools.
The customization is respected if you reinstall Omacy.

### Keyboard

Keyboard mappings are defined as karabiner rules. You can find them in Karabiner complex modifications section.

1. Caps Lock

When Caps Lock is held it's mapped to holding `CMD + Option + Ctrl`. This is an unutilized combination that no apps have hotkeys for.
So you can set hotkeys on Caps Lock + Any other key without conflict with other apps.

When Caps Lock is pressed it acts as escape key, a must have for any vim user.


2. Copy without formatting

Isn't it so annoying that you copy a text from a website and when pasting it somewhere it makes the font bold and shit?
Yeah that's gone.

### Hotkeys

Mostly configured inside Hammerspoon.

Shortcuts use Hyper Key you can customize it in hammerspoon config. By default this key is `CMD + Option + Ctrl`.

| Hotkey | Function |
| --- | --- |
| Hyper + A | Move current window to left half of the screen |
| Hyper + D | Move current window to right half of the screen |
| Hyper + W | Move current window to top half of the screen |
| Hyper + S | Move current window to bottom half of the screen |
| Hyper + C | Center the current window |
| Hyper + ] | Move the current window to the next screen |
| Hyper + [ | Move the current window to the previous screen |
| Hyper + I | Fill the screen with the current window, press again to undo |
| Hyper + J | Open Brave Browser|
| Hyper + K | Open WezTerm |
| Hyper + T | Mute or unmute the microphone |
| Ctrl + ` | Reload the Hammerspoon config |

### Fish

A set of fish functions ship with the default fish config.

`,cp` to copy any file within the terminal to clipboard. You can copy from the terminal then you can paste it to the browser.
`,pf` paste the file that you have copied into current directory. It also works with screenshots saved into clipboard.
`ntfy` a command to send a push notification to yourself using [ntfy](https://docs.ntfy.sh). `ntfy 10min cool!`.

### Wezterm

To enable the wezterm plugin do:

```
require("omacy").apply_to_config(config)
```

- `CMD + Shift + C` will copy the last command output to clipboard.
- Terminal theme is set to light/dark [flexoki](https://github.com/kepano/flexoki) based on system appearance setting.
