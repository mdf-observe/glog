package glog

import (
	"sync"
	"sync/atomic"
	"time"
)

// The RateLimiter limiter is lock-free except when transitioning from throttled to unthrottled.
type RateLimiter struct {
	burstCount         int64
	burstIntervalNanos int64
	count              int64
	unixNanos          int64
	lock               sync.Mutex
	onRelease          func(missed int)
}

func (r *RateLimiter) Allowed() bool {
	count := atomic.AddInt64(&r.count, 1)
	if count <= r.burstCount {
		return true
	}

	// We didn't pass the fast check, so we have to check more.
	now64 := timeNow().UnixNano()
	ts := atomic.LoadInt64(&r.unixNanos)

	// We could assume that a time.Duration is an int64 nanosecond count and do all arithmetic
	// in int64 nanos; this would likely be notably faster.
	delta := now64 - ts
	if delta < r.burstIntervalNanos {
		return false
	}

	// It's possible to do this lock-free, but this path is rare enough it shouldn't matter.
	r.lock.Lock()
	defer r.lock.Unlock()

	// We can allow the event, and so can anyone checking "after" us.  We may race with other
	// threads in resetting the count.  We must reset the count first -- otherwise, if a thread
	// came in between our update to the timestamp and then the count would see that the count
	// is too high and also that not enough time has elapsed.  I.e. we would incorrectly
	// throttle a use.
	//
	// Once the count is reset, new items will immediately be allowed until the threshold is
	// reached again.  If this thread hasn't yet reset the timestamp, another thread may also
	// decide to reset the timestamp.  This may allow too many items, depending on update
	// order, but that seems better than incorrectly throttling, especially since it's less
	// likely that concurrent threads do an allowed() check while we haven't yet updated the
	// timestamp.

	if atomic.LoadInt64(&r.count) >= r.burstCount {
		// We still look like we're over the count, and since we're under the lock, it's our
		// job to reset it.  No one else will decrease it.
		oldCount := atomic.SwapInt64(&r.count, 1 /* for ourself */)

		// Note that in theory we could race with other setters, who already reset the count
		// and timestamp, then the count re-exceeded the threshhold, then we got the lock.  So
		// make sure r.unixNanos always increases.
		oldUnix64 := atomic.LoadInt64(&r.unixNanos)
		if now64 > oldUnix64 {
			atomic.CompareAndSwapInt64(&r.unixNanos, oldUnix64, now64)
		}

		// Subtract 1 because the initial check at the top of the function added 1 that we
		// didn't actually end up throttling.
		if missed := oldCount - r.burstCount - 1; missed > 0 {
			r.onRelease(int(missed))
		}
	}

	return true
}

func NewRateLimiter(timeInterval time.Duration, burst int, onRelease func(missed int)) *RateLimiter {
	if timeInterval < 1 {
		timeInterval = time.Nanosecond
	}
	if burst < 1 {
		burst = 1
	}
	return &RateLimiter{
		burstCount:         int64(burst),
		burstIntervalNanos: int64(timeInterval),
		count:              0,
		unixNanos:          timeNow().UnixNano(),
		onRelease:          onRelease,
	}
}

func newInfiniteRateLimiter(onRelease func(missed int)) *RateLimiter {
	return &RateLimiter{
		burstCount:         0x100000000000000,
		burstIntervalNanos: int64(time.Nanosecond),
		count:              0,
		unixNanos:          timeNow().UnixNano(),
		onRelease:          onRelease,
	}
}
