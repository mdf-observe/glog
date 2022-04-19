package glog

import (
	"fmt"
	"sync/atomic"
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

func checkAllowedLoop(rl *RateLimiter, loops int) error {
	for i := 0; i < loops; i++ {
		if !rl.Allowed() {
			return fmt.Errorf("rl.shouldRateLimit() should never return true")
		}
	}
	return nil
}

func TestInifiniteRateLimiterDoesntRateLimit(t *testing.T) {
	const nParallel int = 16

	rl := newInfiniteRateLimiter(func(int) {})
	errs := make(chan error, nParallel)

	for n := 0; n < nParallel; n++ {
		go func() {
			//	can't spend TOO long running this test, but do it enough that it matters
			err := checkAllowedLoop(rl, 500000)
			errs <- err
		}()
	}

	// Wait for all to finish
	for n := 0; n < nParallel; n++ {
		err := <-errs
		if err != nil {
			t.Error(err)
		}
	}
}

// If we allow 5 items per nanosecond, we shouldn't actually ever see something be limited.
func TestParallelRateLimiterDoesntRateLimit(t *testing.T) {
	const burst int = 5
	const timeIval time.Duration = time.Nanosecond
	const nParallel int = 16

	var missed int64
	rl := NewRateLimiter(timeIval, burst, func(miss int) {
		atomic.AddInt64(&missed, int64(miss))
	})
	errs := make(chan error, nParallel)

	for n := 0; n < nParallel; n++ {
		go func() {
			// We can't spend TOO long running this test, but do it enough that it matters.
			err := checkAllowedLoop(rl, 500000)
			errs <- err
		}()
	}

	// Wait for all to finish
	for n := 0; n < nParallel; n++ {
		err := <-errs
		if err != nil {
			t.Error(err)
		}
	}
	if missed > 0 {
		t.Fatalf("Should never drop a line: missed=%d", missed)
	}
}
