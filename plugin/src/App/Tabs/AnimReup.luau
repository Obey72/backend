--!strict
-- manual animation reuploader tab
-- paste one or more animation ids hit reupload get the new ids back
-- mirrors the v2 animation reuploader plugin but routes through the local backend
-- so the user's cookie / spoofed creator is reused from the desktop app

local Connection = require("../Connection")
local Retry = require("../Retry")
local StatusCodes = require("../StatusCodes")
local UiLibrary = require("../../UiLibrary")

local HttpService = game:GetService("HttpService")
local MarketplaceService = game:GetService("MarketplaceService")
local StudioService = game:GetService("StudioService")
local KeyframeSequenceProvider = game:GetService("KeyframeSequenceProvider")
local RunService = game:GetService("RunService")

local currentVersion = script.Parent.Parent.Parent.Version.Value

local function getGameInfo(): (boolean, number, number)
	local creatorId, placeId = game.CreatorId, game.PlaceId
	if placeId == 0 then return false, creatorId, placeId end
	if creatorId ~= 0 then
		return game.CreatorType == Enum.CreatorType.Group, creatorId, placeId
	end
	local ok, result = Retry(3, MarketplaceService.GetProductInfo, MarketplaceService, placeId)
	if not ok then return false, creatorId, placeId end
	local info = (result :: any).Creator
	return info.CreatorType == "Group", info.CreatorTargetId, placeId
end

local function copyToClipboard(text: string): boolean
	-- setclipboard is the canonical studio plugin global it writes straight
	-- to the system clipboard and works in every studio version we ship to
	-- studioservice:copytoclipboard is a newer api kept as a fallback because
	-- some older studios silently no-op on it
	local ok = pcall(function()
		(setclipboard :: any)(text)
	end)
	if ok then return true end
	ok = pcall(function()
		(StudioService :: any):CopyToClipboard(text)
	end)
	return ok
end

-- ─── primary-colour palette for the preview rigs ────────────────────────────
-- blender-dummy look: distinct primary colours per body region so each joint
-- is easy to track while the animation plays  head is yellow torso blue arms
-- red legs green  matches the kid-toy mannequin reference the user wanted
local rigPalette = {
	head        = Color3.fromRGB(255, 220, 80),   -- yellow
	torso       = Color3.fromRGB(40, 110, 230),   -- blue
	arm         = Color3.fromRGB(220, 50, 50),    -- red
	leg         = Color3.fromRGB(60, 180, 80),    -- green
	hrp         = Color3.fromRGB(180, 180, 188),  -- hidden anyway
}

-- shared part/motor builder so the r15 and r6 rigs don't duplicate setup
local function newPart(parent: Model, name: string, size: Vector3, color: Color3): BasePart
	local p = Instance.new("Part")
	p.Name = name
	p.Size = size
	p.Color = color
	p.Material = Enum.Material.SmoothPlastic
	p.Anchored = false
	p.CanCollide = false
	p.TopSurface = Enum.SurfaceType.Smooth
	p.BottomSurface = Enum.SurfaceType.Smooth
	p.Parent = parent
	return p
end
local function newMotor(name: string, p0: BasePart, p1: BasePart, c0: CFrame, c1: CFrame)
	local m = Instance.new("Motor6D")
	m.Name = name
	m.Part0 = p0
	m.Part1 = p1
	m.C0 = c0
	m.C1 = c1
	m.Parent = p0
end

