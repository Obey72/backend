--!strict
local AssetIdFilter = require("../AssetIdFilter")
local getFilterOptions = require("../GetFilterOptions")
local UiLibrary = require("../../UiLibrary")
local ApiDump = require("../ApiDump")

local HttpService = game:GetService("HttpService")
local StudioService = game:GetService("StudioService")

return function(ui: UiLibrary.Ui, plugin: Plugin)
    local tab = ui:CreateTab("Replace")

    local replaceButton = tab:CreateButton("Replace ids", function()
        local file: File? = StudioService:PromptImportFile({"json"} :: any) :: File
        if not file then return end

        local fileContents = file:GetBinaryContents()

        local success, idsToReplace = pcall(HttpService.JSONDecode, HttpService, fileContents)
        if not success then
            ui:Notify("Notification", "Can't parse JSON")
            return
        end

        for i, idInfo in idsToReplace :: any do
            if type(idInfo) ~= "table" then
                ui:Notify("Notification", `Invalid index: {i}`)
                return
            end

            local oldId = idInfo["oldId"]
            local newId = idInfo["newId"]

            local isOldIdValid = oldId and type(oldId) == "number"
            if not isOldIdValid then
                ui:Notify("Notification", `Invalid type at {i}[oldId]`)
                return
            end

            local isNewIdValid = newId and type(newId) == "number"
            if not isNewIdValid then
                ui:Notify("Notification", `Invalid type at {i}[newId]`)
                return
            end
        end

        local filterOptions = getFilterOptions(plugin, game:GetDescendants())
        local instancesArray = filterOptions.WhitelistedInstances

        local isApiDumpCached = ApiDump.isCached()
        if not isApiDumpCached then
            warn("Api dump not cached, skipping meshes")
        end

        local classNames = isApiDumpCached and {"Animation", "Sound", "MeshPart", "CharacterMesh", "SpecialMesh"} or {"Animation", "Sound"}
        for _, className in classNames do
            table.insert(instancesArray, className)
        end
        
        local filteredIds = AssetIdFilter.filterInstances(filterOptions)
        AssetIdFilter.replaceIds(filteredIds, idsToReplace :: any)

        warn("Finished replacing Ids")
    end)
    replaceButton.instance.LayoutOrder = 2
end