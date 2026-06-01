package v3

import (
	"math"
	"time"
)

// MakerTakerClass classifies an order as passive maker or aggressive taker.
type MakerTakerClass string

const (
	ClassMaker MakerTakerClass = "MAKER"
	ClassTaker MakerTakerClass = "TAKER"
)

// QueueOrder represents an order waiting in the matching engine queue.
type QueueOrder struct {
	OrderID      string
	Side         string // "BUY" | "SELL"
	Type         string // "LIMIT" | "POST_ONLY"
	LimitPrice   float64
	Quantity     float64
	SubmittedAt  time.Time
	MakerTaker   MakerTakerClass
}

// QueueFillResult is the estimated fill outcome from queue position analysis.
type QueueFillResult struct {
	Class               MakerTakerClass
	QueueRank           int
	EstimatedQueueDepth float64 // USD ahead of this order
	FillProbability     float64 // 0–1
	ExpectedFillTime    time.Duration
	ExpectedFillRatio   float64 // fraction of order expected to fill
	PartialFills        []OrderPartialFillEvent
}

// QueuePositionEngine estimates queue position and fill probability for limit orders.
type QueuePositionEngine struct {
	// ArrivalRate is estimated orders per second at a given price level.
	ArrivalRatePerSec float64
	// CancellationRate is fraction of queued orders cancelled per second.
	CancellationRate float64
}

func NewQueuePositionEngine() *QueuePositionEngine {
	return &QueuePositionEngine{
		ArrivalRatePerSec: 0.5,   // ~30 orders/min at best level
		CancellationRate:  0.003, // 0.3% of queue cancelled per second
	}
}

// ClassifyOrder determines maker vs taker based on order type and price relative to book.
func (q *QueuePositionEngine) ClassifyOrder(order QueueOrder, book *OrderBook) MakerTakerClass {
	if order.Type == "MARKET" {
		return ClassTaker
	}
	// Limit orders that would immediately cross the spread are takers.
	if order.Side == "BUY" && order.LimitPrice >= book.BestAsk().Price {
		return ClassTaker
	}
	if order.Side == "SELL" && order.LimitPrice <= book.BestBid().Price {
		return ClassTaker
	}
	return ClassMaker
}

// Estimate computes fill probability and expected time for a queued limit order.
func (q *QueuePositionEngine) Estimate(order QueueOrder, book *OrderBook, tradeVolumePerSec float64) QueueFillResult {
	class := q.ClassifyOrder(order, book)

	if class == ClassTaker {
		// Taker orders fill immediately off the book.
		result := QueueFillResult{
			Class:             ClassTaker,
			QueueRank:         0,
			FillProbability:   1.0,
			ExpectedFillTime:  2 * time.Millisecond,
			ExpectedFillRatio: 1.0,
		}
		result.PartialFills = q.simulateInstantFill(order)
		return result
	}

	// For maker orders, estimate queue depth ahead and fill probability.
	queueDepthUSD := q.estimateQueueDepthAhead(order, book)
	queueRank := int(queueDepthUSD / (order.Quantity * order.LimitPrice))
	if queueRank < 0 {
		queueRank = 0
	}

	// Time to fill: queue clears at tradeVolumePerSec USD/sec.
	if tradeVolumePerSec <= 0 {
		tradeVolumePerSec = order.LimitPrice * 2 // assume 2 BTC/sec turnover
	}
	secondsToFill := queueDepthUSD / tradeVolumePerSec
	expectedFillTime := time.Duration(secondsToFill * float64(time.Second))

	// Fill probability: adjusted for cancel risk and time decay.
	fillProb := q.fillProbability(queueDepthUSD, order.Quantity*order.LimitPrice, secondsToFill, book)

	// Expected ratio: scaled by fill probability.
	expectedRatio := fillProb
	if expectedRatio < 0.25 {
		expectedRatio = 0.25 // minimum 25% if we stay in queue
	}

	partials := q.simulateMakerFills(order, fillProb, expectedFillTime)

	return QueueFillResult{
		Class:               ClassMaker,
		QueueRank:           queueRank,
		EstimatedQueueDepth: queueDepthUSD,
		FillProbability:     fillProb,
		ExpectedFillTime:    expectedFillTime,
		ExpectedFillRatio:   expectedRatio,
		PartialFills:        partials,
	}
}

