--!strict
local Connection = {}

local HttpService = game:GetService("HttpService")

local function get(port: number): string
	return HttpService:GetAsync(`http://localhost:{port}`)
end

local function post(port: number, path: string, data: string): (string, number?)
	-- post can return either a body an error string or in success cases the response body
	-- httpservice does not give us status codes directly so we wrap it
	local body = HttpService:PostAsync(`http://localhost:{port}{path}`, data)
	return body, nil
end

local function checkConnectionStatus(self: Connection)
	-- bail if destroy was called from a previous tick's ondatarecieved
	if self._destroyed then return end

	local connected, data = pcall(get, self._port)
	if not connected then
		-- transient localhost errors happen during goroutine startup allow a few retries
		self._failureStreak += 1
		if self._failureStreak >= 5 then
			if self.onDisconnect then self.onDisconnect() end
			self:Destroy()
			return
		end
	else
		self._failureStreak = 0
		if self.onDataRecieved then self.onDataRecieved(data :: string) end
	end

	-- ondatarecieved may have triggered self-destroy recheck before rescheduling
	if self._destroyed then return end
	self._thread = task.delay(1, checkConnectionStatus, self)
end

local function Send(self: Connection, path: string, JSON: string): (boolean, string)
	local success, response = pcall(post, self._port, path, JSON)
	return success :: boolean, response :: string
end

local function Destroy(self: Connection)
	-- if we're being called from inside the polling thread (eg ondatarecieved
	-- saw "done" and triggered destroy) trying to coroutineclose on ourselves
	-- throws "cannot close running coroutine" instead mark _destroyed so the
	-- thread exits naturally on its next iteration check
	local current = coroutine.running()
	self._destroyed = true
	if self._thread and self._thread ~= current then
		pcall(coroutine.close, self._thread)
	end
	-- defer the tableclear so we don't strip out fields the polling thread is
	-- still reading after returning from ondatarecieved
	if self._thread == current then
		task.defer(function()
			table.clear(self :: any)
		end)
	else
		table.clear(self :: any)
	end
end

type Connection = {
	_port: number,
	_thread: thread,
	_failureStreak: number,
	_destroyed: boolean,

	onDisconnect: (() -> ())?,
	onDataRecieved: ((data: string) -> ())?,

	Send: typeof(Send),
	Destroy: typeof(Destroy),
}

function Connection.new(port: number): (Connection?, string?)
	local connected, data = pcall(get :: (number) -> string, port)
	if not connected then return nil, nil end

	local self
	self = {
		_port = port,
		_failureStreak = 0,
		_destroyed = false,

		Send = Send,
		Destroy = Destroy,
	} :: Connection
	self._thread = task.delay(1, checkConnectionStatus, self)

	-- the initial get / response is the first batch (or empty body) pass it back so the caller
	-- can drain it through the same handler used by the polling loop
	return self, data ~= "null\n" and (data :: string) or nil
end

return Connection