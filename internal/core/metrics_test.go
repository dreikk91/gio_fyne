package core

import (
	"testing"
	"time"
)

func TestMetricsReceivedRatesUseRollingWindow(t *testing.T) {
	m := NewMetrics()

	m.receivedMu.Lock()
	nowSec := time.Now().Unix()
	m.receivedSec = nowSec
	m.receivedRing = [receivedWindowSize]int64{120}
	m.receivedTotal = 120
	m.receivedMu.Unlock()

	ps, pm := m.receivedRates()
	if ps != 2 {
		t.Fatalf("receivedRates() perSecond = %d, want 2", ps)
	}
	if pm != 120 {
		t.Fatalf("receivedRates() perMinute = %d, want 120", pm)
	}
}
