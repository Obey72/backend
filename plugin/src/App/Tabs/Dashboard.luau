--!strict
-- single dashboard tab using bacfr uilibrary widgets
-- combines sound/animation/mesh into one place with live status and cancel controls
-- explicit layoutorder on every widget because the bacfr uilistlayout sorts alphabetically
-- by default which puts splits/frames at the top regardless of insertion order

local ApiDump = require("../ApiDump")
local getFilterOptions = require("../GetFilterOptions")
local reuploadIds = require("../ReuploadIds")
local UiLibrary = require("../../UiLibrary")

local HttpService = game:GetService("HttpService")
local Selection = game:GetService("Selection")

local TYPE_TO_CLASSES = {
	Sound = { "Sound" },
	Animation = { "Animation" },
}

local MESH_SETTINGS_TO_CLASS = {
	MeshParts = "MeshPart",
	SpecialMeshes = "SpecialMesh",
	CharacterMeshes = "CharacterMesh",
}

local SELECTION_EXTRA = { "StringValue", "NumberValue", "IntValue", "LuaSourceContainer" }

-- helper that pulls the underlying guiobject out of any uilibrary widget result
-- and sets its layoutorder so things appear in the order we created them
local function order(widget: any, n: number)
	local inst = widget.instance or widget
	if inst and typeof(inst) == "Instance" then
		inst.LayoutOrder = n
	end
end

