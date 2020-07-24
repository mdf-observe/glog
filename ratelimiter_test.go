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

	rl := newRateLimiter(time.Minute, 6)
	for i := 0; i != 6; i++ {
		if rl.shouldRateLimit() {
			t.Errorf("limits at iteration %d when it shouldn't", i)
		}
	}
	if !rl.shouldRateLimit() {
		t.Errorf("does not limit when it should")
	}
	np = np.Add(time.Duration(9) * time.Second)
	if !rl.shouldRateLimit() {
		t.Errorf("rate limiter doesn't round interval down")
	}
	np = np.Add(time.Duration(2) * time.Second)
	if rl.shouldRateLimit() {
		t.Errorf("rate limiter didn't let up when time advanced")
	}
	np = np.Add(time.Duration(2) * time.Second)
	if !rl.shouldRateLimit() {
		t.Errorf("rate limiter should limit again")
	}
	np = np.Add(time.Duration(2) * time.Minute)
	for i := 0; i != 6; i++ {
		if rl.shouldRateLimit() {
			t.Errorf("iteration %d shouldn't limit now", i)
		}
	}
	for i := 0; i != 6; i++ {
		if !rl.shouldRateLimit() {
			t.Errorf("iteration %d should limit now", i)
		}
	}
}

func TestInifiniteRateLimiterDoesntRateLimit(t *testing.T) {
	rl := newInfiniteRateLimiter()
	wg := sync.WaitGroup{}
	for n := 0; n != 16; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			//	can't spend TOO long running this test, but do it enough that it matters
			for i := 0; i != 500000; i++ {
				if rl.shouldRateLimit() {
					t.Fatal("rl.shouldRateLimit() should never return true for an infinite rate limiter!")
				}
			}
		}()
	}
	wg.Wait()
}
