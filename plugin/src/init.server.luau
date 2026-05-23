--!strict
local RunService = game:GetService("RunService")

if not RunService:IsEdit() then
	return
end

-- register toolbar + button + widget before any require so a downstream module
-- error never prevents the plugin from showing on the plugins tab
local toolbar = plugin:CreateToolbar("re/up")
local widget = plugin:CreateDockWidgetPluginGui(
	"reup",
	-- 560x340 fits the dashboard with status labels and asset type buttons
	DockWidgetPluginGuiInfo.new(Enum.InitialDockState.Float, false, true, 560, 340, 480, 280)
)
widget.Title = `re/up`

local toggleButton = toolbar:CreateButton("Toggle Gui", "toggle reup plugin", "rbxassetid://72408820289994")
toggleButton.Click:Connect(function() widget.Enabled = not widget.Enabled end)
widget:GetPropertyChangedSignal("Enabled"):Connect(function() toggleButton:SetActive(widget.Enabled) end)

if plugin:GetSetting("Port") == nil then
	plugin:SetSetting("Port", 38073)
end

local ok, err = pcall(function()
	local UiLibrary = require(script:WaitForChild("UiLibrary"))
	local App = require(script:WaitForChild("App"))

	local ui = UiLibrary.new(widget, plugin)
	App.init(ui, plugin)

	plugin.Unloading:Once(function()
		ui:Destroy()
	end)
end)

if not ok then
	warn("[re/up] plugin init failed: " .. tostring(err))
end
