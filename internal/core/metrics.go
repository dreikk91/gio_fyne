package core

import (
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

const receivedWindowSize = 60

type Metrics struct {
	accepted   atomic.Int64
	rejected   atomic.Int64
	reconnects atomic.Int64
	connected  atomic.Int32
	clients    atomic.Int64
	started    time.Time

	receivedMu    sync.Mutex
	receivedSec   int64
	receivedTotal int64
	receivedRing  [receivedWindowSize]int64
}

func NewMetrics() *Metrics {
	return &Metrics{started: time.Now()}
}

func (m *Metrics) IncAccepted()   { m.accepted.Add(1) }
func (m *Metrics) IncRejected()   { m.rejected.Add(1) }
func (m *Metrics) IncReconnects() { m.reconnects.Add(1) }
func (m *Metrics) IncReceived() {
	nowSec := time.Now().Unix()
	m.receivedMu.Lock()
	defer m.receivedMu.Unlock()

	if m.receivedSec == 0 {
		m.receivedSec = nowSec
	}
	m.advanceReceivedWindowLocked(nowSec)
	idx := int(nowSec % receivedWindowSize)
	m.receivedRing[idx]++
	m.receivedTotal++
}
func (m *Metrics) SetConnected(v bool) {
	if v {
		m.connected.Store(1)
		return
	}
	m.connected.Store(0)
}
func (m *Metrics) SetClients(v int) { m.clients.Store(int64(v)) }

func (m *Metrics) Snapshot() StatsDTO {
	uptime := time.Since(m.started)
	receivedPS, receivedPM := m.receivedRates()
	return StatsDTO{
		Accepted:   m.accepted.Load(),
		Rejected:   m.rejected.Load(),
		Reconnects: m.reconnects.Load(),
		ReceivedPS: receivedPS,
		ReceivedPM: receivedPM,
		Clients:    int(m.clients.Load()),
		Uptime:     formatUptime(uptime),
		Connected:  m.connected.Load() == 1,
	}
}

func (m *Metrics) advanceReceivedWindowLocked(nowSec int64) {
	if m.receivedSec == 0 {
		return
	}
	if nowSec <= m.receivedSec {
		return
	}
	gap := nowSec - m.receivedSec
	if gap >= receivedWindowSize {
		for i := range m.receivedRing {
			m.receivedRing[i] = 0
		}
		m.receivedTotal = 0
		m.receivedSec = nowSec
		return
	}
	for step := int64(1); step <= gap; step++ {
		idx := int((m.receivedSec + step) % receivedWindowSize)
		m.receivedTotal -= m.receivedRing[idx]
		m.receivedRing[idx] = 0
	}
	m.receivedSec = nowSec
}

func (m *Metrics) receivedRates() (perSecond int64, perMinute int64) {
	nowSec := time.Now().Unix()
	m.receivedMu.Lock()
	defer m.receivedMu.Unlock()

	if m.receivedSec == 0 {
		return 0, 0
	}
	m.advanceReceivedWindowLocked(nowSec)
	return m.receivedTotal / receivedWindowSize, m.receivedTotal
}

func formatUptime(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	return two(h) + ":" + two(m) + ":" + two(s)
}

func two(v int) string {
	if v < 10 {
		return "0" + strconv.Itoa(v)
	}
	return strconv.Itoa(v)
}
