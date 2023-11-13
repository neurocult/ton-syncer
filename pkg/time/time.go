package time

import (
	"context"
	"time"
)

// TickWithCtx returns a chan that receives time.Time every time interval ticks.
// This channel will be closed after context cancelation.
func TickWithCtx(ctx context.Context, interval time.Duration) <-chan time.Time {
	ch := make(chan time.Time)
	ticker := time.NewTicker(interval)

	go func() {
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				close(ch)
			case v := <-ticker.C:
				ch <- v
			}
		}
	}()

	return ch
}
