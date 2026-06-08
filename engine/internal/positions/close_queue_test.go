package positions

import (
	"fmt"
	"sync"
	"testing"
)

func TestCloseQueueNoDropsUnderLoad(t *testing.T) {
	q := newCloseQueue()
	const n = 10_000

	var wg sync.WaitGroup
	wg.Add(1)
	received := 0
	go func() {
		defer wg.Done()
		ch := q.Receive()
		for i := 0; i < n; i++ {
			<-ch
			received++
		}
	}()

	for i := 0; i < n; i++ {
		q.Enqueue(CloseEvent{
			Position: Position{ID: fmt.Sprintf("pos-%d", i)},
		})
	}
	wg.Wait()

	metrics := q.Snapshot()
	if metrics.Dropped != 0 {
		t.Fatalf("dropped=%d want 0", metrics.Dropped)
	}
	if received != n {
		t.Fatalf("received=%d want %d", received, n)
	}
	if metrics.Enqueued != int64(n) {
		t.Fatalf("enqueued=%d want %d", metrics.Enqueued, n)
	}
}
