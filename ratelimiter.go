package glog

import (
	"sync"
	"sync/atomic"
	"time"
)

//	This log rate limiter doesn't use locking on the regular path,
//	but instead only does housekeeping every "burst" log lines.
type logRateLimiter struct {
	lastCount    int64
	burstSize    int64
	lastTime     time.Time
	timeInterval time.Duration
	lock         sync.Mutex
}

//
func (l *logRateLimiter) shouldRateLimit() bool {
	count := atomic.AddInt64(&l.lastCount, 1)
	if count > l.burstSize {
		l.lock.Lock()
		defer l.lock.Unlock()
		//	re-load, because we may have raced with someone else
		count = atomic.LoadInt64(&l.lastCount)
		if count > l.burstSize {
			now := timeNow()
			since := now.Sub(l.lastTime)
			//	This relies on integer division truncating down
			n := int64(since) * int64(l.burstSize) / int64(l.timeInterval)
			if n <= 0 { // rate limited!
				//	I didn't print anything
				atomic.AddInt64(&l.lastCount, -1)
				return true
			}
			if n >= count {
				count = 1
				l.lastTime = now
			} else {
				//	I logged -- so don't delete that
				count -= (n - 1)
				plusDelta := time.Duration(n * int64(l.timeInterval) / l.burstSize)
				l.lastTime = l.lastTime.Add(plusDelta)
			}
			atomic.StoreInt64(&l.lastCount, count)
		}
	}
	return false
}

func newRateLimiter(timeInterval time.Duration, burst int) *logRateLimiter {
	if timeInterval < 1 {
		timeInterval = time.Nanosecond
	}
	if burst < 1 {
		burst = 1
	}
	return &logRateLimiter{
		burstSize:    int64(burst),
		lastTime:     timeNow(),
		timeInterval: timeInterval,
	}
}

func newInfiniteRateLimiter() *logRateLimiter {
	return &logRateLimiter{
		lastTime:     timeNow(),
		burstSize:    0x100000000000000,
		timeInterval: time.Nanosecond,
	}
}
