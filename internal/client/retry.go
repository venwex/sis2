package client

import (
	"context"
	"math/rand/v2"
	"time"
)

const (
	maxAttempts    = 3
	totalTimeout   = 5 * time.Second
	attemptTimeout = 2 * time.Second
	initialBackoff = 100 * time.Millisecond
)

func doWithRetry[T any](ctx context.Context, operation func(context.Context) (T, error), retryable func(error) bool) (T, error) {
	var zero T

	ctx, cancel := context.WithTimeout(ctx, totalTimeout)
	defer cancel()

	backoff := initialBackoff

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return zero, err
		}

		attemptCtx, cancelAttempt := context.WithTimeout(ctx, attemptTimeout)
		result, err := operation(attemptCtx)
		cancelAttempt()

		if err == nil {
			return result, nil
		}

		if ctx.Err() != nil {
			return zero, ctx.Err()
		}

		if attempt == maxAttempts || !retryable(err) {
			return zero, err
		}

		// jitter случайная пауза от 0 до backoff.
		delay := time.Duration(rand.Int64N(int64(backoff) + 1))
		timer := time.NewTimer(delay)

		select {
		case <-ctx.Done():
			timer.Stop()
			return zero, ctx.Err()

		case <-timer.C:
		}

		backoff *= 2
	}

	return zero, context.DeadlineExceeded
}
