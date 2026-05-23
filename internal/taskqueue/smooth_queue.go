package taskqueue

import (
	"container/list"
	"sync"
	"sync/atomic"
	"time"
)

// uniformpacer enforces a minimum wall-clock gap between consecutive wait() calls
// used for steady request spacing (no fixed-window bursts) decrement is a no-op so
// it matches the limiter surface used by mesh/sound retry paths
// aimd adaptation: when enableaimd is called the gap becomes dynamic successful
// operations slowly shrink it (additive increase in throughput) and a single 429
// multiplicatively expands it (multiplicative decrease) this auto-tunes to the
// actual api ceiling instead of relying on a fixed configured rate which is
// usually too conservative on a healthy day and too aggressive when roblox is
// already under load
type UniformPacer struct {
	mu   sync.Mutex
	next time.Time
	gap  time.Duration

	// aimd state  only meaningful when aimdon is true
	aimdOn     bool
	baseGap    time.Duration // configured starting gap anchor for the step size
	minGap     time.Duration // floor (max allowed throughput)
	maxGap     time.Duration // ceiling (slowest we'll ever back off to)
	successInc int           // successes between additive-increase bumps
	successCnt int           // successes since the last bump
}

// newuniformpacer returns a pacer with at least mingap between each wait()
func NewUniformPacer(minGap time.Duration) *UniformPacer {
	if minGap < time.Millisecond {
		minGap = time.Millisecond
	}
	return &UniformPacer{gap: minGap}
}

func (p *UniformPacer) Wait() {
	p.mu.Lock()
	defer p.mu.Unlock()
	now := time.Now()
	if !p.next.IsZero() && now.Before(p.next) {
		time.Sleep(p.next.Sub(now))
		now = time.Now()
	}
	p.next = now.Add(p.gap)
}

// addchill pushes the next allowed wait time forward after a rate limit response
func (p *UniformPacer) AddChill(d time.Duration) {
	if d <= 0 {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.next.IsZero() {
		p.next = time.Now().Add(d)
		return
	}
	p.next = p.next.Add(d)
}

// decrement is a no-op (fixedwindow decrements on some network errors)
func (p *UniformPacer) Decrement() {}

// enableaimd turns on adaptive pacing mingap caps how fast we'll go (defaults to
// basegap/3 if <=0); maxgap caps how slow we'll back off (defaults to basegap*4)
// successinc is how many successful wait()s we want between additive-increase
// bumps (defaults to 20 if <=0) the pacer keeps its current gap as the baseline
func (p *UniformPacer) EnableAIMD(minGap, maxGap time.Duration, successInc int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if minGap <= 0 {
		minGap = p.gap / 3
	}
	if maxGap <= 0 {
		maxGap = p.gap * 4
	}
	if minGap < time.Millisecond {
		minGap = time.Millisecond
	}
	if maxGap < minGap {
		maxGap = minGap
	}
	if successInc <= 0 {
		successInc = 20
	}
	p.aimdOn = true
	p.baseGap = p.gap
	p.minGap = minGap
	p.maxGap = maxGap
	p.successInc = successInc
	p.successCnt = 0
}

// recordsuccess tells the pacer an op succeeded after successinc consecutive
// wins it shaves a small slice off the gap so throughput creeps up over time
// no-op if aimd wasn't enabled
func (p *UniformPacer) RecordSuccess() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.aimdOn {
		return
	}
	p.successCnt++
	if p.successCnt < p.successInc {
		return
	}
	p.successCnt = 0
	// additive increase: ~3% of the baseline gap per cycle anchoring the step to
	// basegap (not the current gap) keeps the recovery rate constant whether
	// we're currently fast or backed off
	step := p.baseGap / 33
	if step <= 0 {
		step = time.Millisecond
	}
	if p.gap-step < p.minGap {
		p.gap = p.minGap
		return
	}
	p.gap -= step
}

// recordratelimit tells the pacer we just got a 429 (or equivalent) it
// multiplicatively widens the gap so future ops back off immediately and resets
// the success streak so we earn the speed back from scratch no-op without aimd
func (p *UniformPacer) RecordRateLimit() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.aimdOn {
		return
	}
	p.successCnt = 0
	next := time.Duration(float64(p.gap) * 1.7)
	if next > p.maxGap {
		next = p.maxGap
	}
	if next < p.gap+time.Millisecond {
		// pgap was tiny; force at least a 1ms step so we actually back off
		next = p.gap + time.Millisecond
	}
	p.gap = next
}

// gap returns the current effective spacing for observability / debug logs
func (p *UniformPacer) Gap() time.Duration {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.gap
}

