package core

import (
	"sync"
	"sync/atomic"
	"time"
)

// Metrics tracks simulator metrics
type Metrics struct {
	sentTotal      int64
	failedTotal    int64
	reconnectTotal int64
	queueLen       int64
	lastError      string
	lastErrorMutex sync.RWMutex
	startTime      time.Time
	lastSentTime   time.Time
	lastSentMutex  sync.RWMutex
}

// NewMetrics creates a new metrics instance
func NewMetrics() *Metrics {
	return &Metrics{
		startTime: time.Now(),
	}
}

// RecordSent increments the sent counter
func (m *Metrics) RecordSent() {
	atomic.AddInt64(&m.sentTotal, 1)
	m.lastSentMutex.Lock()
	m.lastSentTime = time.Now()
	m.lastSentMutex.Unlock()
}

// RecordFailed increments the failed counter and records error
func (m *Metrics) RecordFailed(err error) {
	atomic.AddInt64(&m.failedTotal, 1)
	if err != nil {
		m.lastErrorMutex.Lock()
		m.lastError = err.Error()
		m.lastErrorMutex.Unlock()
	}
}

// RecordReconnect increments the reconnect counter
func (m *Metrics) RecordReconnect() {
	atomic.AddInt64(&m.reconnectTotal, 1)
}

// SetQueueLength sets the current queue length
func (m *Metrics) SetQueueLength(length int64) {
	atomic.StoreInt64(&m.queueLen, length)
}

// GetSnapshot returns a snapshot of current metrics
func (m *Metrics) GetSnapshot() MetricsSnapshot {
	m.lastSentMutex.RLock()
	lastSent := m.lastSentTime
	m.lastSentMutex.RUnlock()

	m.lastErrorMutex.RLock()
	lastError := m.lastError
	m.lastErrorMutex.RUnlock()

	elapsed := time.Since(m.startTime).Seconds()
	sentTotal := atomic.LoadInt64(&m.sentTotal)
	
	var currentRate float64
	if elapsed > 0 && sentTotal > 0 {
		currentRate = float64(sentTotal) / elapsed
	}

	return MetricsSnapshot{
		SentTotal:      atomic.LoadInt64(&m.sentTotal),
		FailedTotal:     atomic.LoadInt64(&m.failedTotal),
		ReconnectTotal:  atomic.LoadInt64(&m.reconnectTotal),
		CurrentRate:     currentRate,
		QueueLength:     atomic.LoadInt64(&m.queueLen),
		LastError:       lastError,
		RunDuration:     elapsed,
		LastSentTime:    lastSent,
	}
}

// MetricsSnapshot represents a snapshot of metrics at a point in time
type MetricsSnapshot struct {
	SentTotal     int64
	FailedTotal   int64
	ReconnectTotal int64
	CurrentRate   float64 // messages per second
	QueueLength   int64
	LastError     string
	RunDuration   float64 // seconds
	LastSentTime  time.Time
}

