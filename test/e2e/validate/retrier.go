package validate

import (
	"fmt"
	"time"
)

// RetryableFunc is a function that can be retried
type RetryableFunc func() error

// Retrier is a struct that encapsulates the retrying logic
type Retrier struct {
	MaxRetries    int
	InitialDelay  time.Duration
	MaxDelay      time.Duration
	BackoffFactor float64
}

// Retry executes the given RetryableFunc and retries it if it fails, with exponential backoff
func (r *Retrier) Retry(fn RetryableFunc) error {
	var delay time.Duration
	for retries := 0; retries < r.MaxRetries; retries++ {
		err := fn()
		if err == nil {
			return nil
		}

		// Calculate the delay for the next retry
		delay = r.InitialDelay * time.Duration(1<<uint(retries))
		if delay > r.MaxDelay {
			delay = r.MaxDelay
		}

		// // Add some jitter to the delay
		// jitter := time.Duration(rand.Float64() * float64(delay))
		// delay = delay + (jitter * r.BackoffFactor)

		fmt.Printf("Retrying in %s...\n", delay)
		time.Sleep(delay)
	}

	return fmt.Errorf("max retries (%d) exceeded", r.MaxRetries)
}