// estimateQueueDepthAhead estimates USD value of orders ahead in the queue at this price level.
// Uses pro-rata model: new orders arrive at the back; existing depth at price level is ahead.
func (q *QueuePositionEngine) estimateQueueDepthAhead(order QueueOrder, book *OrderBook) float64 {
	offsetBps := 0.0
	if book.Mid > 0 && order.LimitPrice > 0 {
		if order.Side == "BUY" {
			offsetBps = (book.Mid - order.LimitPrice) / book.Mid * 10_000
		} else {
			offsetBps = (order.LimitPrice - book.Mid) / book.Mid * 10_000
		}
	}
	offsetBps = math.Abs(offsetBps)

	// At best price, all current depth is ahead. At 5bps away, smaller queue.
	depthAtLevel := book.DepthAtPrice(offsetBps+1, order.Side == "BUY")
	queueFraction := math.Exp(-offsetBps * 0.2) // orders deeper in book have less competition
	return depthAtLevel * queueFraction
}

// fillProbability computes the probability of a maker order filling
// given queue depth, order size, and time available.
func (q *QueuePositionEngine) fillProbability(queueDepthUSD, orderSizeUSD, secondsToFill float64, book *OrderBook) float64 {
	// Base probability from liquidity score.
	liq := book.LiquidityScore()
	baseProbability := liq * 0.7 // even in liquid markets, ~70% of maker orders fill

	// Reduce probability for deep queue positions.
	queuePenalty := math.Exp(-queueDepthUSD / (orderSizeUSD * 10))

	// Cancel risk degrades probability over time.
	cancelPenalty := math.Exp(-q.CancellationRate * secondsToFill)

	// Distance from mid reduces probability (orders far from mid rarely fill).
	prob := baseProbability * queuePenalty * cancelPenalty
	return clampF(prob, 0.05, 0.98)
}

// simulateInstantFill generates partial fill events for a taker order.
func (q *QueuePositionEngine) simulateInstantFill(order QueueOrder) []OrderPartialFillEvent {
	now := order.SubmittedAt
	if now.IsZero() {
		now = time.Now()
	}
	return []OrderPartialFillEvent{
		{
			OrderID:           order.OrderID,
			FilledQuantity:    order.Quantity,
			RemainingQuantity: 0,
			AverageFillPrice:  order.LimitPrice,
			FillNumber:        1,
			Timestamp:         now.Add(2 * time.Millisecond),
		},
	}
}

// simulateMakerFills generates a sequence of partial fill events for a maker order.
// Models the reality that large maker orders often fill in 2–4 chunks.
func (q *QueuePositionEngine) simulateMakerFills(order QueueOrder, fillProb float64, expectedTime time.Duration) []OrderPartialFillEvent {
	if fillProb < 0.3 {
		// Very low probability — likely zero or one small fill.
		if fillProb < 0.15 {
			return nil
		}
		return []OrderPartialFillEvent{{
			OrderID:           order.OrderID,
			FilledQuantity:    order.Quantity * 0.25,
			RemainingQuantity: order.Quantity * 0.75,
			AverageFillPrice:  order.LimitPrice,
			FillNumber:        1,
			Timestamp:         order.SubmittedAt.Add(expectedTime / 4),
		}}
	}

	// Model 2–3 partial fills converging to fillProb × quantity.
	totalFill := order.Quantity * fillProb
	events := make([]OrderPartialFillEvent, 0, 3)
	chunks := []float64{0.4, 0.35, 0.25}
	remaining := order.Quantity
	cumFilled := 0.0
	timeStep := expectedTime / 3

	for i, frac := range chunks {
		chunkQty := totalFill * frac
		if chunkQty <= 0 || cumFilled >= totalFill {
			break
		}
		cumFilled += chunkQty
		remaining -= chunkQty
		if remaining < 0 {
			remaining = 0
		}
		events = append(events, OrderPartialFillEvent{
			OrderID:           order.OrderID,
			FilledQuantity:    chunkQty,
			RemainingQuantity: remaining,
			AverageFillPrice:  order.LimitPrice,
			FillNumber:        i + 1,
			Timestamp:         order.SubmittedAt.Add(time.Duration(i+1) * timeStep),
		})
	}
	return events
}
