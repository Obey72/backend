--!strict
local Tab = {}

local Button = require("./Tab/Button")
local Label = require("./Tab/Label")
local List = require("./Tab/List")
local Split = require("./Tab/Split")
local Toggle = require("./Tab/Toggle")
local Theme = require("./Theme")

local assets = script.Parent.Assets
local asset = assets.Tab
local mainUiFrame = assets.MainFrame

local function CreateButton(self: Tab, text: string, callback: () -> ()): Button.Button
	local button = Button.new(self.instance, text, callback)
	table.insert(self._elements, button)
	return button
end

local function CreateToggle(self: Tab, text: string, callback: (state: boolean) -> (), defaultState: boolean?): Toggle.Toggle
	local toggle = Toggle.new(self.instance, text, callback, defaultState)
	table.insert(self._elements, toggle)
	return toggle
end

local function CreateSplit(self: Tab): Split.Split
	local split = Split.new(self.instance)
	table.insert(self._elements, split)
	return split
end

local function CreateList(self: Tab): List.List
	local list = List.new(self.instance :: any)
	table.insert(self._elements, list)
	return list
end

local function CreateLabel(self: Tab, text: string): Label.Label
	local label = Label.new(self.instance :: any, text)
	table.insert(self._elements, label)
	return label
end

-- self-contained input widget does not depend on the pre-built assetsinput
-- because that asset is single-line and clears on submit; callers that need a
-- persistent multi-line input use this widget instead
export type Input = {
	instance: TextBox,
	UpdateTheme: (self: Input) -> (),
	Destroy: (self: Input) -> (),
}

local function CreateInput(self: Tab, placeholderText: string, multiLine: boolean?): Input
	local box = Instance.new("TextBox")
	box.Name = "Input"
	box.PlaceholderText = placeholderText
	box.Text = ""
	box.ClearTextOnFocus = false
	box.MultiLine = multiLine == true
	box.TextWrapped = true
	box.TextXAlignment = Enum.TextXAlignment.Left
	box.TextYAlignment = Enum.TextYAlignment.Top
	box.Size = if multiLine then UDim2.new(1, 0, 0, 88) else UDim2.new(1, 0, 0, 22)
	box.BorderSizePixel = 1

	local padding = Instance.new("UIPadding")
	padding.PaddingLeft = UDim.new(0, 6)
	padding.PaddingRight = UDim.new(0, 6)
	padding.PaddingTop = UDim.new(0, 4)
	padding.PaddingBottom = UDim.new(0, 4)
	padding.Parent = box

	local function applyTheme()
		local theme = Theme.get()
		if not theme then return end
		box.BackgroundColor3 = theme.UnselectedColor
		box.BorderColor3 = theme.BorderColor
		Theme.updateText(box, false)
	end

	local widget: Input = {
		instance = box,
		UpdateTheme = applyTheme,
		Destroy = function(s: Input)
			s.instance:Destroy()
			table.clear(s :: any)
		end,
	}

	applyTheme()
	box.Parent = self.instance
	table.insert(self._elements, widget)
	return widget
end

local function UpdateTheme(self: Tab)
	self.button:UpdateTheme()

	for _, element in self._elements do 
		element:UpdateTheme() 
	end
end

local function Destroy(self: Tab)
	for _, element in self._elements do 
		element:Destroy() 
	end

	table.clear(self :: any)
end

export type Tab = {
	_elements: { any },

	button: Button.Button,
	instance: typeof(asset),

	CreateButton: typeof(CreateButton),
	CreateToggle: typeof(CreateToggle),
	CreateSplit: typeof(CreateSplit),
	CreateList: typeof(CreateList),
	CreateLabel: typeof(CreateLabel),
	CreateInput: typeof(CreateInput),
	UpdateTheme: typeof(UpdateTheme),
	Destroy: typeof(Destroy),
}

function Tab.new(uiFrame: typeof(mainUiFrame), clickedCallback: () -> (), name: string): Tab
    local instance = asset:Clone()
	local self: Tab = {
        _elements = {},
        
		button = Button.new(uiFrame.Topbar.Frame, name, clickedCallback, false),
		instance = instance,

		CreateButton = CreateButton,
		CreateToggle = CreateToggle,
		CreateSplit = CreateSplit,
		CreateList = CreateList,
		CreateLabel = CreateLabel,
		CreateInput = CreateInput,
		UpdateTheme = UpdateTheme,
		Destroy = Destroy,
	}

	self:UpdateTheme()
	instance.Parent =  uiFrame.Main.Tabs

	return self
end

return Tab