return function(ui: UiLibrary.Ui, plugin: Plugin)
	local tab = ui:CreateTab("Dashboard")

	-- also force the tab's content frame to actually use layoutorder for sorting
	-- bacfr's asset uses default which is name-alphabetical breaking our intent
	local tabContent = tab.instance
	if tabContent then
		local layout = tabContent:FindFirstChildOfClass("UIListLayout")
		if layout then
			layout.SortOrder = Enum.SortOrder.LayoutOrder
		end
	end

	local function classesForType(assetType: string): { string }
		if assetType == "Mesh" then
			local out: { string } = {}
			for setting, class in MESH_SETTINGS_TO_CLASS do
				if plugin:GetSetting(setting) then
					table.insert(out, class)
				end
			end
			if #out == 0 then
				for _, class in MESH_SETTINGS_TO_CLASS do
					table.insert(out, class)
				end
			end
			return out
		end
		return TYPE_TO_CLASSES[assetType] or {}
	end

	local selectionOnly = false

	local function buildFilter(assetType: string)
		local classes = classesForType(assetType)
		if #classes == 0 then return nil end

		if selectionOnly then
			local whitelist = table.clone(classes)
			for _, extra in SELECTION_EXTRA do
				table.insert(whitelist, extra)
			end
			return {
				WhitelistedInstances = whitelist,
				Instances = Selection:Get(),
			}
		end

		local filter = getFilterOptions(plugin, game:GetDescendants())
		for _, className in classes do
			table.insert(filter.WhitelistedInstances, className)
		end
		return filter
	end

	local statusLabel
	local detailLabel
	local lastJobEnd = 0

	local function checkRequirements(): (boolean, string)
		if game.PlaceId == 0 then
			return false, "place must be published before reuploading"
		end
		return true, ""
	end

	local function startReupload(assetType: string)
		local ok, why = checkRequirements()
		if not ok then
			statusLabel:ChangeText("[ blocked ]")
			detailLabel:ChangeText(why)
			return
		end

		if assetType == "Mesh" and not ApiDump.isCached() then
			detailLabel:ChangeText("fetching api dump...")
			task.spawn(function()
				if ApiDump.get() then
					detailLabel:ChangeText("api dump ready, click mesh again")
				else
					detailLabel:ChangeText("failed to fetch api dump")
				end
			end)
			return
		end

		local filter = buildFilter(assetType)
		if not filter then
			detailLabel:ChangeText("unknown asset type: " .. assetType)
			return
		end

		statusLabel:ChangeText("[ running ] " .. string.lower(assetType))
		detailLabel:ChangeText("scanning place for ids...")

		reuploadIds(plugin, ui, filter, assetType :: any, _G.getPlaceList(), {
			onSent = function(count: number)
				statusLabel:ChangeText("[ running ] " .. string.lower(assetType))
				detailLabel:ChangeText(`sent {count} ids, waiting for backend`)
			end,
			onProgress = function(done: number, total: number)
				if total > 0 then
					detailLabel:ChangeText(`reuploading: {done} / {total}`)
				else
					detailLabel:ChangeText(`reuploading: {done} done`)
				end
			end,
			onDone = function(replaced: number)
				statusLabel:ChangeText("[ done ]")
				detailLabel:ChangeText(`replaced {replaced} references`)
				lastJobEnd = os.clock()
			end,
			onError = function(message: string)
				statusLabel:ChangeText("[ failed ]")
				detailLabel:ChangeText(message)
				lastJobEnd = os.clock()
			end,
		})
	end

	local function sendCancel(keep: boolean)
		local port = plugin:GetSetting("Port") or 38073
		local url = `http://localhost:{port}/cancel?keep=` .. (keep and "true" or "false")
		local ok, err = pcall(function()
			HttpService:RequestAsync({
				Url = url,
				Method = "POST",
			})
		end)
		if not ok then
			detailLabel:ChangeText("cancel failed: " .. tostring(err))
			return
		end
		statusLabel:ChangeText("[ cancelling ]")
		detailLabel:ChangeText(keep and "stopping, will replace what was uploaded" or "stopping, discarding all results")
	end

	-- layout: each widget gets an explicit layoutorder via order()
	-- 1-2: status labels at top
	-- 3-5: reupload buttons
	-- 6: cancel split
	-- 7: selection toggle
	-- 8: mesh options label
	-- 9: mesh subtypes split
	-- 10: character meshes toggle
	statusLabel = tab:CreateLabel("[ idle ]")
	order(statusLabel, 1)

	detailLabel = tab:CreateLabel("waiting for backend")
	order(detailLabel, 2)

	local soundBtn = tab:CreateButton("reupload sounds", function() startReupload("Sound") end)
	order(soundBtn, 3)

	local animBtn = tab:CreateButton("reupload animations", function() startReupload("Animation") end)
	order(animBtn, 4)

	local meshBtn = tab:CreateButton("reupload meshes", function() startReupload("Mesh") end)
	order(meshBtn, 5)

	-- ───────── backend-gated buttons ─────────
	-- when the desktop backend is unreachable (firewall denial app closed smart
	-- screen quarantine) we visibly grey out every button that would otherwise
	-- start a network request so the user can't trigger a no-op
	local reuploadButtons = { soundBtn, animBtn, meshBtn }
	local function setReuploadActive(active: boolean)
		for _, btn in reuploadButtons do
			local inst = (btn :: any).instance :: TextButton
			inst.Active = active
			inst.AutoButtonColor = active
			inst.TextTransparency = active and 0 or 0.55
			inst.BackgroundTransparency = active and 0 or 0.4
		end
	end

	-- cancel buttons as two full-width stacked rows so they match the visual
	-- style of the reupload buttons originally these were in a split (half-width
	-- side by side) but that made the labels truncate and looked inconsistent
	-- next to the full-width reupload buttons above
	local cancelKeepBtn = tab:CreateButton("stop & replace", function() sendCancel(true) end)
	order(cancelKeepBtn, 6)
	local cancelDiscardBtn = tab:CreateButton("stop & discard", function() sendCancel(false) end)
	order(cancelDiscardBtn, 7)

	local selToggle = tab:CreateToggle("selection only", function(state)
		selectionOnly = state
	end, false)
	order(selToggle, 8)

	local meshLabel = tab:CreateLabel("mesh search options")
	order(meshLabel, 9)

	local meshSplit = tab:CreateSplit()
	order(meshSplit, 10)
	meshSplit.instance.Size = UDim2.new(1, 0, 0, 22)
	meshSplit:CreateToggle("mesh parts", function(state)
		plugin:SetSetting("MeshParts", state)
	end, plugin:GetSetting("MeshParts"))
	meshSplit:CreateToggle("special meshes", function(state)
		plugin:SetSetting("SpecialMeshes", state)
	end, plugin:GetSetting("SpecialMeshes"))

	local charToggle = tab:CreateToggle("character meshes", function(state)
		plugin:SetSetting("CharacterMeshes", state)
	end, plugin:GetSetting("CharacterMeshes"))
	order(charToggle, 11)

	-- start in disabled state  the loop below re-enables once /status responds
	setReuploadActive(false)

	-- background status loop
	task.spawn(function()
		local port = plugin:GetSetting("Port") or 38073
		local lastStatusKey = ""
		while true do
			task.wait(1)
			local ok, raw = pcall(HttpService.GetAsync, HttpService, `http://localhost:{port}/status`)
			if not ok then
				setReuploadActive(false)
				if os.clock() - lastJobEnd > 5 and lastStatusKey ~= "offline" then
					statusLabel:ChangeText("[ offline ]")
					detailLabel:ChangeText("backend blocked or closed  open re/up to allow it")
					lastStatusKey = "offline"
				end
				continue
			end

			-- reached the backend → safe to re-enable
			setReuploadActive(true)

			local parsedOk, parsed = pcall(HttpService.JSONDecode, HttpService, raw)
			if not parsedOk or type(parsed) ~= "table" then continue end

			local p = parsed :: any
			if p.busy then continue end
			if os.clock() - lastJobEnd < 5 then continue end

			if game.PlaceId == 0 then
				if lastStatusKey ~= "no-place" then
					statusLabel:ChangeText("[ no place ]")
					detailLabel:ChangeText("publish this place first to enable reuploading")
					lastStatusKey = "no-place"
				end
				continue
			end

			local key
			if p.lastError and p.lastError ~= "" then
				key = "err:" .. p.lastError
				if key ~= lastStatusKey then
					statusLabel:ChangeText("[ error ]")
					detailLabel:ChangeText(p.lastError)
					lastStatusKey = key
				end
			elseif not p.cookieOk then
				key = "no-cookie"
				if key ~= lastStatusKey then
					statusLabel:ChangeText("[ no cookie ]")
					detailLabel:ChangeText("paste a .ROBLOSECURITY in the desktop app")
					lastStatusKey = key
				end
			else
				key = "ready:" .. tostring(p.userId)
				if key ~= lastStatusKey then
					statusLabel:ChangeText("[ ready ]")
					detailLabel:ChangeText(`signed in as user {p.userId}`)
					lastStatusKey = key
				end
			end
		end
	end)
end