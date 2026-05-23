--!strict
return function<A..., R...>(tries: number, func: (A...) -> (R...), ...: A...): (boolean, R...)
	local result: any
	for i=1, tries do
		result = { pcall(func, ...) }
		local success = result[1]
		if success then break end
		task.wait(0.5 * i)
	end
	return result[1], table.unpack(result, 2)
end