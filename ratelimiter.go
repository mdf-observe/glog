package glog

import (
	"sync"
	"sync/atomic"
	"time"
)

// The RateLimiter limiter is lock-free when not throttled, except for rare instances of
// book-keeping every burstCount checks.
type RateLimiter struct {
	burstCount    int64         // const after creation
	burstInterval time.Duration // const after creation
	count         int64         // updated atomically
	lock          sync.Mutex
	timestamp     time.Time // updated under lock
	missed        int       // updated under lock
	onRelease     func(missed int)
}

func (r *RateLimiter) Allowed() bool {
	count := atomic.AddInt64(&r.count, 1)
	if count <= r.burstCount {
		return true
	}

	allowed, missed := r.slowPathAllowed()
	if missed > 0 {
		r.onRelease(missed)
	}
	return allowed
}

// We didn't pass the fast check, so we have to check more.  It's tricky to do this lock-free,
// but this path should be e.g. only 1 in 500 calls: rare enough it shouldn't matter.
func (r *RateLimiter) slowPathAllowed() (bool, int) {
	r.lock.Lock()
	defer r.lock.Unlock()

	// Now that we're locked, did we lose a race to get the lock and someone else reset the
	// count?
	count := atomic.AddInt64(&r.count, 1)
	if count <= r.burstCount {
		return true, 0
	}

	now := timeNow()
	ts := r.timestamp

	delta := now.Sub(ts)
	if delta < r.burstInterval {
		r.missed++
		return false, 0
	}

	// We are still over the count, but it's been long enough that we can reset it.  Since
	// we're under the lock, it's our job.
	atomic.SwapInt64(&r.count, 1 /* for ourself */)
	r.timestamp = now

	missed := r.missed
	r.missed = 0
	return true, missed
}

func NewRateLimiter(timeInterval time.Duration, burst int, onRelease func(missed int)) *RateLimiter {
	if timeInterval < 1 {
		timeInterval = time.Nanosecond
	}
	if burst < 1 {
		burst = 1
	}
	return &RateLimiter{
		burstCount:    int64(burst),
		burstInterval: timeInterval,
		count:         0,
		timestamp:     timeNow(),
		missed:        0,
		onRelease:     onRelease,
	}
}

func newInfiniteRateLimiter(onRelease func(missed int)) *RateLimiter {
	return NewRateLimiter(time.Nanosecond, 0x100000000000000, onRelease)
}
