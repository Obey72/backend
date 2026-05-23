--!strict
local List = {}

local Input = require("./Input")
local Theme = require(".././Theme")
local ListElement = require("./List/ListElement")

local assets = script.Parent.Parent.Assets
local tabAsset = assets.Tab
local asset = assets.List

local function Get(self: List): { string }
    local values = {}
    for i, _ in self.elements do
        table.insert(values, i)
    end
    return values
end

local function AddInput(self: List, placeholderText: string, callback: (input: string) -> ()): Input.Input
    local input = Input.new(self.instance :: any, placeholderText, callback)

    local scrollingFrame = self.instance.ScrollingFrame
    scrollingFrame.Position = UDim2.new(0.5, 0, 0, 23)
    scrollingFrame.Size = UDim2.new(1, 0, 1, -23)

    self._input = input
    return input
end

local function CreateElement(self: List, value: string): ListElement.ListElement
    local listElement = ListElement.new(
        self.instance.ScrollingFrame :: any, 
        function() self.elements[value] = nil end,
        value
    )

    self.elements[value] = listElement
    return listElement
end

local function RemoveElement(self: List, value: string)
    if not self.elements[value] then error(`{value} does not exist in list`) end
    self.elements[value]:Destroy()
    self.elements[value] = nil
end

local function UpdateTheme(self: List)
    local listFrame = self.instance.ScrollingFrame
    Theme.updateFrame(listFrame)

    for _, element in self.elements do
        element:UpdateTheme()
    end

    if self._input then
        self._input:UpdateTheme()
    end
end

local function Destroy(self: List)
    for _, element in self.elements do
        element:Destroy()
    end

    self.instance:Destroy()

    table.clear(self :: any)
end

export type List = {
    _input: Input.Input?,

    elements: { [string]: ListElement.ListElement },
    instance: typeof(asset),

    Get: typeof(Get),
    AddInput: typeof(AddInput),
    CreateElement: typeof(CreateElement),
    RemoveElement: typeof(RemoveElement),
    UpdateTheme: typeof(UpdateTheme),
    Destroy: typeof(Destroy),
}

function List.new(parent: typeof(tabAsset)): List
    local instance = asset:Clone()
    instance.Size = UDim2.new(1, 0, 1, -parent.UIListLayout.AbsoluteContentSize.Y - 7)

    local self = {
        elements = {},
        instance = instance,

        Get = Get,
        AddInput = AddInput,
        CreateElement = CreateElement,
        RemoveElement = RemoveElement,
        UpdateTheme = UpdateTheme,
        Destroy = Destroy,
    } :: List

    self:UpdateTheme()
    instance.Parent = parent

    return self
end

return List