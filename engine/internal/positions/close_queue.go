package positions

import (
	"log"
	"sync/atomic"
)

// CloseQueueMetrics tracks close event delivery health.
type CloseQueueMetrics struct {
	Enqueued   int64
	Delivered  int64
	Dropped    int64
	PeakDepth  int64
	AlertCount int64
}

const (
	closeQueueCapacity = 10_000
	closeQueueAlertAt  = 500
)

// CloseQueue wraps the close-events channel with guaranteed delivery (blocking)
// and observability. Events are never silently dropped.
type CloseQueue struct {
	events  chan CloseEvent
	metrics CloseQueueMetrics
}

func newCloseQueue() *CloseQueue {
	return &CloseQueue{
		events: make(chan CloseEvent, closeQueueCapacity),
	}
}

// Enqueue blocks until the event is accepted — no silent drops.
func (q *CloseQueue) Enqueue(evt CloseEvent) {
	atomic.AddInt64(&q.metrics.Enqueued, 1)
	q.events <- evt
	atomic.AddInt64(&q.metrics.Delivered, 1)

	depth := int64(len(q.events))
	for {
		peak := atomic.LoadInt64(&q.metrics.PeakDepth)
		if depth <= peak || atomic.CompareAndSwapInt64(&q.metrics.PeakDepth, peak, depth) {
			break
		}
	}
	if depth >= closeQueueAlertAt {
		n := atomic.AddInt64(&q.metrics.AlertCount, 1)
		if n == 1 || n%100 == 0 {
			log.Printf("[ALERT] CloseEvents queue depth=%d capacity=%d position=%s",
				depth, closeQueueCapacity, evt.Position.ID)
		}
	}
}

// Receive returns the events channel for consumers.
func (q *CloseQueue) Receive() <-chan CloseEvent {
	return q.events
}

// Depth returns current buffered event count.
func (q *CloseQueue) Depth() int {
	return len(q.events)
}

// Snapshot returns a copy of delivery metrics.
func (q *CloseQueue) Snapshot() CloseQueueMetrics {
	return CloseQueueMetrics{
		Enqueued:   atomic.LoadInt64(&q.metrics.Enqueued),
		Delivered:  atomic.LoadInt64(&q.metrics.Delivered),
		Dropped:    atomic.LoadInt64(&q.metrics.Dropped),
		PeakDepth:  atomic.LoadInt64(&q.metrics.PeakDepth),
		AlertCount: atomic.LoadInt64(&q.metrics.AlertCount),
	}
}
