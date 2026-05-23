--!strict
local Label = {}

local Theme = require("../Theme")

local assets = script.Parent.Parent.Assets
local buttonAsset = assets.Button
local asset = assets.List

local function ChangeText(self: Label, newText: string)
    self.instance.Text = newText
end

local function UpdateTheme(self: Label)
    Theme.updateText(self.instance, true)
end

local function Destroy(self: Label)
    self.instance:Destroy()

    table.clear(self :: any)
end

export type Label = {
    instance: typeof(buttonAsset),
    
    ChangeText: typeof(ChangeText),
    UpdateTheme: typeof(UpdateTheme),
    Destroy: typeof(Destroy),
}

function Label.new(parent: typeof(asset), text: string): Label
    local instance = buttonAsset:Clone()
    instance.Text = text

    local self: Label = {
        instance = instance,

        ChangeText = ChangeText,
        UpdateTheme = UpdateTheme,
        Destroy = Destroy,
    }

    self:UpdateTheme()
    instance.Parent = parent

    return self
end

return Label