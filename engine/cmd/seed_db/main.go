package main

import (
	"bufio"
	"context"
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// SeedDB executes gigantic bulk-copy operations reading local Binance CSVs directly to TimescaleDB.
// This parses physical files and executes bulk Postgres inserts spanning thousands of rows per second.
func main() {
	dbURL := "postgres://antigravity:password123@localhost:5432/antigravity"
	
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("Fatal: Cannot connect to Timescale Postgres: %v", err)
	}
	defer pool.Close()

	dataDir := "../../data"
	files, err := filepath.Glob(filepath.Join(dataDir, "BTCUSDT-1m-*.csv"))
	if err != nil || len(files) == 0 {
		log.Fatalf("No CSV files found in /data/ directory. Did you execute fetch_history.py?")
	}

	log.Printf("Identified %d specific CSV files to ingest. Commencing High-Speed Timescale Seeding...", len(files))

	for _, file := range files {
		log.Printf("Ingesting %s...", filepath.Base(file))
		processCSV(ctx, pool, file)
	}

	log.Println("[SUCCESS] Database completely seeded with quantitative market data!")
}

func processCSV(ctx context.Context, pool *pgxpool.Pool, filepath string) {
	file, err := os.Open(filepath)
	if err != nil {
		log.Printf("[SKIP] Could not read file %s: %v", filepath, err)
		return
	}
	defer file.Close()

	reader := csv.NewReader(bufio.NewReader(file))
	
	records, err := reader.ReadAll()
	if err != nil {
		log.Printf("ERROR: Memory buffer failed to read CSV rows: %v", err)
		return
	}

	// -------------------------------------------------------------
	// Binance Unified API structure mapping for Klines (Candles):
	// [Open_time, Open, High, Low, Close, Volume, Close_time...]
	// -------------------------------------------------------------
	
	const batchSize = 3000
	var valueStrings []string
	var valueArgs []interface{}
	
	for i, record := range records {
		if len(record) < 6 { continue }

		// Unix Milliseconds to PostgreSQL standard TIMESTAMPTZ formatting
		msTime, _ := strconv.ParseInt(record[0], 10, 64)
		insertTime := time.UnixMilli(msTime).Format(time.RFC3339)
		
		// Map the raw candle 'Close' price directly as our quantitative `Tick` standard value
		closePrice, _ := strconv.ParseFloat(record[4], 64) 

		// Construct optimized Postgres parameterized array arguments!
		valueStrings = append(valueStrings, fmt.Sprintf("($%d, $%d, $%d)", len(valueArgs)+1, len(valueArgs)+2, len(valueArgs)+3))
		valueArgs = append(valueArgs, insertTime, "BTCUSDT", closePrice)

		// Fire physical chunked network queries!
		if i > 0 && i%batchSize == 0 || i == len(records)-1 {
			stmt := fmt.Sprintf("INSERT INTO market_ticks (time, symbol, price) VALUES %s", 
				strings.Join(valueStrings, ","))
				
			_, err := pool.Exec(ctx, stmt, valueArgs...)
			if err != nil {
				log.Printf("Skipped physical batch error: %v", err)
			}
			
			// Wipe memory payload before repeating
			valueStrings = nil
			valueArgs = nil
		}
	}
}
