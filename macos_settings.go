package main

func macOSSettingGroups() []settingGroup {
	return []settingGroup{
		{
			name: "Security",
			settings: []setting{
				{
					explanation: "Turns the firewall on, so only the services you allow can take connections from outside.",
					commands:    []string{"sudo defaults write /Library/Preferences/com.apple.alf globalstate -int 1"},
				},
				{
					explanation: "Keeps the mac silent when someone pings it, so it does not show up in network scans.",
					commands:    []string{"sudo defaults write /Library/Preferences/com.apple.alf stealthenabled -int 1"},
				},
				{
					explanation: "Stops other macs from sending AppleScript commands to this one.",
					commands:    []string{"sudo systemsetup -setremoteappleevents off"},
				},
				{
					explanation: "Keeps the mac asleep even when network traffic arrives for it.",
					commands:    []string{"sudo systemsetup -setwakeonnetworkaccess off"},
					optional:    true,
				},
				{
					explanation: "Removes the guest user from the login screen.",
					commands:    []string{"sudo defaults write /Library/Preferences/com.apple.loginwindow GuestEnabled -bool false"},
				},
			},
		},
		{
			name: "Trackpad, mouse and keyboard",
			settings: []setting{
				{
					explanation: "Counts a light tap as a click, both while you are logged in and on the login screen.",
					commands: []string{
						"defaults -currentHost write NSGlobalDomain com.apple.mouse.tapBehavior -int 1",
						"defaults write NSGlobalDomain com.apple.mouse.tapBehavior -int 1",
					},
				},
				{
					explanation: "Makes the bottom right corner of the trackpad act as a right click.",
					commands: []string{
						"defaults write com.apple.driver.AppleBluetoothMultitouch.trackpad TrackpadCornerSecondaryClick -int 2",
						"defaults write com.apple.driver.AppleBluetoothMultitouch.trackpad TrackpadRightClick -bool true",
						"defaults -currentHost write NSGlobalDomain com.apple.trackpad.trackpadCornerClickBehavior -int 1",
						"defaults -currentHost write NSGlobalDomain com.apple.trackpad.enableSecondaryClick -bool true",
					},
				},
				{
					explanation: "Moves the pointer fast, for the mouse and for the trackpad.",
					commands: []string{
						"defaults write NSGlobalDomain com.apple.mouse.scaling -float 4",
						"defaults write NSGlobalDomain com.apple.trackpad.scaling -float 10",
					},
				},
				{
					explanation: "Turns natural scrolling off, so the page moves down when you scroll down.",
					commands:    []string{"defaults write NSGlobalDomain com.apple.swipescrolldirection -bool false"},
				},
				{
					explanation: "Lets the tab key reach every control, including the buttons inside dialog boxes.",
					commands:    []string{"defaults write NSGlobalDomain AppleKeyboardUIMode -int 3"},
				},
				{
					explanation: "Repeats the letter when a key is held down, instead of opening the accent menu.",
					commands:    []string{"defaults write NSGlobalDomain ApplePressAndHoldEnabled -bool false"},
				},
				{
					explanation: "Repeats held keys at the fastest speed and waits less before the repeat starts.",
					commands: []string{
						"defaults write NSGlobalDomain KeyRepeat -int 1",
						"defaults write NSGlobalDomain InitialKeyRepeat -int 10",
					},
				},
				{
					explanation: "Stops the mac from changing words while you type.",
					commands:    []string{"defaults write NSGlobalDomain NSAutomaticSpellingCorrectionEnabled -bool false"},
				},
				{
					explanation: "Stops the offer to turn dictation on when the fn key is pressed twice.",
					commands:    []string{"defaults write com.apple.HIToolbox AppleDictationAutoEnable -int 0"},
				},
			},
		},
		{
			name: "Screen",
			settings: []setting{
				{
					explanation: "Asks for your password the moment the screen saver or sleep starts, with no free period.",
					commands: []string{
						"defaults write com.apple.screensaver askForPassword -int 1",
						"defaults write com.apple.screensaver askForPasswordDelay -int 0",
					},
				},
				{
					explanation: "Saves new screenshots on the desktop as png files, with no shadow around the window.",
					commands: []string{
						`defaults write com.apple.screencapture location -string "$HOME/Desktop"`,
						"defaults write com.apple.screencapture type -string png",
						"defaults write com.apple.screencapture disable-shadow -bool true",
					},
				},
				{
					explanation: "Brings the extra sharp display resolutions into the display settings after a restart.",
					commands:    []string{"sudo defaults write /Library/Preferences/com.apple.windowserver DisplayResolutionEnabled -bool true"},
				},
			},
		},
		{
			name: "Finder",
			settings: []setting{
				{
					explanation: "Keeps folders above the files in every list.",
					commands:    []string{"defaults write com.apple.finder _FXSortFoldersFirst -bool true"},
				},
				{
					explanation: "Sorts a list by the date a file last changed.",
					commands: []string{
						`plutil -replace StandardViewSettings.ListViewSettings.sortColumn -string dateModified "$HOME/Library/Preferences/com.apple.finder.plist"`,
						`plutil -replace StandardViewSettings.ExtendedListViewSettingsV2.sortColumn -string dateModified "$HOME/Library/Preferences/com.apple.finder.plist"`,
						`plutil -replace FK_StandardViewSettings.ListViewSettings.sortColumn -string dateModified "$HOME/Library/Preferences/com.apple.finder.plist"`,
						`plutil -replace FK_StandardViewSettings.ExtendedListViewSettingsV2.sortColumn -string dateModified "$HOME/Library/Preferences/com.apple.finder.plist"`,
					},
					optional: true,
				},
				{
					explanation: "Lets you quit Finder with command q, which also hides the icons on the desktop.",
					commands:    []string{"defaults write com.apple.finder QuitMenuItem -bool true"},
				},
				{
					explanation: "Removes the window and get info animations.",
					commands:    []string{"defaults write com.apple.finder DisableAllAnimations -bool true"},
				},
				{
					explanation: "Opens every new window in your home directory.",
					commands: []string{
						"defaults write com.apple.finder NewWindowTarget -string PfHm",
						`defaults write com.apple.finder NewWindowTargetPath -string "file://$HOME/"`,
					},
				},
				{
					explanation: "Shows hidden files, the ones whose name starts with a dot.",
					commands: []string{
						"defaults write com.apple.finder AppleShowAllFiles -bool true",
						"defaults write NSGlobalDomain AppleShowAllExtensions -bool true",
					},
				},
				{
					explanation: "Show the extension of files.",
					commands: []string{
						"defaults write NSGlobalDomain AppleShowAllExtensions -bool true",
					},
				},
				{
					explanation: "Shows the bar with the item count and free space, and the row of parent folders.",
					commands: []string{
						"defaults write com.apple.finder ShowStatusBar -bool true",
						"defaults write com.apple.finder ShowPathbar -bool true",
					},
				},
				{
					explanation: "Puts the full path of the folder in the window title.",
					commands:    []string{"defaults write com.apple.finder _FXShowPosixPathInTitle -bool true"},
				},
				{
					explanation: "Searches the folder you are in instead of the whole mac.",
					commands:    []string{"defaults write com.apple.finder FXDefaultSearchScope -string SCcf"},
				},
				{
					explanation: "Removes the question asked when you rename a file extension.",
					commands:    []string{"defaults write com.apple.finder FXEnableExtensionChangeWarning -bool false"},
				},
				{
					explanation: "Opens a folder on its own, with no wait, when you hold a dragged file over it.",
					commands: []string{
						"defaults write NSGlobalDomain com.apple.springing.enabled -bool true",
						"defaults write NSGlobalDomain com.apple.springing.delay -float 0",
					},
				},
				{
					explanation: "Stops Finder from leaving .DS_Store files on shared network drives.",
					commands:    []string{"defaults write com.apple.desktopservices DSDontWriteNetworkStores -bool true"},
				},
				{
					explanation: "Opens a window when a disk or a usb drive is plugged in.",
					commands: []string{
						"defaults write com.apple.frameworks.diskimages auto-open-ro-root -bool true",
						"defaults write com.apple.frameworks.diskimages auto-open-rw-root -bool true",
						"defaults write com.apple.finder OpenWindowForNewRemovableDisk -bool true",
					},
				},
				{
					explanation: "Makes the list view the one every window opens with.",
					commands:    []string{"defaults write com.apple.finder FXPreferredViewStyle -string Nlsv"},
				},
				{
					explanation: "Removes the question asked before the trash is emptied.",
					commands:    []string{"defaults write com.apple.finder WarnOnEmptyTrash -bool false"},
				},
				{
					explanation: "Opens the general, open with and permissions sections of the get info window.",
					commands:    []string{"defaults write com.apple.finder FXInfoPanesExpanded -dict General -bool true OpenWith -bool true Privileges -bool true"},
				},
			},
		},
		{
			name: "Dock and Mission Control",
			settings: []setting{
				{
					explanation: "Removes every pinned app, folder and file from the dock.",
					commands: []string{
						"defaults write com.apple.dock persistent-apps -array",
						"defaults write com.apple.dock persistent-others -array",
					},
				},
				{
					explanation: "Highlights the icon under the pointer when a stack opens as a grid.",
					commands:    []string{"defaults write com.apple.dock mouse-over-hilite-stack -bool true"},
				},
				{
					explanation: "Sets the dock icons to 36 pixels.",
					commands:    []string{"defaults write com.apple.dock tilesize -int 36"},
				},
				{
					explanation: "Shrinks a window straight into its own app icon instead of the genie curve.",
					commands: []string{
						"defaults write com.apple.dock mineffect -string scale",
						"defaults write com.apple.dock minimize-to-application -bool true",
					},
				},
				{
					explanation: "Opens an app when you hold a dragged file over its dock icon.",
					commands:    []string{"defaults write com.apple.dock enable-spring-load-actions-on-all-items -bool true"},
				},
				{
					explanation: "Shows the small dot under every app that is running.",
					commands:    []string{"defaults write com.apple.dock show-process-indicators -bool true"},
				},
				{
					explanation: "Stops the icon from bouncing while an app starts.",
					commands:    []string{"defaults write com.apple.dock launchanim -bool false"},
				},
				{
					explanation: "Makes the Mission Control animation almost instant and shows each window on its own.",
					commands: []string{
						"defaults write com.apple.dock expose-animation-duration -float 0.1",
						"defaults write com.apple.dock expose-group-by-app -bool false",
					},
				},
				{
					explanation: "Keeps the dock hidden and brings it back with no wait and no animation.",
					commands: []string{
						"defaults write com.apple.dock autohide-delay -float 0",
						"defaults write com.apple.dock autohide-time-modifier -float 0",
						"defaults write com.apple.dock autohide -bool true",
					},
				},
				{
					explanation: "Makes the icons of hidden apps half see through.",
					commands:    []string{"defaults write com.apple.dock showhidden -bool true"},
				},
				{
					explanation: "Keeps recent and suggested apps out of the dock.",
					commands:    []string{"defaults write com.apple.dock show-recents -bool false"},
				},
			},
		},
		{
			name: "Menubar settings",
			settings: []setting{
				{
					explanation: "Puts the WiFi icon in the menu bar.",
					commands:    []string{"defaults -currentHost write com.apple.controlcenter WiFi -int 8"},
				},
				{
					explanation: "Removes the Spotlight search icon from the menu bar.",
					commands:    []string{"defaults -currentHost write com.apple.Spotlight MenuItemHidden -int 1"},
				},
			},
		},
		{
			name: "Default apps",
			settings: []setting{
				{
					explanation: "Makes IINA the app that opens video files.",
					commands:    []string{"duti -s com.colliderli.iina public.movie all"},
					optional:    true,
				},
			},
		},
		{
			name: "Restart",
			settings: []setting{
				{
					explanation: "Restarts Finder, the menu bar and the dock so they read the new settings.",
					commands:    []string{"killall Finder", "killall SystemUIServer", "killall Dock", "killall ControlCenter"},
					optional:    true,
				},
			},
		},
	}
}
