package core

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"
)

// OverflowPolicy defines what to do when the queue is full
type OverflowPolicy string

const (
	OverflowDropOldest OverflowPolicy = "drop_oldest"
	OverflowDropNewest OverflowPolicy = "drop_newest"
	OverflowRetry      OverflowPolicy = "retry"
)

// RateMode defines how rate control works
type RateMode string

const (
	RateModeInterval RateMode = "interval" // Fixed interval between messages
	RateModeRate     RateMode = "rate"     // Messages per second
)

// EngineConfig defines the engine configuration
type EngineConfig struct {
	RateMode        RateMode       `yaml:"rate_mode" json:"rate_mode"` // "interval" or "rate"
	IntervalMs      int            `yaml:"interval_ms,omitempty" json:"interval_ms,omitempty"` // For interval mode
	Rate            float64        `yaml:"rate,omitempty" json:"rate,omitempty"` // Messages per second for rate mode
	JitterPercent   float64        `yaml:"jitter_percent,omitempty" json:"jitter_percent,omitempty"` // ±% jitter
	QueueSize       int            `yaml:"queue_size" json:"queue_size"`
	OverflowPolicy  OverflowPolicy `yaml:"overflow_policy" json:"overflow_policy"`
	RetryCount      int            `yaml:"retry_count,omitempty" json:"retry_count,omitempty"` // For retry policy
	MetricsInterval time.Duration  `yaml:"metrics_interval,omitempty" json:"metrics_interval,omitempty"` // How often to log metrics
}

// Engine orchestrates the generator and adapter
type Engine struct {
	generator Generator
	adapter   Adapter
	config    EngineConfig
	metrics   *Metrics
	queue     chan TelemetryMessage
	wg        sync.WaitGroup
	ctx       context.Context
	cancel    context.CancelFunc
}

// NewEngine creates a new engine instance
func NewEngine(generator Generator, adapter Adapter, config EngineConfig) *Engine {
	ctx, cancel := context.WithCancel(context.Background())
	
	queueSize := config.QueueSize
	if queueSize <= 0 {
		queueSize = 1000
	}

	return &Engine{
		generator: generator,
		adapter:   adapter,
		config:    config,
		metrics:   NewMetrics(),
		queue:     make(chan TelemetryMessage, queueSize),
		ctx:       ctx,
		cancel:    cancel,
	}
}

// Run starts the engine and runs until context is cancelled
func (e *Engine) Run(ctx context.Context) error {
	// Connect adapter with retry
	var connectErr error
	maxConnectRetries := 5
	for i := 0; i < maxConnectRetries; i++ {
		connectErr = e.adapter.Connect(ctx)
		if connectErr == nil {
			break
		}
		if i < maxConnectRetries-1 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(i+1) * time.Second):
			}
		}
	}
	if connectErr != nil {
		return fmt.Errorf("failed to connect adapter after %d retries: %w", maxConnectRetries, connectErr)
	}

	// Start generator goroutine
	e.wg.Add(1)
	go e.generatorLoop(ctx)

	// Start publisher goroutine
	e.wg.Add(1)
	go e.publisherLoop(ctx)

	// Start metrics reporter if configured
	if e.config.MetricsInterval > 0 {
		e.wg.Add(1)
		go e.metricsReporter(ctx)
	}

	// Wait for context cancellation
	<-ctx.Done()

	// Stop gracefully
	e.stop()

	return nil
}

// generatorLoop generates messages and puts them in the queue
func (e *Engine) generatorLoop(ctx context.Context) {
	defer e.wg.Done()

	var interval time.Duration
	if e.config.RateMode == RateModeInterval {
		interval = time.Duration(e.config.IntervalMs) * time.Millisecond
	} else if e.config.RateMode == RateModeRate {
		if e.config.Rate > 0 {
			interval = time.Duration(float64(time.Second) / e.config.Rate)
		} else {
			interval = time.Second
		}
	} else {
		interval = time.Second // Default
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			msg, err := e.generator.Next(ctx)
			if err != nil {
				e.metrics.RecordFailed(err)
				continue
			}

			// Apply jitter
			if e.config.JitterPercent > 0 {
				jitter := float64(interval) * e.config.JitterPercent / 100.0
				jitterMs := time.Duration(jitter * (randFloat()*2 - 1)) // ±jitter
				time.Sleep(jitterMs)
			}

			// Try to enqueue
			select {
			case e.queue <- msg:
				e.metrics.SetQueueLength(int64(len(e.queue)))
			default:
				// Queue is full
				e.handleOverflow(msg)
			}
		}
	}
}

// publisherLoop consumes messages from the queue and publishes them
func (e *Engine) publisherLoop(ctx context.Context) {
	defer e.wg.Done()

	for {
		select {
		case <-ctx.Done():
			// Drain queue
			for {
				select {
				case msg := <-e.queue:
					e.publishMessage(ctx, msg)
				default:
					return
				}
			}
		case msg := <-e.queue:
			e.metrics.SetQueueLength(int64(len(e.queue)))
			e.publishMessage(ctx, msg)
		}
	}
}

// publishMessage publishes a single message with retry logic
func (e *Engine) publishMessage(ctx context.Context, msg TelemetryMessage) {
	retries := 0
	maxRetries := e.config.RetryCount
	if maxRetries <= 0 {
		maxRetries = 3 // Default
	}

	for {
		err := e.adapter.Publish(ctx, msg)
		if err == nil {
			e.metrics.RecordSent()
			return
		}

		e.metrics.RecordFailed(err)
		retries++

		if retries >= maxRetries {
			return
		}

		// Exponential backoff
		backoff := time.Duration(math.Pow(2, float64(retries))) * 100 * time.Millisecond
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
	}
}

// handleOverflow handles queue overflow based on policy
func (e *Engine) handleOverflow(msg TelemetryMessage) {
	switch e.config.OverflowPolicy {
	case OverflowDropOldest:
		// Remove oldest message
		select {
		case <-e.queue:
			// Try to add new message
			select {
			case e.queue <- msg:
			default:
				// Still full, drop
			}
		default:
		}
	case OverflowDropNewest:
		// Drop the new message (do nothing)
		e.metrics.RecordFailed(fmt.Errorf("queue full, dropping message"))
	case OverflowRetry:
		// Retry enqueueing
		select {
		case e.queue <- msg:
		case <-time.After(100 * time.Millisecond):
			e.metrics.RecordFailed(fmt.Errorf("queue full, retry timeout"))
		}
	}
}

// metricsReporter periodically logs metrics
func (e *Engine) metricsReporter(ctx context.Context) {
	defer e.wg.Done()

	ticker := time.NewTicker(e.config.MetricsInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			snapshot := e.metrics.GetSnapshot()
			fmt.Printf("[METRICS] sent=%d failed=%d reconnect=%d rate=%.2f msg/s queue=%d duration=%.1fs\n",
				snapshot.SentTotal,
				snapshot.FailedTotal,
				snapshot.ReconnectTotal,
				snapshot.CurrentRate,
				snapshot.QueueLength,
				snapshot.RunDuration)
		}
	}
}

// stop stops the engine gracefully
func (e *Engine) stop() {
	e.cancel()
	close(e.queue)
	e.wg.Wait()
	
	// Close adapter
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	e.adapter.Close(ctx)
}

// GetMetrics returns the current metrics snapshot
func (e *Engine) GetMetrics() MetricsSnapshot {
	return e.metrics.GetSnapshot()
}

// randFloat returns a random float64 between 0 and 1
func randFloat() float64 {
	return rand.Float64()
}

