--!strict
local WaitGroup = {}

local function Add(self: WaitGroup, f: (any) -> ())
    self._threads += 1
    task.spawn(function()
        f()
        self._threads -= 1

        local waitThread = self._waitThread
        if self._threads == 0 and waitThread then
            self._waitThread = nil
            coroutine.resume(waitThread)
        end
    end)
end

local function Wait(self: WaitGroup)
    if self._threads == 0 then return end
    assert(not self._waitThread, "WaitGroup:Wait() already in progress")

    self._waitThread = coroutine.running()
    coroutine.yield()
end

type WaitGroup = {
    _threads: number,
    _waitThread: thread?,

    Add: typeof(Add),
    Wait: typeof(Wait),
}

function WaitGroup.new(): WaitGroup
    return {
        _threads = 0,

        Add=Add,
        Wait=Wait
    }
end

return WaitGroup