-- ─── proper r15 preview rig ──────────────────────────────────────────────────
-- canonical roblox block-r15 sizes and motor6d offsets ripped from the studio
-- character template so the rest pose lines up properly and animations don't
-- come out goofy  the previous rig used hand-tuned offsets that didn't match
-- the standard r15 author target which is why everything looked broken
local function buildR15Dummy(): Model
	local model = Instance.new("Model")
	model.Name = "AnimPreviewDummyR15"

	local hrp = newPart(model, "HumanoidRootPart", Vector3.new(2, 2, 1), rigPalette.hrp)
	hrp.Transparency = 1
	hrp.Anchored = true -- pin the root so the rig doesn't drift; joints animate the rest

	-- canonical r15 sizes (block rig from the standard character template)
	local lowerTorso = newPart(model, "LowerTorso",     Vector3.new(2, 0.4, 1),  rigPalette.torso)
	local upperTorso = newPart(model, "UpperTorso",     Vector3.new(2, 1.6, 1),  rigPalette.torso)
	local head       = newPart(model, "Head",           Vector3.new(2, 1, 1),    rigPalette.head)
	local lua = newPart(model, "LeftUpperArm",    Vector3.new(1, 1.4, 1),  rigPalette.arm)
	local lla = newPart(model, "LeftLowerArm",    Vector3.new(1, 1.4, 1),  rigPalette.arm)
	local lh  = newPart(model, "LeftHand",        Vector3.new(1, 0.2, 1),  rigPalette.arm)
	local rua = newPart(model, "RightUpperArm",   Vector3.new(1, 1.4, 1),  rigPalette.arm)
	local rla = newPart(model, "RightLowerArm",   Vector3.new(1, 1.4, 1),  rigPalette.arm)
	local rh  = newPart(model, "RightHand",       Vector3.new(1, 0.2, 1),  rigPalette.arm)
	local lul = newPart(model, "LeftUpperLeg",    Vector3.new(1, 1.4, 1),  rigPalette.leg)
	local lll = newPart(model, "LeftLowerLeg",    Vector3.new(1, 1.4, 1),  rigPalette.leg)
	local lf  = newPart(model, "LeftFoot",        Vector3.new(1, 0.2, 1),  rigPalette.leg)
	local rul = newPart(model, "RightUpperLeg",   Vector3.new(1, 1.4, 1),  rigPalette.leg)
	local rll = newPart(model, "RightLowerLeg",   Vector3.new(1, 1.4, 1),  rigPalette.leg)
	local rf  = newPart(model, "RightFoot",       Vector3.new(1, 0.2, 1),  rigPalette.leg)

	-- spine + neck (canonical r15 c0/c1 offsets these are the exact values
	-- from the standard studio block rig anything that authors against r15
	-- will play correctly when the joints line up like this)
	newMotor("Root",          hrp,        lowerTorso, CFrame.new(0, -0.2, 0),   CFrame.new(0, 0, 0))
	newMotor("Waist",         lowerTorso, upperTorso, CFrame.new(0, 0.2, 0),    CFrame.new(0, -0.8, 0))
	newMotor("Neck",          upperTorso, head,       CFrame.new(0, 0.8, 0),    CFrame.new(0, -0.5, 0))
	-- arms (shoulder attaches outer-top of upper torso to inner-top of upper arm)
	newMotor("LeftShoulder",  upperTorso, lua, CFrame.new(-1, 0.8, 0),    CFrame.new(0.5, 0.6, 0))
	newMotor("LeftElbow",     lua, lla,        CFrame.new(0, -0.7, 0),    CFrame.new(0, 0.7, 0))
	newMotor("LeftWrist",     lla, lh,         CFrame.new(0, -0.7, 0),    CFrame.new(0, 0.1, 0))
	newMotor("RightShoulder", upperTorso, rua, CFrame.new(1, 0.8, 0),     CFrame.new(-0.5, 0.6, 0))
	newMotor("RightElbow",    rua, rla,        CFrame.new(0, -0.7, 0),    CFrame.new(0, 0.7, 0))
	newMotor("RightWrist",    rla, rh,         CFrame.new(0, -0.7, 0),    CFrame.new(0, 0.1, 0))
	-- legs (hip on bottom-outer of lower torso to top of upper leg)
	newMotor("LeftHip",       lowerTorso, lul, CFrame.new(-0.5, -0.2, 0), CFrame.new(0, 0.7, 0))
	newMotor("LeftKnee",      lul, lll,        CFrame.new(0, -0.7, 0),    CFrame.new(0, 0.7, 0))
	newMotor("LeftAnkle",     lll, lf,         CFrame.new(0, -0.7, 0),    CFrame.new(0, 0.1, 0))
	newMotor("RightHip",      lowerTorso, rul, CFrame.new(0.5, -0.2, 0),  CFrame.new(0, 0.7, 0))
	newMotor("RightKnee",     rul, rll,        CFrame.new(0, -0.7, 0),    CFrame.new(0, 0.7, 0))
	newMotor("RightAnkle",    rll, rf,         CFrame.new(0, -0.7, 0),    CFrame.new(0, 0.1, 0))

	local humanoid = Instance.new("Humanoid")
	humanoid.RigType = Enum.HumanoidRigType.R15
	humanoid.Parent = model
	local animator = Instance.new("Animator")
	animator.Parent = humanoid

	model.PrimaryPart = hrp
	hrp.CFrame = CFrame.new(0, 2.5, 0)
	return model
