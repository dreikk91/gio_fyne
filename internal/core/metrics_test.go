package core

import (
	"testing"
	"time"
)

func TestMetricsReceivedRateUsesRollingWindow(t *testing.T) {
	m := NewMetrics()

	m.receivedMu.Lock()
	nowSec := time.Now().Unix()
	m.receivedSec = nowSec
	m.receivedRing = [receivedWindowSize]int64{10, 0, 0, 0, 0}
	m.receivedTotal = 10
	m.receivedMu.Unlock()

	if got := m.receivedRate(); got != 2 {
		t.Fatalf("receivedRate() = %d, want 2", got)
	}
}
