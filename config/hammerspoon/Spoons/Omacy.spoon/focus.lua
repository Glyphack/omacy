-- Launching apps, focusing them, and rotating between their windows or
-- browser tabs from a single entry point.
---@class omacy.Focus
local focus = {}

local log = hs.logger.new("omacy.focus", "info")

local APP_LAUNCH_RETRY_DELAY = 0.3
local APP_LAUNCH_MAX_ATTEMPTS = 20

local focusHistory = {}
local currentFocusedId = nil
local previousFocusedId = nil

local function recordFocus(win)
	if not win then
		return
	end
	local id = win:id()
	if id == currentFocusedId then
		return
	end
	previousFocusedId = currentFocusedId
	currentFocusedId = id
end

function focusHistory.previousWindow()
	if not previousFocusedId then
		return nil
	end
	local win = hs.window.get(previousFocusedId)
	if win and win:isVisible() then
		return win
	end
	return nil
end

local focusWatcher = hs.window.filter.new():subscribe(hs.window.filter.windowFocused, recordFocus)

local function mouseToCenter(window)
	local current_pos = hs.geometry(hs.mouse.absolutePosition())
	local frame = window:frame()
	if not current_pos:inside(frame) then
		local current_screen = hs.mouse.getCurrentScreen()
		local window_screen = window:screen()
		if current_screen and window_screen and current_screen ~= window_screen then
			hs.mouse.absolutePosition(current_screen:frame().center)
			hs.mouse.absolutePosition(window_screen:frame().center)
		end
		hs.mouse.absolutePosition(frame.center)
	end
end

-- See https://www.hammerspoon.org/docs/hs.window.html#application
local function windowBelongsToApp(win, appName)
	local path = win:application():path()
	local nameOnDisk = string.gsub(path, "/Applications/", "")
	nameOnDisk = string.gsub(nameOnDisk, "%.app$", "")
	nameOnDisk = string.gsub(nameOnDisk, "/System/Library/CoreServices/", "")
	return nameOnDisk:find(appName, 1, true) ~= nil
end

local function escapeForAppleScript(text)
	return (text:gsub("\\", "\\\\"):gsub('"', '\\"'))
end

-- Tabs are driven over AppleScript, which covers Chromium based browsers such
-- as Chrome, Brave and Edge. macOS asks once for permission to control the
-- browser and every script here fails until that is granted.
local browserTabs = {}

function browserTabs.run(appName, body)
	local script = string.format('tell application "%s"\n%s\nend tell', escapeForAppleScript(appName), body)
	local ok, result, err = hs.osascript.applescript(script)
	if not ok then
		log.w("browser script failed for " .. appName .. ": " .. hs.inspect(err))
	end
	return ok, result
end

function browserTabs.focus(target)
	local pattern = escapeForAppleScript(target.tab)
	local ok, found = browserTabs.run(
		target.app,
		string.format(
			[[
	repeat with theWindow in windows
		set tabIndex to 0
		repeat with theTab in tabs of theWindow
			set tabIndex to tabIndex + 1
			if (title of theTab contains "%s") or (URL of theTab contains "%s") then
				set active tab index of theWindow to tabIndex
				set index of theWindow to 1
				return true
			end if
		end repeat
	end repeat
	return false]],
			pattern,
			pattern
		)
	)
	return ok and found == true
end

function browserTabs.isFocusedOn(target)
	local pattern = escapeForAppleScript(target.tab)
	local ok, found = browserTabs.run(
		target.app,
		string.format(
			[[
	if (count of windows) is 0 then
		return false
	end if
	if URL of active tab of front window contains "%s" then
		return true
	end if
	return false]],
			pattern
		)
	)
	return ok and found == true
end

function browserTabs.openURL(target)
	local url = escapeForAppleScript(target.tab)
	local ok = browserTabs.run(
		target.app,
		string.format(
			[[
	if (count of windows) is 0 then
		make new window
		set URL of active tab of front window to "%s"
	else
		tell front window to make new tab with properties {URL:"%s"}
		set index of front window to 1
	end if]],
			url,
			url
		)
	)
	return ok
end

-- Focuses the tab whose title or URL matches target.tab, and opens target.tab
-- as a new tab when no tab matches.
local function selectTabOrOpenURL(hsApp, target)
	if browserTabs.focus(target) then
		return
	end
	if browserTabs.openURL(target) then
		return
	end
	hs.urlevent.openURLWithBundle(target.tab, hsApp:bundleID())
end

local function focusPreviousOrHide(hsApp)
	local previous = focusHistory.previousWindow()
	if previous then
		previous:focus()
		mouseToCenter(previous)
		return
	end
	hsApp:hide()
end

local function rotateWindows(hsApp, appName)
	local appWindows = hsApp:allWindows()
	if #appWindows <= 1 then
		focusPreviousOrHide(hsApp)
		return
	end

	-- The window list order changes after one window gets focused,
	-- so directly bring the last one to focus every time
	-- https://www.hammerspoon.org/docs/hs.window.html#focus
	local targetWin = appWindows[#appWindows]
	if appName == "Finder" then
		-- Finder reports one more window than actually exists, so subtract one
		targetWin = appWindows[#appWindows - 1]
	end
	targetWin:focus()
	mouseToCenter(targetWin)
end

local function withApp(appName, fn, attempt)
	attempt = attempt or 1
	local hsApp = hs.application.get(appName)
	if hsApp then
		fn(hsApp)
		return
	end
	if attempt >= APP_LAUNCH_MAX_ATTEMPTS then
		log.w("App never showed up: " .. appName)
		return
	end
	hs.timer.doAfter(APP_LAUNCH_RETRY_DELAY, function()
		withApp(appName, fn, attempt + 1)
	end)
end

local function openTarget(target)
	hs.application.launchOrFocus(target.app)
	if target.tab then
		withApp(target.app, function(hsApp)
			selectTabOrOpenURL(hsApp, target)
		end)
	end
	hs.timer.doAfter(0.1, function()
		local win = hs.window.focusedWindow()
		if win then
			mouseToCenter(win)
		end
	end)
end

-- target is a table { app = "...", tab = "..." }.
-- app is app name. opens the app or focuses it's window. If it's window is already focused then it cycles between other windows of the app. If there are no other windows it moves the focus to previous window.
-- tab is a URL opens the tab with that url in the browser.
-- Must be provided with app = Brave Browser or app = Chrome. It will open the app and focus this tab or create it if does not exist.
function focus.launchOrFocusOrRotate(target)
	log.d("launchOrFocusOrRotate: " .. target.app .. (target.tab and (" tab=" .. target.tab) or ""))

	local focusedWindow = hs.window.focusedWindow()
	if not focusedWindow or not windowBelongsToApp(focusedWindow, target.app) then
		openTarget(target)
		return
	end

	local hsApp = focusedWindow:application()
	if not target.tab then
		rotateWindows(hsApp, target.app)
		return
	end

	if browserTabs.isFocusedOn(target) then
		focusPreviousOrHide(hsApp)
		return
	end
	selectTabOrOpenURL(hsApp, target)
end

return focus