end

-- ─── r6 preview rig ──────────────────────────────────────────────────────────
-- standard 6-part r6 layout with the canonical joint names ("Right Shoulder"
-- etc with spaces) and the exact offsets the studio rig editor produces so r6
-- animations play with the correct rest pose
local function buildR6Dummy(): Model
	local model = Instance.new("Model")
	model.Name = "AnimPreviewDummyR6"

	local hrp = newPart(model, "HumanoidRootPart", Vector3.new(2, 2, 1), rigPalette.hrp)
	hrp.Transparency = 1
	hrp.Anchored = true

	-- canonical r6 sizes
	local torso = newPart(model, "Torso",     Vector3.new(2, 2, 1), rigPalette.torso)
	local head  = newPart(model, "Head",      Vector3.new(2, 1, 1), rigPalette.head)
	local la    = newPart(model, "Left Arm",  Vector3.new(1, 2, 1), rigPalette.arm)
	local ra    = newPart(model, "Right Arm", Vector3.new(1, 2, 1), rigPalette.arm)
	local ll    = newPart(model, "Left Leg",  Vector3.new(1, 2, 1), rigPalette.leg)
	local rl    = newPart(model, "Right Leg", Vector3.new(1, 2, 1), rigPalette.leg)

	-- canonical r6 motor6d offsets (matches studio's rig editor output)
	newMotor("RootJoint",      hrp,   torso, CFrame.new(0, 0, 0),     CFrame.new(0, 0, 0))
	newMotor("Neck",           torso, head,  CFrame.new(0, 1, 0),     CFrame.new(0, -0.5, 0))
	newMotor("Left Shoulder",  torso, la,    CFrame.new(-1, 0.5, 0),  CFrame.new(0.5, 0.5, 0))
	newMotor("Right Shoulder", torso, ra,    CFrame.new(1, 0.5, 0),   CFrame.new(-0.5, 0.5, 0))
	newMotor("Left Hip",       torso, ll,    CFrame.new(-0.5, -1, 0), CFrame.new(0, 1, 0))
	newMotor("Right Hip",      torso, rl,    CFrame.new(0.5, -1, 0),  CFrame.new(0, 1, 0))

	local humanoid = Instance.new("Humanoid")
	humanoid.RigType = Enum.HumanoidRigType.R6
	humanoid.Parent = model
	local animator = Instance.new("Animator")
	animator.Parent = humanoid

	model.PrimaryPart = hrp
	hrp.CFrame = CFrame.new(0, 3, 0)
	return model
end

-- ─── rig auto-detect ─────────────────────────────────────────────────────────
-- roblox keyframesequences name each pose after the *part* it moves not the
-- joint so r6 poses look like "Left Arm" / "Right Leg" (with spaces) and r15
-- poses look like "LeftUpperArm" / "RightFoot" we tally hits in each set and
-- pick the winner; ties or no hits fall back to r15
local r6PartSet: { [string]: true } = {
	["Torso"] = true,
	["Left Arm"] = true, ["Right Arm"] = true,
	["Left Leg"] = true, ["Right Leg"] = true,
}
local r15PartSet: { [string]: true } = {
	["UpperTorso"] = true, ["LowerTorso"] = true,
	["LeftUpperArm"] = true, ["RightUpperArm"] = true,
	["LeftLowerArm"] = true, ["RightLowerArm"] = true,
	["LeftHand"] = true, ["RightHand"] = true,
	["LeftUpperLeg"] = true, ["RightUpperLeg"] = true,
	["LeftLowerLeg"] = true, ["RightLowerLeg"] = true,
	["LeftFoot"] = true, ["RightFoot"] = true,
}

-- detects from an already-fetched keyframesequence so the caller can reuse
-- the same fetch for rig detect + registration without round-tripping twice
local function detectRigFromKfs(kfs: KeyframeSequence): string?
	local r6Count, r15Count = 0, 0
	local function visit(n: Instance)
		if n:IsA("Pose") then
			if r6PartSet[n.Name] then
				r6Count += 1
			elseif r15PartSet[n.Name] then
				r15Count += 1
			end
		end
		for _, c in n:GetChildren() do visit(c) end
	end
	visit(kfs)
	if r15Count > r6Count then return "R15" end
	if r6Count > r15Count then return "R6" end
	return nil
end

-- ─── keyframe fetch + extraction ─────────────────────────────────────────────
-- pulls the keyframesequence using the studio user's creds (which works for
-- anything the user can themselves see) then flattens it into a per-joint
-- pose timeline studio's animator:loadanimation doesn't tick inside
-- worldmodel viewportframes in plugin edit mode AND it fights the
-- experience asset-permission check so we drive motor6d.transform manually
-- on a heartbeat connection instead see playManual below
type PoseSample = { time: number, cframe: CFrame, easeStyle: Enum.PoseEasingStyle, easeDir: Enum.PoseEasingDirection }
type PoseTimeline = { [string]: { PoseSample } }

