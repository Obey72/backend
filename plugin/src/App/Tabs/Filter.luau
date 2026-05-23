--!strict
local UiLibrary = require("../../UiLibrary")

local HttpService = game:GetService("HttpService")
local MarketplaceService = game:GetService("MarketplaceService")
local Players = game:GetService("Players")

return function(ui: UiLibrary.Ui, plugin: Plugin)
	local tab = ui:CreateTab("Filter")
        local placeList

    _G.getPlaceList = function(): { number } -- guys im so sorry im just so drained :sob: will be back at full on v200 forgive me for _g use i wont forgive myself
        local placesListValues = placeList:Get()
        local list = table.create(#placesListValues)
        for _, value in placesListValues do
            table.insert(list, tonumber(value))
        end
        return list :: { number }
    end

    -- send placeid + username + placename to the app so it can show the friendly name
    -- in the place list (mirrors what the plugin's filter list shows) placename is optional;
    -- the initial-load broadcast fires before marketplaceservice lookups so it sends ""
    local function sendPluginData(placeId: string, placeName: string?)
        if placeId == "" or placeId == "0" then return end
        local port = plugin:GetSetting("Port") or 38073
        local username = ""
        local localPlayer = Players.LocalPlayer
        if localPlayer then
            username = localPlayer.Name
        end
        local body = HttpService:JSONEncode({
            placeId = placeId,
            username = username,
            placeName = placeName or "",
        })
        pcall(HttpService.PostAsync, HttpService, `http://localhost:{port}/plugin-data`, body)
    end

    -- send current place on load (after a short delay to let the app start)
    task.delay(3, function()
        local pid = tostring(game.PlaceId)
        if pid ~= "0" then
            -- resolve our own placename too so the app's initial entry has a friendly name
            local ok, info = pcall(MarketplaceService.GetProductInfo, MarketplaceService, game.PlaceId)
            local placeName = if ok then (info :: any).Name else nil
            sendPluginData(pid, placeName)
        end
        -- also send saved place list entries
        for _, v in _G.getPlaceList() do
            sendPluginData(tostring(v))
        end
    end)

    -- name lookup worker: poll the backend for placeids the desktop app wants resolved
    -- (it can't resolve private places via the public roblox web api) resolve each via
    -- marketplaceservice (which uses the studio user's session) and post the name back
    -- through sendplugindata so the existing /plugin-data + sse path delivers it to the app
    local lookupResolved: { [string]: true } = {}
    task.spawn(function()
        while true do
            task.wait(1.5)
            local port = plugin:GetSetting("Port") or 38073
            local ok, body = pcall(HttpService.GetAsync, HttpService, `http://localhost:{port}/pending-place-lookups`)
            if not ok then continue end
            local parseOk, parsed = pcall(HttpService.JSONDecode, HttpService, body)
            if not parseOk or typeof(parsed) ~= "table" then continue end
            for _, idStr in parsed :: { string } do
                if lookupResolved[idStr] then continue end
                local numId = tonumber(idStr)
                if not numId then continue end
                local infoOk, info = pcall(MarketplaceService.GetProductInfo, MarketplaceService, numId)
                if infoOk then
                    local placeName = (info :: any).Name
                    lookupResolved[idStr] = true
                    sendPluginData(idStr, placeName)
                end
            end
        end
    end)
	
	local split1 = tab:CreateSplit()
    split1:CreateToggle("Search string values", function(state)
        plugin:SetSetting("StrVals", state)
    end, plugin:GetSetting("StrVals"))
    split1:CreateToggle("Search scripts", function(state)
        plugin:SetSetting("Scripts", state)
    end, plugin:GetSetting("Scripts"))
    
    local split2 = tab:CreateSplit()
    split2:CreateToggle("Search int/number values", function(state)
        plugin:SetSetting("NumVals", state)
    end, plugin:GetSetting("NumVals"))

    placeList = tab:CreateList()

	local placeInput = placeList:AddInput("Place Id", function(input)
		local placeId = tonumber(input)
        local success, productInfo = pcall(MarketplaceService.GetProductInfo, MarketplaceService, placeId)

        if not success then
            ui:Notify("Notification", "Error getting product info.") 
            return 
        end

        local assetTypeId = (productInfo :: any).AssetTypeId
		if assetTypeId ~= 9 then
            ui:Notify("Notification", "Invalid place id.") 
            return 
        end

        local stringPlaceId = tostring(placeId)
		if placeList.elements[stringPlaceId] then
            ui:Notify("Notification", "Place id is in list.") 
            return 
        end

		local element = placeList:CreateElement(stringPlaceId)
            :AddDestroyButton()

        local placeName = (productInfo :: any).Name
        element.instance.TextLabel.Text = `({placeId}) {placeName}`
        -- notify the app about this new place id include the resolved name so the
        -- app can show it next to the id instead of "" or unknown
        sendPluginData(stringPlaceId, placeName)
	end)
	placeInput:AddTip(ui:CreateTip(
		"Reuploading needs a place id. Public games under the creator are found automatically."
	))

    local textBox = placeInput.instance.TextBox
    textBox:GetPropertyChangedSignal("Text"):Connect(function() 
        textBox.Text = string.gsub(textBox.Text, "%D", "") 
    end)
end