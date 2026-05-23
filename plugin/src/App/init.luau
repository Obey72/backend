--!strict
local App = {}

local PLUGIN_ID = "rbxassetid://89096096219225"

local UiLibrary = require("./UiLibrary")

local currentVersion = script.Parent.Version.Value

-- distinct re/up theme registered before any tab loads so the dashboard sees it
-- characterized by mint accents monospace font dark slate background no borders
-- why "code" instead of jetbrainsmono: jetbrainsmono is fetched lazily from a cdn
-- and some users see "temp read failed" which leaves every text widget invisible
-- and makes the plugin look blank "code" is a guaranteed built-in monospace
local REUP_THEME = {
	Name = "re/up",
	Font = "Code",
	TextSize = 13,
	BackgroundColor = { 6, 6, 10 },
	ForegroundColor = { 14, 14, 22 },
	BorderColor = { 32, 32, 48 },
	UnselectedColor = { 22, 22, 34 },
	SelectedColor = { 122, 240, 212 },
	TextColor = { 232, 232, 238 },
	DimmedTextColor = { 110, 110, 130 },
	InputColor = { 232, 232, 238 },
	TipColor = { 122, 240, 212 },
	ToggleBackgroundColor = { 122, 240, 212 },
	StrokeTransparency = 1,
}

-- filter must run first because it sets up _ggetplacelist that dashboard reads
-- themes comes after settings so the re/up default is already registered when it loads
local tabOrder = {
	"Filter",
	"Dashboard",
	"AnimReup",
	"Replace",
	"Settings",
	"Themes",
}

local function getLatestVersion(): string
	local success, objects = pcall(game.GetObjects, game, PLUGIN_ID)
	if not success then return currentVersion end

	local latestPlugin = (objects :: any)[1]
	local latestVersion = latestPlugin.Version.Value
	latestPlugin:Destroy()

	return latestVersion
end

-- always apply the re/up theme on init ignore any saved theme
-- this is what gives the plugin its distinct visual identity vs bacfr's default studio theme
local function applyReupTheme(ui: UiLibrary.Ui, plugin: Plugin)
	local existingId
	for id, theme in ui.theme.getThemes() do
		if (theme :: any).Name == "re/up" then
			existingId = id
			break
		end
	end
	if existingId then
		ui.theme.set(existingId)
		ui:UpdateTheme()
		return
	end
	local ok, newId = pcall(ui.theme.add, REUP_THEME)
	if ok then
		ui.theme.set(newId :: string)
		ui:UpdateTheme()
	end
end

function App.init(ui: UiLibrary.Ui, plugin: Plugin)
	-- apply the theme before tabs load so widgets render with the correct font/colors first try
	-- wrapped in pcall so a font/theme error never prevents the plugin from loading
	local themeOk, themeErr = pcall(applyReupTheme, ui, plugin)
	if not themeOk then
		warn("re/up: could not apply custom theme, using default. Error: " .. tostring(themeErr))
	end

	for _, tabName in tabOrder do
		local tab = script.Tabs:FindFirstChild(tabName)
		assert(tab, "tab not found: " .. tabName)
		task.spawn(require(tab), ui, plugin)
	end

	-- re-apply once everything is built so any late-bound widgets pick it up
	task.defer(function()
		pcall(applyReupTheme, ui, plugin)
	end)

	if currentVersion ~= getLatestVersion() then
		ui:Notify("Notification", "Plugin is out of date.")
	end
end

return App