local function buildPoseTimeline(kfs: KeyframeSequence): (PoseTimeline, number, number)
	local timeline: PoseTimeline = {}
	local maxTime = 0
	local frames = kfs:GetKeyframes()
	for _, kf in frames do
		if kf.Time > maxTime then maxTime = kf.Time end
	end
	local function visit(node: Instance, t: number)
		for _, child in node:GetChildren() do
			if child:IsA("Pose") then
				local pose = child :: Pose
				if not timeline[pose.Name] then timeline[pose.Name] = {} end
				table.insert(timeline[pose.Name], {
					time = t,
					cframe = pose.CFrame,
					easeStyle = pose.EasingStyle,
					easeDir = pose.EasingDirection,
				})
				visit(pose, t)
			end
		end
	end
	for _, kf in frames do visit(kf, kf.Time) end
	-- sort each joint's samples by time so binary-ish lookup in the heartbeat
	-- loop walks them in order without needing to sort every frame
	for _, samples in timeline do
		table.sort(samples, function(a, b) return a.time < b.time end)
	end
	return timeline, #frames, maxTime
end

-- returns: timeline keyframecount length detected rig error
local function preparePreview(animId: number): (PoseTimeline?, number?, number?, string?, string?)
	local ok, result = pcall(function()
		return KeyframeSequenceProvider:GetKeyframeSequenceAsync("rbxassetid://" .. animId)
	end)
	if not ok then return nil, nil, nil, nil, tostring(result) end
	local kfs = result :: KeyframeSequence
	local timeline, count, length = buildPoseTimeline(kfs)
	return timeline, count, length, detectRigFromKfs(kfs), nil
end

-- ease curve approx for the standard poseeasingstyle/direction combos linear
-- is plenty for a preview; covers the common constant/elastic cases too so
-- shouty animations don't look unnaturally smooth
local function applyEase(alpha: number, style: Enum.PoseEasingStyle, dir: Enum.PoseEasingDirection): number
	if style == Enum.PoseEasingStyle.Constant then
		return if dir == Enum.PoseEasingDirection.In then 0 else 1
	end
	if style == Enum.PoseEasingStyle.Linear then return alpha end
	-- cubic ease-in-out approximation for cubic/bounce/elastic close enough
	-- for a viewport preview the upload is unaffected by our easing math
	if dir == Enum.PoseEasingDirection.In then
		return alpha * alpha
	elseif dir == Enum.PoseEasingDirection.Out then
		return 1 - (1 - alpha) * (1 - alpha)
	end
	-- inout
	return if alpha < 0.5 then 2 * alpha * alpha else 1 - 2 * (1 - alpha) * (1 - alpha)
end

