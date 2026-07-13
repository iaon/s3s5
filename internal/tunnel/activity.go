package tunnel

import (
	"context"
	"time"
)

type activitySignal struct {
	ch chan struct{}
}

func newActivitySignal() *activitySignal {
	return &activitySignal{ch: make(chan struct{}, 1)}
}

func notifyActivity(a *activitySignal) {
	if a == nil {
		return
	}
	select {
	case a.ch <- struct{}{}:
	default:
	}
}

func sleepOrActivity(ctx context.Context, delay, activeDuration time.Duration, a *activitySignal) (bool, error) {
	if a == nil || activeDuration <= 0 {
		return false, sleep(ctx, delay)
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false, ctx.Err()
	case <-timer.C:
		return false, nil
	case <-a.ch:
		return true, nil
	}
}
