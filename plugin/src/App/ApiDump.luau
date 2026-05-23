--!strict
local ApiDump = {}

local API_DUMP_URL = "https://raw.githubusercontent.com/MaximumADHD/Roblox-Client-Tracker/refs/heads/roblox/API-Dump.json"

local retry = require("./Retry")

local HttpService = game:GetService("HttpService")

local classIndexMap: { [string]: number } = {}
local cachedDump: ApiDump? = nil
local blacklistedTags: { [string]: boolean } = {
	Hidden = true,
	ReadOnly = true,
	NotScriptable = true
}

type Member = {
	Category: string?,
	MemberType: string,
	Name: string,
	Security: {
		Read: string,
		Write: string
	},
	Serialization: {
		CanLoad: boolean,
		CanSave: boolean
	}?,
	Tags: { string }?,
	ThreadSafety: string,
	ValueType: {
		Category: string,
		Name: string
	}?,
	Parameters: {
		{
			Name: string,
			Type: {
				Category: string,
				Name: string
			}
		}
	}?,
	ReturnType: {
		Category: string,
		Name: string
	}?
}

export type ApiDump = {
    Classes: {
		{
			Members: {
				Member
			},
			MemoryCategory: string,
			Name: string,
			Superclass: string,
			Tags: { string }?
		}
    },
    Enums: {
        {
            Items: {
                {
                    Name: string,
                    Value: number
                }
            },
            Name: string,
            Tags:  {string }?
        }
    },
    Version: number
}

function ApiDump.isCached(): boolean
    return cachedDump ~= nil
end

local function cacheDump(apiDump: ApiDump)
	cachedDump = apiDump
	
	for i, class in apiDump.Classes do
		classIndexMap[class.Name] = i
	end
end

function ApiDump.get(): ApiDump?
    if cachedDump then return cachedDump end

    local success, response = retry(3, HttpService.GetAsync, HttpService, API_DUMP_URL)
	if not success then
		warn("Failed to get latest roblox API dump:", response)
		return nil
	end

	local apiDump = HttpService:JSONDecode(response :: string)
	cacheDump(apiDump)
	return cachedDump
end

local function isValidProperty(member: Member): boolean
	if member.MemberType ~= "Property" then return false end
	if member.Security.Write ~= "None" then return false end

	local tags = member.Tags
	if tags then
		for _, tag in tags do
			if not blacklistedTags[tag] then continue end
			return false
		end
	end

	return true
end

function ApiDump.getProperties(className: string): { string }
	assert(cachedDump, "api dump is not cached use ApiDump.get()")
	local properties = {}
	
	local classIndex = classIndexMap[className]
	if not classIndex then error(`{className} is not in api dump cache`) end
	
	local class = cachedDump.Classes[classIndex]
	for _, member in class.Members do
		if not isValidProperty(member) then continue end
		table.insert(properties, member.Name)
	end
	
	if class.Superclass ~= "<<<ROOT>>>" then
		for _, property in ApiDump.getProperties(class.Superclass) do
			table.insert(properties, property)
		end
	end
	
	return properties
end

return ApiDump