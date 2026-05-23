--!strict
local AssetIdFilter = require("./AssetIdFilter")
local Connection = require("./Connection")
local retry = require("./Retry")
local StatusCodes = require("./StatusCodes")
local UiLibrary = require("../UiLibrary")

local MarketplaceService = game:GetService("MarketplaceService")
local ScriptEditorService = game:GetService("ScriptEditorService")
local HttpService = game:GetService("HttpService")

local currentVersion = script.Parent.Parent.Version.Value
local busy = false

type ProductInfo = {
	AssetTypeId: number,
	TargetId: number,
	IsNew: boolean,
	Updated: string,
	AssetId: number,
	ProductId: number,
	MinimumMembershipLevel: number,
	Created: string,
	Creator: {
		Name: string,
		CreatorTargetId: number,
		Id: number,
		HasVerifiedBadge: boolean,
		CreatorType: string,
	},
	IsLimited: boolean,
	ContentRatingTypeId: number,
	Sales: number,
	IsPublicDomain: boolean,
	Name: string,
	IsForSale: boolean,
	Description: string,
	IconImageAssetId: number,
	IsLimitedUnique: boolean,
}

type Request = {
	placeId: number,
	creatorId: number,
	ids: { number },
	defaultPlaceIds: { number },
	pluginVersion: string,
	assetType: string,
	exportJson: boolean,
	isGroup: boolean,
}

-- callbacks the dashboard passes in to mirror progress in its label
-- all are optional missing ones become no-ops
export type Callbacks = {
	onSent: ((count: number) -> ())?,
	onProgress: ((done: number, total: number) -> ())?,
	onDone: ((replaced: number) -> ())?,
	onError: ((message: string) -> ())?,
}

local function noop() end

local function getCallbacks(c: Callbacks?): {
	onSent: (count: number) -> (),
	onProgress: (done: number, total: number) -> (),
	onDone: (replaced: number) -> (),
	onError: (message: string) -> (),
}
	c = c or {}
	return {
		onSent = (c :: any).onSent or noop,
		onProgress = (c :: any).onProgress or noop,
		onDone = (c :: any).onDone or noop,
		onError = (c :: any).onError or noop,
	}
end

local function scriptInjectionEnabled(): boolean
	local testScript = Instance.new("Script")
	local success = pcall(ScriptEditorService.UpdateSourceAsync, ScriptEditorService, testScript, function()
		return ""
	end)
	testScript:Destroy()
	return success
end

local function getGameInfo(): (boolean, number, number)
	local creatorId, placeId = game.CreatorId, game.PlaceId
	if placeId == 0 then return false, creatorId, placeId end

	if creatorId ~= 0 then
		local isGroup = game.CreatorType == Enum.CreatorType.Group
		return isGroup, creatorId, placeId
	end

	local success, result = retry(3, MarketplaceService.GetProductInfo, MarketplaceService, placeId)
	if not success then return false, creatorId, placeId end

	local creatorInfo = (result :: ProductInfo).Creator
	local isGroup = creatorInfo.CreatorType == "Group"
	return isGroup, creatorInfo.CreatorTargetId, placeId
end

return function(
	plugin: Plugin,
	ui: UiLibrary.Ui,
	filterOptions: AssetIdFilter.FilterOptions,
	assetType: "Animation" | "Sound" | "Mesh",
	placeArray: { number }?,
	callbacks: Callbacks?
)
	local cb = getCallbacks(callbacks)

	if busy then
		ui:Notify("Notification", "Plugin is busy.")
		cb.onError("plugin already running a job")
		return
	end
	busy = true

	if not scriptInjectionEnabled() then
		ui:Notify("Notification", "Plugin needs script injection permission.")
		cb.onError("script injection permission required")
		busy = false
		return
	end

	local isGroup, creatorId, placeId = getGameInfo()
	if placeId == 0 then
		ui:Notify("Notification", "Game needs to be published. Restart place after publishing.")
		cb.onError("place must be published first")
		busy = false
		return
	elseif creatorId == 0 then
		ui:Notify("Notification", "Failed to get place info. Try again or restart the place.")
		cb.onError("could not resolve place creator")
		busy = false
		return
	end

	local filteredIds = AssetIdFilter.filterInstances(filterOptions)
	if next(filteredIds) == nil then
		ui:Notify("Notification", "No ids found.")
		cb.onError("no ids found in scan")
		busy = false
		return
	end

	local connection, response = Connection.new(plugin:GetSetting("Port") or 38073)
	if not connection then
		ui:Notify("Notification", "Unable to connect to localhost. Did you start the executable?")
		cb.onError("backend offline, start the desktop app first")
		busy = false
		return
	end

	-- track running totals for the progress callback
	local totalSent = AssetIdFilter.getIdArray(filteredIds)
	local totalCount = #totalSent
	local doneCount = 0
	local replacedCount = 0

	local function onDataRecieved(data: string)
		-- empty body means backend has nothing yet keep polling
		if data == "" or data == "null\n" or data == "null" then return end

		-- "done" is the terminator
		if data == "done" then
			ui:Notify("Notification", "Finished reuploading all ids.")
			cb.onDone(replacedCount)
			busy = false
			(connection :: any):Destroy()
			return
		end

		local success, result = pcall(HttpService.JSONDecode, HttpService, data)
		if not success then
			-- not json and not "done" probably backend transient state ignore
			return
		end

		local items = result :: { { oldId: number, newId: number } }
		if #items == 0 then return end

		AssetIdFilter.replaceIds(filteredIds, items)
		replacedCount += #items
		doneCount += #items
		cb.onProgress(doneCount, totalCount)
		warn(`Replaced {#items} ids`)
	end

	(connection :: any).onDataRecieved = onDataRecieved
	;(connection :: any).onDisconnect = function()
		ui:Notify("Notification", "Disconnected from localhost. (Try reconnecting)")
		cb.onError("backend disconnected mid-job")
		busy = false
	end

	local request: Request = {
		pluginVersion = currentVersion,
		creatorId = creatorId,
		isGroup = isGroup,
		assetType = assetType,
		ids = totalSent,
		placeId = placeId,
		defaultPlaceIds = placeArray or {},
		exportJson = plugin:GetSetting("ExportJSON") or false,
	}

	local success, statusCode = (connection :: any):Send("/reupload", HttpService:JSONEncode(request))
	if not success then
		-- backend rejected the request surface the http code so the user knows why
		local message: string = (StatusCodes :: any)[statusCode] or ("Unable to send data to localhost.\n" .. tostring(statusCode))
		ui:Notify("Notification", message)
		cb.onError(message)
		busy = false
		(connection :: any):Destroy()
		return
	end

	ui:Notify("Notification", `Sent {totalCount} ids to localhost.`)
	cb.onSent(totalCount)

	-- the post might have already returned a first batch of items in its response body
	-- forward those into the same handler so we don't lose them
	if response then onDataRecieved(response) end
end