return function(ui: UiLibrary.Ui, plugin: Plugin)
	local tab = ui:CreateTab("AnimReup")

	local statusLabel = tab:CreateLabel("[ idle ]")
	local detailLabel = tab:CreateLabel("enter ids below, one per line")

	-- preview viewport: shows the dummy and plays the animation from the
	-- first id in the input lives inside the tab content frame; sized to a
	-- comfortable preview height without dominating the tab built up here
	-- so the rig-mode dropdown below can swap models in/out of it freely
	local viewport = Instance.new("ViewportFrame")
	viewport.Name = "AnimPreviewViewport"
	viewport.Size = UDim2.new(1, 0, 0, 180)
	viewport.BackgroundColor3 = Color3.fromRGB(18, 18, 24)
	viewport.BorderSizePixel = 0
	viewport.LayoutOrder = 50  -- high order so the preview lands at the bottom of the tab

	local previewCorner = Instance.new("UICorner")
	previewCorner.CornerRadius = UDim.new(0, 6)
	previewCorner.Parent = viewport

	local worldModel = Instance.new("WorldModel")
	worldModel.Parent = viewport
	local previewCamera = Instance.new("Camera")
	previewCamera.CFrame = CFrame.lookAt(Vector3.new(0, 3.5, 9), Vector3.new(0, 2.5, 0))
	previewCamera.FieldOfView = 50
	previewCamera.Parent = viewport
	viewport.CurrentCamera = previewCamera

	-- swap-in/out rig state held in upvalues so preview/stop callbacks can
	-- reach across to the active dummy regardless of whether the user has
	-- flipped rig modes since the last preview
	local currentRig: string = "R15"
	local previewConn: RBXScriptConnection? = nil

	-- build the initial r15 rig eagerly so the viewport renders something the
	-- moment the tab opens setrig swaps it out when the user flips rig modes
	local dummy = buildR15Dummy()
	dummy.Parent = worldModel

	-- disconnects the heartbeat loop and snaps every joint back to its rest
	-- pose so the dummy stops in a neutral position between previews
	local function stopCurrent()
		if previewConn then
			previewConn:Disconnect()
			previewConn = nil
		end
		for _, d in dummy:GetDescendants() do
			if d:IsA("Motor6D") then
				(d :: Motor6D).Transform = CFrame.new()
			end
		end
	end

	-- swap the dummy in the viewport for a freshly built rig of the given type
	-- killing any playing track first no-op if the current rig already matches
	local function setRig(kind: string)
		if kind == currentRig then return end
		stopCurrent()
		pcall(function() dummy:Destroy() end)
		dummy = if kind == "R6" then buildR6Dummy() else buildR15Dummy()
		dummy.Parent = worldModel
		currentRig = kind
	end

	-- ─── manual joint driver ─────────────────────────────────────────────────
	-- animator:loadanimation does not tick inside a plugin viewportframe in
	-- edit mode (no physics step) so we drive the rig ourselves at heartbeat
	-- rate by walking the pose timeline lerping the bracketing samples for
	-- each joint and writing the result to motor6d.transform this replicates
	-- what the engine would do for us at runtime
	local function playManual(timeline: PoseTimeline, length: number, looped: boolean)
		stopCurrent()
		if length <= 0 then return end

		-- pose.name in a keyframesequence matches motor6d.part1.name (the
		-- part the joint moves) not the motor6d's own name so we key by
		-- part1 name  we also map by motor6d name as a fallback in case an
		-- animation tool keyed poses by joint name instead of part name
		local joints: { [string]: Motor6D } = {}
		for _, d in dummy:GetDescendants() do
			if d:IsA("Motor6D") then
				local m = d :: Motor6D
				if m.Part1 then joints[m.Part1.Name] = m end
				if m.Name ~= "" then joints[m.Name] = m end
			end
		end

		local startClock = os.clock()
		previewConn = RunService.Heartbeat:Connect(function()
			local elapsed = os.clock() - startClock
			if elapsed > length then
				if looped then
					startClock = os.clock()
					elapsed = 0
				else
					if previewConn then previewConn:Disconnect() end
					previewConn = nil
					return
				end
			end

			for jointName, samples in timeline do
				local joint = joints[jointName]
				if not joint then continue end
				local n = #samples
				if n == 0 then continue end

				-- find bracketing samples: prev <= elapsed <= next  the timeline
				-- is sorted so a single linear scan is fine for the joint counts
				-- we hit (~20 max for r15) and the array stays cache-warm
				local prev = samples[1]
				local nextS = samples[n]
				if elapsed <= prev.time then
					nextS = prev
				elseif elapsed >= nextS.time then
					prev = nextS
				else
					for i = 1, n - 1 do
						if samples[i].time <= elapsed and samples[i + 1].time >= elapsed then
							prev = samples[i]
							nextS = samples[i + 1]
							break
						end
					end
				end

				if prev.time == nextS.time then
					joint.Transform = prev.cframe
				else
					local raw = (elapsed - prev.time) / (nextS.time - prev.time)
					local alpha = applyEase(raw, nextS.easeStyle, nextS.easeDir)
					joint.Transform = prev.cframe:Lerp(nextS.cframe, alpha)
				end
			end
		end)
	end

	-- ─── dropdowns: asset type + rig mode ────────────────────────────────────
	-- type dropdown determines whether the rest of the tab is an animation
	-- job or a sound job  when sound the preview ui is hidden + disabled
	-- since previewing a sound on a viewportframe makes no sense and the old
	-- code path would try to loadanimation on an audio asset
	local typeDropdown: any
	local rigDropdown: any
	local previewBtn: any
	local stopBtn: any

	typeDropdown = tab:CreateDropdown({ "Animation", "Sound" }, function(opt)
		detailLabel:ChangeText(`reuploading as: {opt}`)
		local isAnim = opt == "Animation"
		viewport.Visible = isAnim
		if rigDropdown then rigDropdown.instance.Visible = isAnim end
		if previewBtn then previewBtn.instance.Visible = isAnim end
		if stopBtn then stopBtn.instance.Visible = isAnim end
		if not isAnim then
			-- kill any in-flight preview so audio jobs don't ride a stale track
			stopCurrent()
		end
	end)

	local inputBox = tab:CreateInput("asset id(s), one per line", true)

	-- rig-mode dropdown: auto detects from the first id's keyframe joints
	-- r15 / r6 force the dummy regardless of detection useful when you know
	-- the source rig and want to skip the detection round-trip
	rigDropdown = tab:CreateDropdown({ "Auto", "R15", "R6" }, function(opt)
		if opt == "R15" or opt == "R6" then
			if currentRig ~= opt then setRig(opt) end
			detailLabel:ChangeText(`preview rig: {opt}`)
		else
			detailLabel:ChangeText("preview rig: auto (detected on preview)")
		end
	end)

	-- viewport gets parented after the dropdowns and input layoutorder=50
	-- lands it below the rest of the controls so the preview is the bottom
	-- panel of the tab content
	viewport.Parent = tab.instance

	local busy = false
	local results: { { old: number, new: number } } = {}

	previewBtn = tab:CreateButton("preview first id", function()
		if typeDropdown:GetSelection() ~= "Animation" then
			ui:Notify("Notification", "Preview only works for animations.")
			return
		end

		local raw = inputBox.instance.Text or ""
		local id: number? = nil
		for token in raw:gmatch("[%d]+") do
			id = tonumber(token)
			if id then break end
		end
		if not id then
			ui:Notify("Notification", "Paste an animation id first.")
			detailLabel:ChangeText("nothing to preview  paste an id first")
			return
		end

		stopCurrent()
		detailLabel:ChangeText(`fetching {id}...`)

		-- fetch the keyframesequence flatten to a per-joint timeline detect
		-- the rig from the same kfs so we only round-trip once  playback is
		-- manual via heartbeat (see playManual above) because animator tracks
		-- don't tick in plugin viewports
		local rigMode = rigDropdown:GetSelection()
		task.spawn(function()
			local timeline, count, length, detected, err = preparePreview(id :: number)
			if not timeline then
				detailLabel:ChangeText(`preview failed: {err or "unknown"}`)
				return
			end

			-- decide rig: auto trusts detection r15/r6 is explicit
			local target = if rigMode == "Auto" then (detected or currentRig) else rigMode
			if currentRig ~= target then setRig(target) end

			playManual(timeline, length or 0, true)

			local rigNote = if rigMode == "Auto" and detected then `auto: {detected}` else `{currentRig}`
			detailLabel:ChangeText(string.format("preview: %d keyframes, %.2fs · %s rig", count or 0, length or 0, rigNote))
		end)
	end)

	stopBtn = tab:CreateButton("stop preview", function()
		stopCurrent()
		detailLabel:ChangeText("preview stopped")
	end)

	local reupBtn = tab:CreateButton("reupload", function()
		if busy then
			ui:Notify("Notification", "Already running.")
			return
		end

		local raw = inputBox.instance.Text or ""
		local ids: { number } = {}
		local seen: { [number]: true } = {}
		for token in raw:gmatch("[%w]+") do
			local num = tonumber(token)
			if num then
				local id = math.floor(num)
				if not seen[id] then
					seen[id] = true
					table.insert(ids, id)
				end
			end
		end

		if #ids == 0 then
			ui:Notify("Notification", "No valid ids found.")
			statusLabel:ChangeText("[ blocked ]")
			detailLabel:ChangeText("no valid animation ids entered")
			return
		end

		if game.PlaceId == 0 then
			ui:Notify("Notification", "Place must be published before reuploading.")
			statusLabel:ChangeText("[ blocked ]")
			detailLabel:ChangeText("place must be published first")
			return
		end

		busy = true
		results = {}
		statusLabel:ChangeText("[ running ]")
		detailLabel:ChangeText("connecting to backend...")

		task.spawn(function()
			local isGroup, creatorId, placeId = getGameInfo()
			if placeId == 0 or creatorId == 0 then
				statusLabel:ChangeText("[ failed ]")
				detailLabel:ChangeText("could not resolve place info, try again")
				busy = false
				return
			end

			local connection, response = Connection.new(plugin:GetSetting("Port") or 38073)
			if not connection then
				ui:Notify("Notification", "Backend offline. Start the desktop app first.")
				statusLabel:ChangeText("[ failed ]")
				detailLabel:ChangeText("backend offline, start the desktop app")
				busy = false
				return
			end

			local doneCount = 0
			local skippedCount = 0
			local skippedBreakdown: { [string]: number } = {}
			local total = #ids
			local assetType = typeDropdown:GetSelection()
			local port = plugin:GetSetting("Port") or 38073

			local function formatSkipped(): string
				if skippedCount == 0 then return "" end
				local parts = {}
				for reason, n in skippedBreakdown do
					if n > 0 then
						table.insert(parts, `{n} {reason:gsub("_", " ")}`)
					end
				end
				if #parts == 0 then return ` ({skippedCount} skipped)` end
				return ` ({table.concat(parts, ", ")})`
			end

			local function fmtId(n: any): string
				local num = tonumber(n)
				if num and num > 0 then return string.format("%d", num) end
				return "?"
			end

			local function renderProgress(finished: boolean)
				local lines = {}
				for _, r in results do
					table.insert(lines, fmtId(r.old) .. " -> " .. fmtId(r.new))
				end
				local progress = doneCount + skippedCount
				local header = if finished
					then `done: {progress} / {total}{formatSkipped()}`
					else `reuploading: {progress} / {total}{formatSkipped()}`
				table.insert(lines, 1, header)
				detailLabel:ChangeText(table.concat(lines, "\n"))
			end

			-- poll /status every 2s for skip counters (skipped ids never come through the
			-- main /data stream only via /status) when skipped goes up advance progress
			-- so the user sees forward motion on jobs full of already-owned/accessible ids
			task.spawn(function()
				while busy do
					task.wait(2)
					if not busy then return end
					local ok, body = pcall(HttpService.GetAsync, HttpService, `http://localhost:{port}/status`)
					if not ok then continue end
					local parsedOk, parsed = pcall(HttpService.JSONDecode, HttpService, body)
					if not parsedOk then continue end
					local s = parsed :: any
					local newSkipped = tonumber(s.skippedTotal) or 0
					local changed = newSkipped ~= skippedCount
					skippedCount = newSkipped
					if typeof(s.skipped) == "table" then
						skippedBreakdown = s.skipped
					end
					if changed then renderProgress(false) end
				end
			end)

			local function onDataRecieved(data: string)
				if data == "" or data == "null\n" or data == "null" then return end

				if data == "done" then
					if #results == 0 and skippedCount == 0 then
						ui:Notify("Notification", "Done, but no ids were replaced.")
						statusLabel:ChangeText("[ done ]")
						detailLabel:ChangeText("finished, no ids were swapped (check cookie)")
					else
						local msg = `Done. Replaced {#results} {string.lower(assetType)}(s).`
						if skippedCount > 0 then msg ..= ` Skipped {skippedCount}.` end
						ui:Notify("Notification", msg)
						statusLabel:ChangeText("[ done ]")
						renderProgress(true)
					end
					busy = false
					;(connection :: any):Destroy()
					return
				end

				local ok, parsed = pcall(HttpService.JSONDecode, HttpService, data)
				if not ok then return end

				-- the backend returns a json array of {oldid newid} but tolerate either casing
				-- and either a bare array or {items=[]} envelope so a future tweak to the
				-- backend doesn't silently break this tab
				local rawItems
				if typeof(parsed) == "table" then
					if typeof((parsed :: any).items) == "table" then
						rawItems = (parsed :: any).items
					else
						rawItems = parsed
					end
				end
				if typeof(rawItems) ~= "table" then return end

				local accepted = 0
				for _, item in rawItems :: { any } do
					if typeof(item) ~= "table" then continue end
					local raw = item :: any
					local old = tonumber(raw.oldId) or tonumber(raw.OldID) or tonumber(raw.old) or tonumber(raw.old_id)
					local new = tonumber(raw.newId) or tonumber(raw.NewID) or tonumber(raw.new) or tonumber(raw.new_id)
					if (old and old > 0) or (new and new > 0) then
						table.insert(results, { old = old or 0, new = new or 0 })
						accepted += 1
					else
						-- surface unexpected payloads to the output window so we can spot a backend regression
						warn("[AnimReup] dropped item with no usable id:", HttpService:JSONEncode(raw))
					end
				end
				if accepted == 0 then return end
				doneCount += accepted
				statusLabel:ChangeText("[ running ]")
				renderProgress(false)
			end

			;(connection :: any).onDataRecieved = onDataRecieved
			;(connection :: any).onDisconnect = function()
				ui:Notify("Notification", "Disconnected from backend.")
				statusLabel:ChangeText("[ failed ]")
				detailLabel:ChangeText("backend disconnected mid-job")
				busy = false
			end

			local request = {
				pluginVersion = currentVersion,
				creatorId = creatorId,
				isGroup = isGroup,
				assetType = assetType,
				ids = ids,
				placeId = placeId,
				defaultPlaceIds = _G.getPlaceList and _G.getPlaceList() or {},
				exportJson = false,
			}

			statusLabel:ChangeText("[ running ]")
			detailLabel:ChangeText(`sending {total} {string.lower(assetType)} id(s) to backend...`)

			local sent, statusCode = (connection :: any):Send("/reupload", HttpService:JSONEncode(request))
			if not sent then
				local message = (StatusCodes :: any)[statusCode] or "failed to send to backend: " .. tostring(statusCode)
				ui:Notify("Notification", message)
				statusLabel:ChangeText("[ failed ]")
				detailLabel:ChangeText(message)
				busy = false
				;(connection :: any):Destroy()
				return
			end

			ui:Notify("Notification", `Sent {total} {string.lower(assetType)} id(s) to backend.`)
			if response then onDataRecieved(response) end
		end)
	end)

	tab:CreateButton("copy new ids", function()
		-- only collect entries that actually have a usable new id  the result
		-- list can include partial rows (old set, new=0) when the backend
		-- reports a skip mid-batch and those would show up as "0" in the
		-- clipboard which is useless
		local lines: { string } = {}
		for _, r in results do
			if r.new and r.new > 0 then
				table.insert(lines, tostring(r.new))
			end
		end
		if #lines == 0 then
			ui:Notify("Notification", "No new ids to copy yet.")
			return
		end
		local text = table.concat(lines, "\n")

		-- always drop the ids into the input box and select-all  even if the
		-- clipboard apis succeed this gives the user a visible copy of the
		-- result they can re-grab without re-running the job and is the only
		-- 100% reliable fallback when both setclipboard and studioservice
		-- silently no-op (which they apparently do in some studio versions)
		local box = inputBox.instance
		box.Text = text
		pcall(function()
			box:CaptureFocus()
			box.CursorPosition = -1
			box.SelectionStart = 1
		end)

		if copyToClipboard(text) then
			ui:Notify("Notification", "Copied " .. #lines .. " new id(s) to clipboard.")
			detailLabel:ChangeText(`copied {#lines} new id(s) to clipboard · also dropped into the input box`)
		else
			ui:Notify("Notification", "Clipboard blocked. Ids placed in input box, press Ctrl+A then Ctrl+C.")
			detailLabel:ChangeText(`clipboard blocked  ids placed in input box  press Ctrl+A then Ctrl+C to copy`)
		end
	end)

	tab:CreateLabel("tip: ids must belong to your account or group")

	detailLabel:ChangeText(`reuploading as: {typeDropdown:GetSelection()}`)

	-- gate the reupload button behind backend reachability same pattern as
	-- the dashboard tab cheap /status probe once a second; disable + grey out
	-- when the backend doesn't respond
	local reupBtnInstance = (reupBtn :: any).instance :: TextButton
	local function setReupActive(active: boolean)
		reupBtnInstance.Active = active
		reupBtnInstance.AutoButtonColor = active
		reupBtnInstance.TextTransparency = active and 0 or 0.55
		reupBtnInstance.BackgroundTransparency = active and 0 or 0.4
	end
	setReupActive(false)
	task.spawn(function()
		while true do
			task.wait(1)
			local port = plugin:GetSetting("Port") or 38073
			local ok = pcall(HttpService.GetAsync, HttpService, `http://localhost:{port}/status`)
			setReupActive(ok and not busy)
		end
	end)
end
