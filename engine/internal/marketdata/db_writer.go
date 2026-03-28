package marketdata

import (
	"context"
	"log"
	"time"
)

type DBWriter struct {
	tickChan <-chan Tick
	dbURL    string
}

func NewDBWriter(dbURL string, ch <-chan Tick) *DBWriter {
	return &DBWriter{
		tickChan: ch,
		dbURL:    dbURL,
	}
}

func (d *DBWriter) Start(ctx context.Context) {
	// In a real implementation using jackc/pgx/v5:
	// conn, _ := pgxpool.New(ctx, d.dbURL)
	// defer conn.Close()

	log.Println("DBWriter started, waiting for ticks...")

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	var batch []Tick

	for {
		select {
		case <-ctx.Done():
			return
		case tick := <-d.tickChan:
			batch = append(batch, tick)
			// Print live output to console just so user sees something!
			log.Printf("[LIVE TRADE] %s | Side: %s | Price: %.2f | Qty: %.5f", tick.Symbol, tick.Side, tick.Price, tick.Quantity)

		case <-ticker.C:
			if len(batch) > 0 {
				log.Printf("--> Flushed %d ticks to TimescaleDB (MOCK)", len(batch))
				// pgx batch insert logic goes here
				batch = nil // Reset batch
			}
		}
	}
}