// smoothqueue runs tasks with:
//   1) at most maxconcurrent goroutines inside the task function at once and
//   2) at least (timeminute / startsperminute) between consecutive task starts
// this avoids both fixed-window edge bursts and huge stacks of concurrent operation
// polls that trigger roblox 429 on the assets api
type SmoothQueue[R any] struct {
	// limiter spaces every task start and retry wait(); decrement is a no-op
	Limiter *UniformPacer

	sem                chan struct{}
	mutex              sync.Mutex
	tasks              *list.List
	isSchedulerRunning bool

	// optional anti-burst: after every breatherevery starts sleep breatherpausens (0 = off)
	// call setantiburstbreather before queuetask lets the api cool between bursts without
	// changing the base pacing gap
	breatherEvery   atomic.Int64 // n > 0: pause every n starts
	breatherPauseNs atomic.Int64 // nanoseconds
	breathCount     atomic.Uint64
}

// newsmoothqueue creates a queue with uniform start spacing and a concurrency ceiling
// startsperminute must be > 0; maxconcurrent must be > 0
func NewSmoothQueue[R any](maxConcurrent, startsPerMinute int) *SmoothQueue[R] {
	if maxConcurrent < 1 {
		maxConcurrent = 1
	}
	if startsPerMinute < 1 {
		startsPerMinute = 1
	}
	gap := time.Minute / time.Duration(startsPerMinute)
	p := NewUniformPacer(gap)
	return &SmoothQueue[R]{
		Limiter: p,
		sem:     make(chan struct{}, maxConcurrent),
		tasks:   list.New(),
	}
}

// setantiburstbreather adds a tiny sleep every n upload starts (scheduler only) after
// the normal paced wait use small pause (eg 25, 50ms) and n≈15, 30 so total overhead
// stays low pass every < 1 or pause < 1ms to disable call before queuetask
func (q *SmoothQueue[R]) SetAntiBurstBreather(every int, pause time.Duration) {
	if every < 1 || pause < time.Millisecond {
		q.breatherEvery.Store(0)
		return
	}
	q.breatherPauseNs.Store(int64(pause)) // store before every so scheduler never sees n>0 with unset pause
	q.breatherEvery.Store(int64(every))
}

func (q *SmoothQueue[R]) QueueTask(f func() (R, error)) chan TaskResult[R] {
	c := make(chan TaskResult[R])

	q.mutex.Lock()
	defer q.mutex.Unlock()

	q.tasks.PushBack(task[R]{
		Func: f,
		Chan: c,
	})

	if !q.isSchedulerRunning {
		q.isSchedulerRunning = true
		go q.scheduler()
	}

	return c
}

// chill nudges the uniform pacer after roblox signals rate limiting
func (q *SmoothQueue[R]) Chill(d time.Duration) {
	if q.Limiter != nil {
		q.Limiter.AddChill(d)
	}
}

// enableaimd turns on adaptive pacing on the queue's limiter see
// uniformpacerenableaimd call once after newsmoothqueue before queuetask
func (q *SmoothQueue[R]) EnableAIMD(minGap, maxGap time.Duration, successInc int) {
	if q.Limiter != nil {
		q.Limiter.EnableAIMD(minGap, maxGap, successInc)
	}
}

// recordsuccess feeds an upload success into the limiter's aimd bookkeeping so
// it can speed up after a sustained good run safe to call when aimd is off
func (q *SmoothQueue[R]) RecordSuccess() {
	if q.Limiter != nil {
		q.Limiter.RecordSuccess()
	}
}

// recordratelimit feeds a 429 into the limiter's aimd bookkeeping so it backs
// off immediately for future ops safe to call when aimd is off
func (q *SmoothQueue[R]) RecordRateLimit() {
	if q.Limiter != nil {
		q.Limiter.RecordRateLimit()
	}
}

// pacergap exposes the current effective spacing for status / debug surfaces
// returns 0 if there's no limiter
func (q *SmoothQueue[R]) PacerGap() time.Duration {
	if q.Limiter == nil {
		return 0
	}
	return q.Limiter.Gap()
}

func (q *SmoothQueue[R]) scheduler() {
	for {
		q.mutex.Lock()
		if q.tasks.Len() == 0 {
			q.isSchedulerRunning = false
			q.mutex.Unlock()
			return
		}

		e := q.tasks.Front()
		t := e.Value.(task[R])
		q.tasks.Remove(e)
		q.mutex.Unlock()

		q.sem <- struct{}{}
		q.Limiter.Wait()
		if n := q.breatherEvery.Load(); n > 0 {
			c := q.breathCount.Add(1)
			if c%uint64(n) == 0 {
				time.Sleep(time.Duration(q.breatherPauseNs.Load()))
			}
		}
		go func(t task[R]) {
			defer func() { <-q.sem }()
			res, err := t.Func()
			t.Chan <- TaskResult[R]{
				Result: res,
				Error:  err,
			}
		}(t)
	}
}
