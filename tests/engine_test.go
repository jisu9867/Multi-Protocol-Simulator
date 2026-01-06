package tests

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/smartfactory/simulator/internal/core"
)

// MockAdapter is a test adapter that records published messages
type MockAdapter struct {
	name           string
	connected      bool
	publishedCount int64
	messages       []core.TelemetryMessage
	shouldFail     bool
}

func NewMockAdapter(name string) *MockAdapter {
	return &MockAdapter{
		name:      name,
		messages:  make([]core.TelemetryMessage, 0),
		shouldFail: false,
	}
}

func (m *MockAdapter) Name() string {
	return m.name
}

func (m *MockAdapter) Connect(ctx context.Context) error {
	m.connected = true
	return nil
}

func (m *MockAdapter) Publish(ctx context.Context, msg core.TelemetryMessage) error {
	if m.shouldFail {
		return &MockError{message: "mock publish error"}
	}
	atomic.AddInt64(&m.publishedCount, 1)
	m.messages = append(m.messages, msg)
	return nil
}

func (m *MockAdapter) Close(ctx context.Context) error {
	m.connected = false
	return nil
}

func (m *MockAdapter) GetPublishedCount() int64 {
	return atomic.LoadInt64(&m.publishedCount)
}

type MockError struct {
	message string
}

func (e *MockError) Error() string {
	return e.message
}

func TestEngineRateControlInterval(t *testing.T) {
	generatorConfig := core.GeneratorConfig{
		SourceID: "test-engine",
		Tags: []core.TagConfig{
			{
				Tag:     "temp",
				Pattern: core.PatternUniform,
				Min:     20.0,
				Max:     30.0,
			},
		},
	}

	generator := core.NewGenerator(generatorConfig)
	adapter := NewMockAdapter("mock")

	engineConfig := core.EngineConfig{
		RateMode:       core.RateModeInterval,
		IntervalMs:     100, // 100ms = 10 msg/s
		QueueSize:      100,
		OverflowPolicy: core.OverflowDropOldest,
		RetryCount:     3,
	}

	engine := core.NewEngine(generator, adapter, engineConfig)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// Run engine for 1 second
	go func() {
		_ = engine.Run(ctx)
	}()

	// Wait for context timeout
	<-ctx.Done()

	// Give a bit of time for messages to be processed
	time.Sleep(200 * time.Millisecond)

	publishedCount := adapter.GetPublishedCount()

	// Should have published approximately 10 messages (1 second / 100ms)
	// Allow some tolerance
	if publishedCount < 5 || publishedCount > 15 {
		t.Errorf("Expected approximately 10 messages, got %d", publishedCount)
	}
}

func TestEngineQueueOverflowDropOldest(t *testing.T) {
	generatorConfig := core.GeneratorConfig{
		SourceID: "test-overflow",
		Tags: []core.TagConfig{
			{
				Tag:     "temp",
				Pattern: core.PatternUniform,
				Min:     20.0,
				Max:     30.0,
			},
		},
	}

	generator := core.NewGenerator(generatorConfig)
	adapter := NewMockAdapter("mock")

	// Make adapter slow to cause queue overflow
	adapter.shouldFail = false

	engineConfig := core.EngineConfig{
		RateMode:       core.RateModeInterval,
		IntervalMs:     10, // Very fast generation
		QueueSize:      5,  // Small queue
		OverflowPolicy: core.OverflowDropOldest,
		RetryCount:     1,
	}

	engine := core.NewEngine(generator, adapter, engineConfig)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	go func() {
		_ = engine.Run(ctx)
	}()

	<-ctx.Done()
	time.Sleep(100 * time.Millisecond)

	// Queue should have overflowed, but engine should continue running
	snapshot := engine.GetMetrics()
	if snapshot.FailedTotal > snapshot.SentTotal*2 {
		t.Errorf("Too many failures: %d failures vs %d sent", snapshot.FailedTotal, snapshot.SentTotal)
	}
}

func TestEngineMetrics(t *testing.T) {
	generatorConfig := core.GeneratorConfig{
		SourceID: "test-metrics",
		Tags: []core.TagConfig{
			{
				Tag:     "temp",
				Pattern: core.PatternUniform,
				Min:     20.0,
				Max:     30.0,
			},
		},
	}

	generator := core.NewGenerator(generatorConfig)
	adapter := NewMockAdapter("mock")

	engineConfig := core.EngineConfig{
		RateMode:       core.RateModeInterval,
		IntervalMs:     50,
		QueueSize:      100,
		OverflowPolicy: core.OverflowDropOldest,
		RetryCount:     3,
	}

	engine := core.NewEngine(generator, adapter, engineConfig)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	go func() {
		_ = engine.Run(ctx)
	}()

	<-ctx.Done()
	time.Sleep(100 * time.Millisecond)

	snapshot := engine.GetMetrics()

	if snapshot.SentTotal == 0 {
		t.Error("Expected at least one sent message")
	}

	if snapshot.RunDuration <= 0 {
		t.Error("Run duration should be positive")
	}

	if snapshot.CurrentRate <= 0 {
		t.Error("Current rate should be positive")
	}
}

