package glog

import (
	"sync"
	"testing"
	"time"
)

func TestRateLimiterLimits(t *testing.T) {
	np := time.Now()
	now := &np
	timeNow = func() time.Time { return *now }
	defer func() { timeNow = time.Now }()

	const kBurst int = 6
	const kInterval time.Duration = time.Minute

	skipped := -1
	rl := NewRateLimiter(kInterval, kBurst, func(sk int) { skipped = sk })
	for i := 0; i != kBurst; i++ {
		if !rl.Allowed() {
			t.Errorf("limits at iteration %d when it shouldn't", i)
		}
	}
	if rl.Allowed() {
		t.Errorf("does not limit when it should")
	}
	np = np.Add(kInterval / 2)
	if rl.Allowed() {
		t.Errorf("rate limiter shouldn't have advanced")
	}
	np = np.Add(kInterval / 2)
	for i := 0; i != kBurst; i++ {
		if !rl.Allowed() {
			t.Errorf("iteration %d shouldn't limit now", i)
		}
	}
	if skipped != 2 {
		t.Errorf("expected 2 skipped items, not %d", skipped)
	}
	for i := 0; i != kBurst; i++ {
		if rl.Allowed() {
			t.Errorf("iteration %d should limit now", i)
		}
	}
}

func TestInifiniteRateLimiterDoesntRateLimit(t *testing.T) {
	rl := newInfiniteRateLimiter(func(int) {})
	wg := sync.WaitGroup{}
	for n := 0; n != 16; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			//	can't spend TOO long running this test, but do it enough that it matters
			for i := 0; i != 500000; i++ {
				if !rl.Allowed() {
					t.Fatal("rl.shouldRateLimit() should never return true for an infinite rate limiter!")
				}
			}
		}()
	}
	wg.Wait()
}
