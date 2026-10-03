package main

func macOSSettingGroups() settingGroups {
	return settingGroups{
		{
			name: "Security",
			settings: []setting{
				{
					option:   option{flag: "macos-firewall", title: "Firewall", help: "Turns the firewall on, so only apps you allow can accept connections from outside."},
					commands: []string{"sudo /usr/libexec/ApplicationFirewall/socketfilterfw --setglobalstate on"},
				},
				{
					option:   option{flag: "macos-no-wake-on-network", title: "No wake for network", help: "Keeps the mac asleep even when network traffic arrives for it."},
					commands: []string{"sudo systemsetup -setwakeonnetworkaccess off"},
				},
			},
		},
		{
			name: "Trackpad, mouse and keyboard",
			settings: []setting{
				{
					option: option{flag: "macos-fast-pointer", title: "Fast pointer", help: "Moves the pointer fast, for the mouse and for the trackpad."},
					commands: []string{
						"defaults write NSGlobalDomain com.apple.mouse.scaling -float 4",
						"defaults write NSGlobalDomain com.apple.trackpad.scaling -float 10",
					},
				},
				{
					option:   option{flag: "macos-no-natural-scrolling", title: "No natural scrolling", help: "Turns natural scrolling off, so the page moves down when you scroll down."},
					commands: []string{"defaults write NSGlobalDomain com.apple.swipescrolldirection -bool false"},
				},
				{
					option:   option{flag: "macos-hold-to-repeat", title: "Hold a key to repeat it", help: "Repeats the letter when a key is held down, instead of opening the accent menu."},
					commands: []string{"defaults write NSGlobalDomain ApplePressAndHoldEnabled -bool false"},
				},
				{
					option: option{flag: "macos-fast-key-repeat", title: "Fast key repeat", help: "Repeats held keys at the fastest speed and waits less before the repeat starts."},
					commands: []string{
						"defaults write NSGlobalDomain KeyRepeat -int 1",
						"defaults write NSGlobalDomain InitialKeyRepeat -int 10",
					},
				},
				{
					option:   option{flag: "macos-no-autocorrect", title: "No autocorrect", help: "Stops the mac from changing words while you type."},
					commands: []string{"defaults write NSGlobalDomain NSAutomaticSpellingCorrectionEnabled -bool false"},
				},
			},
		},
		{
			name: "Screen",
			settings: []setting{
				{
					option: option{flag: "macos-screenshots", title: "Screenshots as png on desktop", help: "Saves new screenshots on the desktop as png files, with no shadow around the window."},
					commands: []string{
						`defaults write com.apple.screencapture location -string "$HOME/Desktop"`,
						"defaults write com.apple.screencapture type -string png",
						"defaults write com.apple.screencapture disable-shadow -bool true",
					},
				},
			},
		},
		{
			name: "Accessibility",
			settings: []setting{
				{
					option: option{flag: "macos-reduce-motion", title: "Reduce motion", help: "Turns on reduce motion, so switching spaces and opening windows fade instead of slide."},
					commands: []string{
						"defaults write com.apple.universalaccess reduceMotion -bool true",
						"defaults write com.apple.Accessibility ReduceMotionEnabled -int 1",
					},
				},
			},
		},
		{
			name: "Finder",
			settings: []setting{
				{
					option:   option{flag: "macos-finder-folders-first", title: "Folders first", help: "Keeps folders above the files in every list."},
					commands: []string{"defaults write com.apple.finder _FXSortFoldersFirst -bool true"},
				},
				{
					option: option{flag: "macos-finder-sort-by-date", title: "Sort by date modified", help: "Sorts a list by the date a file last changed."},
					commands: []string{
						`plutil -replace StandardViewSettings.ListViewSettings.sortColumn -string dateModified "$HOME/Library/Preferences/com.apple.finder.plist"`,
						`plutil -replace StandardViewSettings.ExtendedListViewSettingsV2.sortColumn -string dateModified "$HOME/Library/Preferences/com.apple.finder.plist"`,
						`plutil -replace FK_StandardViewSettings.ListViewSettings.sortColumn -string dateModified "$HOME/Library/Preferences/com.apple.finder.plist"`,
						`plutil -replace FK_StandardViewSettings.ExtendedListViewSettingsV2.sortColumn -string dateModified "$HOME/Library/Preferences/com.apple.finder.plist"`,
					},
				},
				{
					option:   option{flag: "macos-finder-quit", title: "Quit with command q", help: "Lets you quit Finder with command q, which also hides the icons on the desktop."},
					commands: []string{"defaults write com.apple.finder QuitMenuItem -bool true"},
				},
				{
					option:   option{flag: "macos-finder-no-animations", title: "No animations", help: "Removes the window and get info animations."},
					commands: []string{"defaults write com.apple.finder DisableAllAnimations -bool true"},
				},
				{
					option: option{flag: "macos-finder-open-home", title: "New windows open home", help: "Opens every new window in your home directory."},
					commands: []string{
						"defaults write com.apple.finder NewWindowTarget -string PfHm",
						`defaults write com.apple.finder NewWindowTargetPath -string "file://$HOME/"`,
					},
				},
				{
					option:   option{flag: "macos-finder-hidden-files", title: "Show hidden files", help: "Shows hidden files, the ones whose name starts with a dot."},
					commands: []string{"defaults write com.apple.finder AppleShowAllFiles -bool true"},
				},
				{
					option: option{flag: "macos-finder-extensions", title: "Show file extensions", help: "Shows the extension of every file."},
					commands: []string{
						"defaults write NSGlobalDomain AppleShowAllExtensions -bool true",
					},
				},
				{
					option: option{flag: "macos-finder-status-bars", title: "Show status and path bars", help: "Shows the bar with the item count and free space, and the row of parent folders."},
					commands: []string{
						"defaults write com.apple.finder ShowStatusBar -bool true",
						"defaults write com.apple.finder ShowPathbar -bool true",
					},
				},
				{
					option:   option{flag: "macos-finder-path-in-title", title: "Full path in title", help: "Puts the full path of the folder in the window title."},
					commands: []string{"defaults write com.apple.finder _FXShowPosixPathInTitle -bool true"},
				},
				{
					option:   option{flag: "macos-finder-search-folder", title: "Search the current folder", help: "Searches the folder you are in instead of the whole mac."},
					commands: []string{"defaults write com.apple.finder FXDefaultSearchScope -string SCcf"},
				},
				{
					option:   option{flag: "macos-finder-no-extension-warning", title: "No extension change warning", help: "Removes the question asked when you rename a file extension."},
					commands: []string{"defaults write com.apple.finder FXEnableExtensionChangeWarning -bool false"},
				},
				{
					option: option{flag: "macos-finder-spring-loading", title: "Open folders on drag hover", help: "Opens a folder on its own, with no wait, when you hold a dragged file over it."},
					commands: []string{
						"defaults write NSGlobalDomain com.apple.springing.enabled -bool true",
						"defaults write NSGlobalDomain com.apple.springing.delay -float 0",
					},
				},
				{
					option:   option{flag: "macos-finder-no-network-ds-store", title: "No .DS_Store on network drives", help: "Stops Finder from leaving .DS_Store files on shared network drives."},
					commands: []string{"defaults write com.apple.desktopservices DSDontWriteNetworkStores -bool true"},
				},
				{
					option: option{flag: "macos-finder-open-disk-images", title: "Open mounted disk images", help: "Opens a window when a disk image is mounted."},
					commands: []string{
						"defaults write com.apple.frameworks.diskimages auto-open-ro-root -bool true",
						"defaults write com.apple.frameworks.diskimages auto-open-rw-root -bool true",
					},
				},
				{
					option:   option{flag: "macos-finder-list-view", title: "List view everywhere", help: "Makes the list view the one every window opens with."},
					commands: []string{"defaults write com.apple.finder FXPreferredViewStyle -string Nlsv"},
				},
				{
					option:   option{flag: "macos-finder-no-trash-warning", title: "No empty trash warning", help: "Removes the question asked before the trash is emptied."},
					commands: []string{"defaults write com.apple.finder WarnOnEmptyTrash -bool false"},
				},
				{
					option:   option{flag: "macos-finder-info-panes", title: "Open get info sections", help: "Opens the general, open with and permissions sections of the get info window."},
					commands: []string{"defaults write com.apple.finder FXInfoPanesExpanded -dict General -bool true OpenWith -bool true Privileges -bool true"},
				},
			},
		},
		{
			name: "Dock and Mission Control",
			settings: []setting{
				{
					option: option{flag: "macos-dock-clear", title: "Clear the dock", help: "Removes every pinned app, folder and file from the dock."},
					commands: []string{
						"defaults write com.apple.dock persistent-apps -array",
						"defaults write com.apple.dock persistent-others -array",
					},
				},
				{
					option:   option{flag: "macos-dock-stack-highlight", title: "Highlight in stacks", help: "Highlights the icon under the pointer when a stack opens as a grid."},
					commands: []string{"defaults write com.apple.dock mouse-over-hilite-stack -bool true"},
				},
				{
					option:   option{flag: "macos-dock-icon-size", title: "36 pixel icons", help: "Sets the dock icons to 36 pixels."},
					commands: []string{"defaults write com.apple.dock tilesize -int 36"},
				},
				{
					option: option{flag: "macos-dock-minimize-to-app", title: "Minimize into app icon", help: "Shrinks a window straight into its own app icon instead of the genie curve."},
					commands: []string{
						"defaults write com.apple.dock mineffect -string scale",
						"defaults write com.apple.dock minimize-to-application -bool true",
					},
				},
				{
					option:   option{flag: "macos-dock-spring-loading", title: "Open apps on drag hover", help: "Opens an app when you hold a dragged file over its dock icon."},
					commands: []string{"defaults write com.apple.dock enable-spring-load-actions-on-all-items -bool true"},
				},
				{
					option:   option{flag: "macos-dock-running-dots", title: "Dots under running apps", help: "Shows the small dot under every app that is running."},
					commands: []string{"defaults write com.apple.dock show-process-indicators -bool true"},
				},
				{
					option:   option{flag: "macos-dock-no-bounce", title: "No bounce on launch", help: "Stops the icon from bouncing while an app starts."},
					commands: []string{"defaults write com.apple.dock launchanim -bool false"},
				},
				{
					option:   option{flag: "macos-mission-control-ungroup", title: "Windows apart in Mission Control", help: "Shows each window on its own in Mission Control instead of grouping them by app."},
					commands: []string{"defaults write com.apple.dock expose-group-apps -bool false"},
				},
				{
					option: option{flag: "macos-dock-autohide", title: "Hide the dock, no delay", help: "Keeps the dock hidden and brings it back with no wait and no animation."},
					commands: []string{
						"defaults write com.apple.dock autohide-delay -float 0",
						"defaults write com.apple.dock autohide-time-modifier -float 0",
						"defaults write com.apple.dock autohide -bool true",
					},
				},
				{
					option:   option{flag: "macos-dock-no-recents", title: "No recent apps", help: "Keeps recent and suggested apps out of the dock."},
					commands: []string{"defaults write com.apple.dock show-recents -bool false"},
				},
				{
					option: option{flag: "macos-no-hot-corners", title: "No hot corners", help: "Turns off all four hot corners, so moving the pointer to a screen corner does nothing."},
					commands: []string{
						"defaults write com.apple.dock wvous-tl-corner -int 1",
						"defaults write com.apple.dock wvous-tl-modifier -int 0",
						"defaults write com.apple.dock wvous-tr-corner -int 1",
						"defaults write com.apple.dock wvous-tr-modifier -int 0",
						"defaults write com.apple.dock wvous-bl-corner -int 1",
						"defaults write com.apple.dock wvous-bl-modifier -int 0",
						"defaults write com.apple.dock wvous-br-corner -int 1",
						"defaults write com.apple.dock wvous-br-modifier -int 0",
					},
				},
			},
		},
		{
			name: "Menubar",
			settings: []setting{
				{
					option:   option{flag: "macos-menubar-no-wifi", title: "No WiFi icon", help: "Removes the WiFi icon from the menu bar."},
					commands: []string{"defaults -currentHost write com.apple.controlcenter WiFi -int 8"},
				},
				{
					option:   option{flag: "macos-menubar-no-spotlight", title: "No Spotlight icon", help: "Removes the Spotlight search icon from the menu bar."},
					commands: []string{"defaults -currentHost write com.apple.Spotlight MenuItemHidden -int 1"},
				},
			},
		},
		{
			name: "Default apps",
			settings: []setting{
				{
					option: option{flag: "macos-iina-videos", title: "IINA for videos", help: "Makes IINA the app that opens video files."},
					commands: []string{
						"duti -s com.colliderli.iina public.movie all",
						"duti -s com.colliderli.iina com.apple.quicktime-movie all",
						"duti -s com.colliderli.iina public.avi all",
						"duti -s com.colliderli.iina public.mpeg-4 all",
						"duti -s com.colliderli.iina .mkv all",
					},
				},
			},
		},
	}
}
