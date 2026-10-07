local windowFilter = require("hs.window.filter")
local FuzzyChooser = dofile(hs.spoons.resourcePath("fuzzy_chooser.lua"))

local function appIcon(app)
	local bundleId = app:bundleID()
	if not bundleId then
		return nil
	end
	return hs.image.imageFromAppBundle(bundleId)
end

local WindowChooser = {
	filter = windowFilter.new():setDefaultFilter({}):keepActive(),
	windowsById = {},
	chooser = nil,
	pointerTimer = nil,
}

function WindowChooser:load()
	self.windowsById = {}
	local choices = {}
	for _, win in ipairs(self.filter:getWindows(windowFilter.sortByFocusedLast)) do
		local id = win:id()
		local app = win:application()
		if id and app then
			local appName = app:name()
			local title = win:title()
			if title == "" then
				title = appName
			end
			self.windowsById[id] = win
			table.insert(choices, {
				text = title,
				subText = appName,
				image = appIcon(app),
				windowId = id,
			})
		end
	end
	return choices
end

function WindowChooser:focus(choice)
	if self.pointerTimer then
		self.pointerTimer:stop()
		self.pointerTimer = nil
	end
	local win = self.windowsById[choice.windowId]
	if not win then
		return
	end
	if win:isMinimized() then
		win:unminimize()
	end
	win:focus()
	-- Wait for focus and any Space transition before moving to the selected window.
	local attempts = 0
	self.pointerTimer = hs.timer.doEvery(0.1, function()
		attempts = attempts + 1
		local focused = hs.window.focusedWindow()
		if focused and focused:id() == choice.windowId then
			local frame = focused:frame()
			local pointer = hs.geometry(hs.mouse.absolutePosition())
			if not pointer:inside(frame) then
				hs.mouse.absolutePosition(frame.center)
			end
			self.pointerTimer:stop()
			self.pointerTimer = nil
		elseif attempts >= 30 then
			self.pointerTimer:stop()
			self.pointerTimer = nil
		end
	end)
end

function WindowChooser:show()
	self.chooser:show()
end

WindowChooser.chooser = FuzzyChooser.new("Window", function()
	return WindowChooser:load()
end, function(choice)
	WindowChooser:focus(choice)
end)

return WindowChooser
