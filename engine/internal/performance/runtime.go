package performance

import (
	"runtime"
	"time"
)

type RuntimeSnapshot struct {
	CapturedAt     time.Time
	Goroutines     int
	HeapAllocBytes uint64
	HeapInUseBytes uint64
	NumGC          uint32
	LastGCPauseNs  uint64
	TotalGCPauseNs uint64
}

func CaptureRuntime() RuntimeSnapshot {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	pause := uint64(0)
	if mem.NumGC > 0 {
		pause = mem.PauseNs[(mem.NumGC+255)%256]
	}
	return RuntimeSnapshot{
		CapturedAt:     time.Now(),
		Goroutines:     runtime.NumGoroutine(),
		HeapAllocBytes: mem.HeapAlloc,
		HeapInUseBytes: mem.HeapInuse,
		NumGC:          mem.NumGC,
		LastGCPauseNs:  pause,
		TotalGCPauseNs: mem.PauseTotalNs,
	}
}
