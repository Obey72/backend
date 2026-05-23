--!strict
-- rebuilt uilibrary: builds the entire plugin ui from instancenew with no asset deps
-- preserves the public api used by dashboard/filter/replace/settings/themes so
-- nothing in app/* needed to change every tab is a scrollingframe so content never clips

local UiLibrary = {}

local Theme = require("./UiLibrary/Theme")

type ThemeManager = typeof(Theme)

-- ---------- shared theme helpers ----------

local function color(name: any)
	local t = Theme.get()
	return (t :: any)[name]
end

local function applyTextStyle(obj: any, dim: boolean?)
	local t = Theme.get()
	obj.FontFace = t.Font
	obj.TextSize = t.TextSize
	obj.TextColor3 = if dim then t.DimmedTextColor else t.TextColor
	obj.TextStrokeColor3 = t.StrokeColor
	obj.TextStrokeTransparency = t.StrokeTransparency
	if obj:IsA("TextBox") then
		obj.PlaceholderColor3 = t.DimmedTextColor
	end
end

local function applyFrame(obj: any)
	local t = Theme.get()
	obj.BackgroundColor3 = t.ForegroundColor
	obj.BorderColor3 = t.BorderColor
end

-- ---------- tip ----------
-- tips are just a chunk of help text ui:createtip(text) returns an object that can be
-- passed to input:addtip(tip) which renders the text as a dim label under the input

export type Tip = { text: string }

local function newTip(text: string): Tip
	return { text = text }
end

-- ---------- button ----------

export type Button = {
	instance: TextButton,
	_callback: (() -> ())?,
	ChangeText: (self: Button, text: string) -> (),
	SetCallback: (self: Button, cb: () -> ()) -> (),
	ColorSelected: (self: Button) -> (),
	ColorUnselected: (self: Button) -> (),
	SetActive: (self: Button, active: boolean) -> (),
	UpdateTheme: (self: Button) -> (),
	Destroy: (self: Button) -> (),
}

local function newButton(parent: Instance, text: string, callback: (() -> ())?, layoutOrder: number?): Button
	local btn = Instance.new("TextButton")
	btn.Name = "Button"
	btn.Size = UDim2.new(1, 0, 0, 26)
	btn.AutoButtonColor = true
	btn.BorderSizePixel = 1
	btn.Text = text
	btn.LayoutOrder = layoutOrder or 0

	local self = {} :: Button
	self.instance = btn
	self._callback = callback

	function self:ChangeText(t: string)
		self.instance.Text = t
	end
	function self:SetCallback(cb: () -> ())
		self._callback = cb
	end
	function self:ColorSelected()
		self.instance.BackgroundColor3 = color("SelectedColor")
	end
	function self:ColorUnselected()
		self.instance.BackgroundColor3 = color("UnselectedColor")
	end
	function self:SetActive(active: boolean)
		self.instance.BackgroundColor3 = if active then color("SelectedColor") else color("UnselectedColor")
	end
	function self:UpdateTheme()
		applyTextStyle(self.instance)
		self.instance.BackgroundColor3 = color("UnselectedColor")
		self.instance.BorderColor3 = color("BorderColor")
	end

	function self:Destroy()
		self.instance:Destroy()
		table.clear(self :: any)
	end

	btn.MouseButton1Click:Connect(function()
		if self._callback then self._callback() end
	end)

	self:UpdateTheme()
	btn.Parent = parent
	return self
end

-- ---------- label ----------

export type Label = {
	instance: TextLabel,
	ChangeText: (self: Label, text: string) -> (),
	UpdateTheme: (self: Label) -> (),
	Destroy: (self: Label) -> (),
}

local function newLabel(parent: Instance, text: string, layoutOrder: number?): Label
	local lbl = Instance.new("TextLabel")
	lbl.Name = "Label"
	lbl.AutomaticSize = Enum.AutomaticSize.Y
	lbl.Size = UDim2.new(1, 0, 0, 0)
	lbl.BackgroundTransparency = 1
	lbl.Text = text
	lbl.TextXAlignment = Enum.TextXAlignment.Left
	lbl.TextYAlignment = Enum.TextYAlignment.Top
	lbl.TextWrapped = true
	lbl.LayoutOrder = layoutOrder or 0

	local self = {} :: Label
	self.instance = lbl
	function self:ChangeText(t: string) self.instance.Text = t end
	function self:UpdateTheme() applyTextStyle(self.instance) end
	function self:Destroy() self.instance:Destroy(); table.clear(self :: any) end

	self:UpdateTheme()
	lbl.Parent = parent
	return self
end

-- ---------- simple input (from tab:createinput) ----------
-- instance is the textbox itself (matches the old tabcreateinput contract)

export type SimpleInput = {
	instance: TextBox,
	UpdateTheme: (self: SimpleInput) -> (),
	Destroy: (self: SimpleInput) -> (),
}

local function newSimpleInput(parent: Instance, placeholder: string, multiline: boolean?, layoutOrder: number?): SimpleInput
	local box = Instance.new("TextBox")
	box.Name = "Input"
	box.PlaceholderText = placeholder
	box.Text = ""
	box.ClearTextOnFocus = false
	box.MultiLine = multiline == true
	box.TextWrapped = true
	box.TextXAlignment = Enum.TextXAlignment.Left
	box.TextYAlignment = if multiline then Enum.TextYAlignment.Top else Enum.TextYAlignment.Center
	box.Size = if multiline then UDim2.new(1, 0, 0, 120) else UDim2.new(1, 0, 0, 24)
	box.BorderSizePixel = 1
	box.LayoutOrder = layoutOrder or 0

	local pad = Instance.new("UIPadding")
	pad.PaddingLeft = UDim.new(0, 6)
	pad.PaddingRight = UDim.new(0, 6)
	pad.PaddingTop = UDim.new(0, 4)
	pad.PaddingBottom = UDim.new(0, 4)
	pad.Parent = box

	local self = {} :: SimpleInput
	self.instance = box
	function self:UpdateTheme()
		applyTextStyle(self.instance)
		self.instance.BackgroundColor3 = color("UnselectedColor")
		self.instance.BorderColor3 = color("BorderColor")
	end
	function self:Destroy() self.instance:Destroy(); table.clear(self :: any) end

	self:UpdateTheme()
	box.Parent = parent
	return self
end

-- ---------- toggle ----------

export type Toggle = {
	instance: Frame,
	_state: boolean,
	_label: TextLabel,
	_box: TextButton,
	_callback: ((boolean) -> ())?,
	SetState: (self: Toggle, state: boolean) -> (),
	GetState: (self: Toggle) -> boolean,
	UpdateTheme: (self: Toggle) -> (),
	Destroy: (self: Toggle) -> (),
}

local function newToggle(parent: Instance, text: string, callback: ((boolean) -> ())?, default: boolean?, layoutOrder: number?): Toggle
	local row = Instance.new("Frame")
	row.Name = "Toggle"
	row.Size = UDim2.new(1, 0, 0, 22)
	row.BackgroundTransparency = 1
	row.LayoutOrder = layoutOrder or 0

	local check = Instance.new("TextButton")
	check.Name = "Check"
	check.Size = UDim2.fromOffset(18, 18)
	check.Position = UDim2.new(0, 0, 0.5, 0)
	check.AnchorPoint = Vector2.new(0, 0.5)
	check.AutoButtonColor = true
	check.Text = ""
	check.BorderSizePixel = 1
	check.Parent = row

	local lbl = Instance.new("TextLabel")
	lbl.Name = "Label"
	lbl.Size = UDim2.new(1, -26, 1, 0)
	lbl.Position = UDim2.fromOffset(24, 0)
	lbl.BackgroundTransparency = 1
	lbl.Text = text
	lbl.TextXAlignment = Enum.TextXAlignment.Left
	lbl.TextYAlignment = Enum.TextYAlignment.Center
	lbl.Parent = row

	local self = {} :: Toggle
	self.instance = row
	self._state = default == true
	self._label = lbl
	self._box = check
	self._callback = callback

	function self:SetState(state: boolean)
		self._state = state == true
		self._box.BackgroundColor3 = if self._state then color("ToggleBackgroundColor") else color("UnselectedColor")
		self._box.Text = if self._state then "✓" else ""
	end
	function self:GetState(): boolean return self._state end
	function self:UpdateTheme()
		applyTextStyle(self._label)
		applyTextStyle(self._box)
		self._box.BorderColor3 = color("BorderColor")
		self:SetState(self._state)
	end
	function self:Destroy() self.instance:Destroy(); table.clear(self :: any) end

	check.MouseButton1Click:Connect(function()
		self:SetState(not self._state)
		if self._callback then self._callback(self._state) end
	end)

	self:UpdateTheme()
	row.Parent = parent
	return self
end

-- forward decl: splitcreateinput needs newlistinput defined further down
local newListInput: any

-- ---------- split (horizontal container that hosts toggles side-by-side) ----------

export type Split = {
	instance: Frame,
	_children: { any },
	CreateToggle: (self: Split, text: string, callback: ((boolean) -> ())?, default: boolean?) -> Toggle,
	CreateInput: (self: Split, placeholder: string, callback: (input: string) -> ()) -> ListInput,
	UpdateTheme: (self: Split) -> (),
	Destroy: (self: Split) -> (),
}

local function newSplit(parent: Instance, layoutOrder: number?): Split
	local frame = Instance.new("Frame")
	frame.Name = "Split"
	frame.Size = UDim2.new(1, 0, 0, 22)
	frame.BackgroundTransparency = 1
	frame.LayoutOrder = layoutOrder or 0

	local h = Instance.new("UIListLayout")
	h.FillDirection = Enum.FillDirection.Horizontal
	h.SortOrder = Enum.SortOrder.LayoutOrder
	h.Padding = UDim.new(0, 8)
	h.Parent = frame

	local self = {} :: Split
	self.instance = frame
	self._children = {}

	function self:CreateToggle(text: string, callback: ((boolean) -> ())?, default: boolean?): Toggle
		-- toggles inside a split share width so override their size
		local t = newToggle(frame, text, callback, default, #self._children + 1)
		t.instance.Size = UDim2.new(0.5, -4, 1, 0)
		table.insert(self._children, t)
		return t
	end
	function self:CreateInput(placeholder: string, callback: (input: string) -> ()): ListInput
		local input = newListInput(frame, placeholder, callback)
		input.instance.LayoutOrder = #self._children + 1
		input.instance.Size = UDim2.new(0.5, -4, 1, 0)
		table.insert(self._children, input)
		return input
	end
	function self:UpdateTheme()
		for _, c in self._children do c:UpdateTheme() end
	end
	function self:Destroy()
		for _, c in self._children do c:Destroy() end
		self.instance:Destroy()
		table.clear(self :: any)
	end

	frame.Parent = parent
	return self
end

-- ---------- list + listelement + listinput ----------
-- list:addinput(placeholder callback) -> listinput
--   instancetextbox (frame with textbox child used by filter)
--   :addtip(tip)
-- list:createelement(value) -> listelement
--   instancetextlabel (frame with textlabel child)
--   :adddestroybutton() (chainable adds an x button)
-- list:get() -> { string }
-- listelements: { [string]: listelement }

export type ListElement = {
	instance: Frame,
	_value: string,
	_onDestroy: () -> (),
	AddDestroyButton: (self: ListElement) -> ListElement,
	UpdateTheme: (self: ListElement) -> (),
	Destroy: (self: ListElement) -> (),
}

local function newListElement(parent: Instance, value: string, onDestroy: () -> ()): ListElement
	local row = Instance.new("Frame")
	row.Name = "ListElement"
	row.Size = UDim2.new(1, 0, 0, 22)
	row.BorderSizePixel = 1

	local txt = Instance.new("TextLabel")
	txt.Name = "TextLabel"
	txt.Size = UDim2.new(1, -28, 1, 0)
	txt.BackgroundTransparency = 1
	txt.Text = value
	txt.TextXAlignment = Enum.TextXAlignment.Left
	txt.TextYAlignment = Enum.TextYAlignment.Center
	txt.Parent = row

	local txtPad = Instance.new("UIPadding")
	txtPad.PaddingLeft = UDim.new(0, 6)
	txtPad.PaddingRight = UDim.new(0, 6)
	txtPad.Parent = txt

	local self = {} :: ListElement
	self.instance = row
	self._value = value
	self._onDestroy = onDestroy

	function self:AddDestroyButton(): ListElement
		local x = Instance.new("TextButton")
		x.Name = "Destroy"
		x.Size = UDim2.fromOffset(22, 22)
		x.AnchorPoint = Vector2.new(1, 0)
		x.Position = UDim2.new(1, 0, 0, 0)
		x.Text = "x"
		x.BorderSizePixel = 1
		applyTextStyle(x)
		x.BackgroundColor3 = color("UnselectedColor")
		x.BorderColor3 = color("BorderColor")
		x.MouseButton1Click:Connect(function() self:Destroy() end)
		x.Parent = self.instance
		return self
	end
	function self:UpdateTheme()
		applyFrame(self.instance)
		applyTextStyle(txt)
	end
	function self:Destroy()
		self._onDestroy()
		self.instance:Destroy()
		table.clear(self :: any)
	end

	self:UpdateTheme()
	row.Parent = parent
	return self
end

export type ListInput = {
	instance: Frame,
	_box: TextBox,
	_tipLabel: TextLabel?,
	AddTip: (self: ListInput, tip: Tip) -> (),
	UpdateTheme: (self: ListInput) -> (),
	Destroy: (self: ListInput) -> (),
}

function newListInput(parent: Instance, placeholder: string, callback: (input: string) -> ()): ListInput
	local row = Instance.new("Frame")
	row.Name = "ListInput"
	row.Size = UDim2.new(1, 0, 0, 22)
	row.BackgroundTransparency = 1
	row.LayoutOrder = 0

	local box = Instance.new("TextBox")
	box.Name = "TextBox"
	box.Size = UDim2.new(1, 0, 0, 22)
	box.BorderSizePixel = 1
	box.Text = ""
	box.PlaceholderText = placeholder
	box.ClearTextOnFocus = true
	box.TextXAlignment = Enum.TextXAlignment.Left
	box.TextYAlignment = Enum.TextYAlignment.Center
	box.Parent = row

	local pad = Instance.new("UIPadding")
	pad.PaddingLeft = UDim.new(0, 6)
	pad.PaddingRight = UDim.new(0, 6)
	pad.Parent = box

	local self = {} :: ListInput
	self.instance = row
	self._box = box
	self._tipLabel = nil

	function self:AddTip(tip: Tip)
		if self._tipLabel then self._tipLabel:Destroy() end
		local t = Instance.new("TextLabel")
		t.Name = "Tip"
		t.AutomaticSize = Enum.AutomaticSize.Y
		t.Size = UDim2.new(1, 0, 0, 0)
		t.Position = UDim2.new(0, 0, 1, 4)
		t.BackgroundTransparency = 1
		t.Text = tip.text
		t.TextXAlignment = Enum.TextXAlignment.Left
		t.TextYAlignment = Enum.TextYAlignment.Top
		t.TextWrapped = true
		applyTextStyle(t, true)
		t.Parent = self.instance
		self._tipLabel = t
		-- grow row so the tip is visible below the input
		self.instance.AutomaticSize = Enum.AutomaticSize.Y
	end
	function self:UpdateTheme()
		applyTextStyle(box)
		box.BackgroundColor3 = color("UnselectedColor")
		box.BorderColor3 = color("BorderColor")
		if self._tipLabel then applyTextStyle(self._tipLabel, true) end
	end
	function self:Destroy() self.instance:Destroy(); table.clear(self :: any) end

	box.FocusLost:Connect(function(enterPressed)
		if enterPressed and box.Text ~= "" then
			local v = box.Text
			box.Text = ""
			callback(v)
		end
	end)

	self:UpdateTheme()
	row.Parent = parent
	return self
end

export type List = {
	instance: Frame,
	_input: ListInput?,
	_elementsFrame: ScrollingFrame,
	elements: { [string]: ListElement },
	AddInput: (self: List, placeholder: string, callback: (input: string) -> ()) -> ListInput,
	CreateElement: (self: List, value: string) -> ListElement,
	RemoveElement: (self: List, value: string) -> (),
	Get: (self: List) -> { string },
	UpdateTheme: (self: List) -> (),
	Destroy: (self: List) -> (),
}

local function newList(parent: Instance, layoutOrder: number?): List
	local frame = Instance.new("Frame")
	frame.Name = "List"
	frame.AutomaticSize = Enum.AutomaticSize.Y
	frame.Size = UDim2.new(1, 0, 0, 0)
	frame.BackgroundTransparency = 1
	frame.LayoutOrder = layoutOrder or 0

	local vert = Instance.new("UIListLayout")
	vert.FillDirection = Enum.FillDirection.Vertical
	vert.SortOrder = Enum.SortOrder.LayoutOrder
	vert.Padding = UDim.new(0, 4)
	vert.Parent = frame

	local elementsFrame = Instance.new("ScrollingFrame")
	elementsFrame.Name = "Elements"
	elementsFrame.Size = UDim2.new(1, 0, 0, 160)
	elementsFrame.BackgroundTransparency = 0
	elementsFrame.BorderSizePixel = 1
	elementsFrame.ScrollBarThickness = 6
	elementsFrame.CanvasSize = UDim2.new()
	elementsFrame.AutomaticCanvasSize = Enum.AutomaticSize.Y
	elementsFrame.LayoutOrder = 2
	elementsFrame.Parent = frame

	local elementsLayout = Instance.new("UIListLayout")
	elementsLayout.SortOrder = Enum.SortOrder.LayoutOrder
	elementsLayout.Padding = UDim.new(0, 2)
	elementsLayout.Parent = elementsFrame

	local self = {} :: List
	self.instance = frame
	self._input = nil
	self._elementsFrame = elementsFrame
	self.elements = {}

	function self:AddInput(placeholder: string, callback: (input: string) -> ()): ListInput
		local input = newListInput(frame, placeholder, callback)
		input.instance.LayoutOrder = 1
		self._input = input
		return input
	end
	function self:CreateElement(value: string): ListElement
		local el = newListElement(elementsFrame, value, function() self.elements[value] = nil end)
		self.elements[value] = el
		return el
	end
	function self:RemoveElement(value: string)
		if not self.elements[value] then error(`{value} does not exist in list`) end
		self.elements[value]:Destroy()
		self.elements[value] = nil
	end
	function self:Get(): { string }
		local out = {}
		for k in self.elements do table.insert(out, k) end
		return out
	end
	function self:UpdateTheme()
		applyFrame(elementsFrame)
		if self._input then self._input:UpdateTheme() end
		for _, el in self.elements do el:UpdateTheme() end
	end
	function self:Destroy()
		for _, el in self.elements do el:Destroy() end
		self.instance:Destroy()
		table.clear(self :: any)
	end

	self:UpdateTheme()
	frame.Parent = parent
	return self
end

-- ---------- dropdown ----------
-- collapsing dropdown: header button shows current selection click expands a
-- vertical list of options selecting an option collapses the list and fires the callback

export type Dropdown = {
	instance: Frame,
	_header: TextButton,
	_optionsFrame: Frame,
	_current: string,
	_callback: ((string) -> ())?,
	GetSelection: (self: Dropdown) -> string,
	SetSelection: (self: Dropdown, opt: string) -> (),
	UpdateTheme: (self: Dropdown) -> (),
	Destroy: (self: Dropdown) -> (),
}

local function newDropdown(parent: Instance, options: { string }, callback: ((string) -> ())?, layoutOrder: number?): Dropdown
	local container = Instance.new("Frame")
	container.Name = "Dropdown"
	container.Size = UDim2.new(1, 0, 0, 28)
	container.AutomaticSize = Enum.AutomaticSize.Y
	container.BackgroundTransparency = 1
	container.LayoutOrder = layoutOrder or 0

	local stack = Instance.new("UIListLayout")
	stack.SortOrder = Enum.SortOrder.LayoutOrder
	stack.Padding = UDim.new(0, 1)
	stack.Parent = container

	local header = Instance.new("TextButton")
	header.Name = "Header"
	header.Size = UDim2.new(1, 0, 0, 26)
	header.BorderSizePixel = 1
	header.LayoutOrder = 1
	header.AutoButtonColor = true
	header.Parent = container

	local optionsFrame = Instance.new("Frame")
	optionsFrame.Name = "Options"
	optionsFrame.Size = UDim2.new(1, 0, 0, 0)
	optionsFrame.AutomaticSize = Enum.AutomaticSize.Y
	optionsFrame.BackgroundTransparency = 1
	optionsFrame.Visible = false
	optionsFrame.LayoutOrder = 2
	optionsFrame.Parent = container

	local optLayout = Instance.new("UIListLayout")
	optLayout.SortOrder = Enum.SortOrder.LayoutOrder
	optLayout.Padding = UDim.new(0, 1)
	optLayout.Parent = optionsFrame

	local self = {} :: Dropdown
	self.instance = container
	self._header = header
	self._optionsFrame = optionsFrame
	self._current = options[1] or ""
	self._callback = callback

	function self:GetSelection() return self._current end
	function self:SetSelection(opt: string)
		self._current = opt
		self._header.Text = opt .. "  v"
		self._optionsFrame.Visible = false
	end
	function self:UpdateTheme()
		applyTextStyle(self._header)
		self._header.BackgroundColor3 = color("UnselectedColor")
		self._header.BorderColor3 = color("BorderColor")
		for _, c in self._optionsFrame:GetChildren() do
			if c:IsA("TextButton") then
				applyTextStyle(c)
				c.BackgroundColor3 = color("UnselectedColor")
				c.BorderColor3 = color("BorderColor")
			end
		end
	end
	function self:Destroy() self.instance:Destroy(); table.clear(self :: any) end

	for i, opt in options do
		local optBtn = Instance.new("TextButton")
		optBtn.Name = "Opt_" .. opt
		optBtn.Size = UDim2.new(1, 0, 0, 24)
		optBtn.BorderSizePixel = 1
		optBtn.LayoutOrder = i
		optBtn.Text = opt
		optBtn.AutoButtonColor = true
		optBtn.Parent = optionsFrame
		optBtn.MouseButton1Click:Connect(function()
			self:SetSelection(opt)
			if self._callback then self._callback(opt) end
		end)
	end

	header.MouseButton1Click:Connect(function()
		optionsFrame.Visible = not optionsFrame.Visible
	end)

	self:SetSelection(self._current)
	self:UpdateTheme()
	container.Parent = parent
	return self
end

-- ---------- tab ----------

export type Tab = {
	_elements: { any },
	_orderCounter: number,
	button: Button,
	instance: ScrollingFrame,
	CreateButton: (self: Tab, text: string, callback: () -> ()) -> Button,
	CreateLabel: (self: Tab, text: string) -> Label,
	CreateInput: (self: Tab, placeholder: string, multiline: boolean?) -> SimpleInput,
	CreateToggle: (self: Tab, text: string, callback: ((boolean) -> ())?, default: boolean?) -> Toggle,
	CreateSplit: (self: Tab) -> Split,
	CreateList: (self: Tab) -> List,
	CreateDropdown: (self: Tab, options: { string }, callback: ((string) -> ())?) -> Dropdown,
	UpdateTheme: (self: Tab) -> (),
	Destroy: (self: Tab) -> (),
}

local function newTab(topbar: Instance, contentRoot: Instance, name: string, onSelect: () -> ()): Tab
	-- topbar button (sized by main ui's tab grid)
	local btn = newButton(topbar, name, onSelect)
	btn.instance.Size = UDim2.new(0, 0, 1, 0)

	-- content scrollingframe
	local content = Instance.new("ScrollingFrame")
	content.Name = "Tab_" .. name
	content.Size = UDim2.fromScale(1, 1)
	content.BackgroundTransparency = 1
	content.BorderSizePixel = 0
	content.ScrollBarThickness = 6
	content.ScrollingDirection = Enum.ScrollingDirection.Y
	content.CanvasSize = UDim2.new()
	content.AutomaticCanvasSize = Enum.AutomaticSize.Y
	content.Visible = false
	content.Parent = contentRoot

	local layout = Instance.new("UIListLayout")
	layout.SortOrder = Enum.SortOrder.LayoutOrder
	layout.Padding = UDim.new(0, 6)
	layout.Parent = content

	local pad = Instance.new("UIPadding")
	pad.PaddingLeft = UDim.new(0, 8)
	pad.PaddingRight = UDim.new(0, 12)
	pad.PaddingTop = UDim.new(0, 8)
	pad.PaddingBottom = UDim.new(0, 8)
	pad.Parent = content

	local self = {} :: Tab
	self._elements = {}
	self._orderCounter = 0
	self.button = btn
	self.instance = content

	local function nextOrder(): number
		self._orderCounter += 1
		return self._orderCounter
	end

	function self:CreateButton(text, callback)
		local w = newButton(content, text, callback, nextOrder())
		table.insert(self._elements, w)
		return w
	end
	function self:CreateLabel(text)
		local w = newLabel(content, text, nextOrder())
		table.insert(self._elements, w)
		return w
	end
	function self:CreateInput(placeholder, multiline)
		local w = newSimpleInput(content, placeholder, multiline, nextOrder())
		table.insert(self._elements, w)
		return w
	end
	function self:CreateToggle(text, callback, default)
		local w = newToggle(content, text, callback, default, nextOrder())
		table.insert(self._elements, w)
		return w
	end
	function self:CreateSplit()
		local w = newSplit(content, nextOrder())
		table.insert(self._elements, w)
		return w
	end
	function self:CreateList()
		local w = newList(content, nextOrder())
		table.insert(self._elements, w)
		return w
	end
	function self:CreateDropdown(options, callback)
		local w = newDropdown(content, options, callback, nextOrder())
		table.insert(self._elements, w)
		return w
	end
	function self:UpdateTheme()
		self.button:UpdateTheme()
		for _, e in self._elements do
			if (e :: any).UpdateTheme then (e :: any):UpdateTheme() end
		end
	end
	function self:Destroy()
		for _, e in self._elements do
			if (e :: any).Destroy then (e :: any):Destroy() end
		end
		self.instance:Destroy()
		self.button:Destroy()
		table.clear(self :: any)
	end

	return self
end

-- ---------- main ui ----------

export type Ui = {
	_currentTabIndex: number,
	_studioThemeChanged: RBXScriptConnection,
	_tabs: { Tab },
	_topbar: Frame,
	_topbarRow: Frame,
	_contentRoot: Frame,
	_notification: Frame,
	theme: ThemeManager,
	instance: Frame,
	CreateTab: (self: Ui, name: string) -> Tab,
	CreateTip: (self: Ui, text: string) -> Tip,
	Notify: (self: Ui, title: string, description: string) -> (),
	UpdateTheme: (self: Ui) -> (),
	Destroy: (self: Ui) -> (),
}

local function buildNotification(parent: Instance): Frame
	local overlay = Instance.new("Frame")
	overlay.Name = "Notification"
	overlay.AnchorPoint = Vector2.new(0.5, 1)
	overlay.Position = UDim2.new(0.5, 0, 1, -8)
	overlay.Size = UDim2.new(1, -16, 0, 0)
	overlay.AutomaticSize = Enum.AutomaticSize.Y
	overlay.BorderSizePixel = 1
	overlay.Visible = false
	overlay.ZIndex = 10

	local pad = Instance.new("UIPadding")
	pad.PaddingLeft = UDim.new(0, 8)
	pad.PaddingRight = UDim.new(0, 8)
	pad.PaddingTop = UDim.new(0, 6)
	pad.PaddingBottom = UDim.new(0, 6)
	pad.Parent = overlay

	local layout = Instance.new("UIListLayout")
	layout.SortOrder = Enum.SortOrder.LayoutOrder
	layout.Padding = UDim.new(0, 2)
	layout.Parent = overlay

	local title = Instance.new("TextLabel")
	title.Name = "Title"
	title.AutomaticSize = Enum.AutomaticSize.Y
	title.Size = UDim2.new(1, -24, 0, 0)
	title.BackgroundTransparency = 1
	title.TextXAlignment = Enum.TextXAlignment.Left
	title.TextWrapped = true
	title.LayoutOrder = 1
	title.Parent = overlay

	local desc = Instance.new("TextLabel")
	desc.Name = "Description"
	desc.AutomaticSize = Enum.AutomaticSize.Y
	desc.Size = UDim2.new(1, 0, 0, 0)
	desc.BackgroundTransparency = 1
	desc.TextXAlignment = Enum.TextXAlignment.Left
	desc.TextWrapped = true
	desc.LayoutOrder = 2
	desc.Parent = overlay

	local close = Instance.new("TextButton")
	close.Name = "Close"
	close.Size = UDim2.fromOffset(20, 20)
	close.AnchorPoint = Vector2.new(1, 0)
	close.Position = UDim2.new(1, -4, 0, 4)
	close.Text = "x"
	close.BorderSizePixel = 1
	close.ZIndex = 11
	close.Parent = overlay
	close.MouseButton1Click:Connect(function() overlay.Visible = false end)

	overlay.Parent = parent
	return overlay
end

function UiLibrary.new(parent: Instance, plugin: Plugin): Ui
	Theme.init(plugin)

	local root = Instance.new("Frame")
	root.Name = "ReupRoot"
	root.Size = UDim2.fromScale(1, 1)
	root.BorderSizePixel = 0

	local topbar = Instance.new("Frame")
	topbar.Name = "Topbar"
	topbar.Size = UDim2.new(1, 0, 0, 28)
	topbar.BorderSizePixel = 0
	topbar.Parent = root

	local topbarRow = Instance.new("Frame")
	topbarRow.Name = "Tabs"
	topbarRow.Size = UDim2.new(1, -8, 1, -4)
	topbarRow.Position = UDim2.fromOffset(4, 2)
	topbarRow.BackgroundTransparency = 1
	topbarRow.Parent = topbar

	local topbarLayout = Instance.new("UIListLayout")
	topbarLayout.FillDirection = Enum.FillDirection.Horizontal
	topbarLayout.SortOrder = Enum.SortOrder.LayoutOrder
	topbarLayout.Padding = UDim.new(0, 4)
	topbarLayout.Parent = topbarRow

	local content = Instance.new("Frame")
	content.Name = "Content"
	content.Position = UDim2.fromOffset(0, 28)
	content.Size = UDim2.new(1, 0, 1, -28)
	content.BorderSizePixel = 0
	content.Parent = root

	local notification = buildNotification(root)

	local self: Ui; self = {
		_currentTabIndex = 0,
		_studioThemeChanged = settings().Studio.ThemeChanged:Connect(function()
			Theme.set(Theme.get().Id);
			(self :: any):UpdateTheme()
		end),
		_tabs = {},
		_topbar = topbar,
		_topbarRow = topbarRow,
		_contentRoot = content,
		_notification = notification,

		theme = Theme,
		instance = root,

		CreateTab = nil :: any,
		CreateTip = nil :: any,
		Notify = nil :: any,
		UpdateTheme = nil :: any,
		Destroy = nil :: any,
	}

	local function selectTab(idx: number)
		if self._currentTabIndex == idx then return end
		if self._currentTabIndex ~= 0 then
			local prev = self._tabs[self._currentTabIndex]
			prev.button:ColorUnselected()
			prev.instance.Visible = false
		end
		local sel = self._tabs[idx]
		sel.button:ColorSelected()
		sel.instance.Visible = true
		self._currentTabIndex = idx
	end

	function self:CreateTab(name: string): Tab
		local idx = #self._tabs + 1
		local tab; tab = newTab(self._topbarRow, self._contentRoot, name, function()
			selectTab(idx)
		end)
		-- equal-width tab buttons
		local count = #self._tabs + 1
		tab.button.instance.Size = UDim2.new(1 / count, -2, 1, 0)
		table.insert(self._tabs, tab)
		-- resize all tab buttons so they share width
		for _, t in self._tabs do
			t.button.instance.Size = UDim2.new(1 / count, -2, 1, 0)
		end
		if self._currentTabIndex == 0 then selectTab(idx) end
		return tab
	end

	function self:CreateTip(text: string): Tip
		return newTip(text)
	end

	function self:Notify(title: string, description: string)
		local titleLbl = notification:FindFirstChild("Title") :: TextLabel
		local descLbl = notification:FindFirstChild("Description") :: TextLabel
		titleLbl.Text = title
		descLbl.Text = description
		notification.Visible = true
	end

	function self:UpdateTheme()
		local t = Theme.get()
		root.BackgroundColor3 = t.BackgroundColor
		topbar.BackgroundColor3 = t.ForegroundColor
		content.BackgroundColor3 = t.ForegroundColor

		notification.BackgroundColor3 = t.ForegroundColor
		notification.BorderColor3 = t.BorderColor
		local nTitle = notification:FindFirstChild("Title") :: TextLabel?
		local nDesc = notification:FindFirstChild("Description") :: TextLabel?
		local nClose = notification:FindFirstChild("Close") :: TextButton?
		if nTitle then applyTextStyle(nTitle) end
		if nDesc then applyTextStyle(nDesc, true) end
		if nClose then
			applyTextStyle(nClose)
			nClose.BackgroundColor3 = t.UnselectedColor
			nClose.BorderColor3 = t.BorderColor
		end

		for _, tab in self._tabs do
			tab:UpdateTheme()
		end
		if self._currentTabIndex ~= 0 then
			self._tabs[self._currentTabIndex].button:ColorSelected()
		end
	end

	function self:Destroy()
		for _, tab in self._tabs do tab:Destroy() end
		self._studioThemeChanged:Disconnect()
		root:Destroy()
		table.clear(self :: any)
	end

	self:UpdateTheme()
	root.Parent = parent
	return self
end

return UiLibrary
