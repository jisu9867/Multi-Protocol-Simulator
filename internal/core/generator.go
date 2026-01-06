package core

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"
)

// GeneratorPattern defines the pattern type for value generation
type GeneratorPattern string

const (
	PatternUniform   GeneratorPattern = "uniform"   // Uniform random distribution
	PatternNormal    GeneratorPattern = "normal"     // Normal (Gaussian) distribution
	PatternSine      GeneratorPattern = "sine"       // Sine wave
	PatternStep      GeneratorPattern = "step"       // Step function
	PatternRandomWalk GeneratorPattern = "randomwalk" // Random walk
)

// TagConfig defines configuration for a specific tag
type TagConfig struct {
	Tag      string          `yaml:"tag" json:"tag"`
	Pattern  GeneratorPattern `yaml:"pattern" json:"pattern"`
	Min      float64         `yaml:"min" json:"min"`
	Max      float64         `yaml:"max" json:"max"`
	Mean     float64         `yaml:"mean,omitempty" json:"mean,omitempty"`     // For normal distribution
	StdDev   float64         `yaml:"stddev,omitempty" json:"stddev,omitempty"` // For normal distribution
	Unit     string          `yaml:"unit,omitempty" json:"unit,omitempty"`
	Quality  string          `yaml:"quality,omitempty" json:"quality,omitempty"`
	StepSize float64         `yaml:"step_size,omitempty" json:"step_size,omitempty"` // For random walk
}

// GeneratorConfig defines the overall generator configuration
type GeneratorConfig struct {
	SourceID   string      `yaml:"source_id" json:"source_id"`
	Tags       []TagConfig `yaml:"tags" json:"tags"`
	TraceID    string      `yaml:"trace_id,omitempty" json:"trace_id,omitempty"`
	Seed       int64       `yaml:"seed,omitempty" json:"seed,omitempty"` // Random seed (0 = use time)
}

// Generator interface for generating telemetry messages
type Generator interface {
	Next(ctx context.Context) (TelemetryMessage, error)
}

// DefaultGenerator implements the Generator interface
type DefaultGenerator struct {
	config     GeneratorConfig
	rng        *rand.Rand
	sequences  map[string]int64 // key: source_id:tag
	seqMutex   sync.Mutex
	startTime  time.Time
	tagConfigs map[string]TagConfig
	state      map[string]float64 // For patterns that need state (sine, random walk)
	stateMutex sync.Mutex
}

// NewGenerator creates a new generator instance
func NewGenerator(config GeneratorConfig) Generator {
	var rng *rand.Rand
	if config.Seed == 0 {
		rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	} else {
		rng = rand.New(rand.NewSource(config.Seed))
	}

	tagConfigs := make(map[string]TagConfig)
	for _, tc := range config.Tags {
		tagConfigs[tc.Tag] = tc
	}

	return &DefaultGenerator{
		config:     config,
		rng:        rng,
		sequences:  make(map[string]int64),
		startTime:  time.Now(),
		tagConfigs: tagConfigs,
		state:      make(map[string]float64),
	}
}

// Next generates the next telemetry message
func (g *DefaultGenerator) Next(ctx context.Context) (TelemetryMessage, error) {
	if len(g.config.Tags) == 0 {
		return TelemetryMessage{}, fmt.Errorf("no tags configured")
	}

	// Select a random tag
	tagIdx := g.rng.Intn(len(g.config.Tags))
	tagConfig := g.config.Tags[tagIdx]

	// Get next sequence number for this source_id:tag
	seqKey := fmt.Sprintf("%s:%s", g.config.SourceID, tagConfig.Tag)
	g.seqMutex.Lock()
	g.sequences[seqKey]++
	seq := g.sequences[seqKey]
	g.seqMutex.Unlock()

	// Generate value based on pattern
	value := g.generateValue(tagConfig)

	// Determine quality
	quality := tagConfig.Quality
	if quality == "" {
		quality = QualityGood
	}

	// Generate trace ID if configured
	traceID := g.config.TraceID
	if traceID == "" {
		traceID = generateTraceID()
	}

	return TelemetryMessage{
		TS:       time.Now(),
		SourceID: g.config.SourceID,
		Tag:      tagConfig.Tag,
		Value:    value,
		Unit:     tagConfig.Unit,
		Quality:  quality,
		Seq:      seq,
		TraceID:  traceID,
	}, nil
}

// generateValue generates a value based on the tag's pattern configuration
func (g *DefaultGenerator) generateValue(tc TagConfig) interface{} {
	g.stateMutex.Lock()
	defer g.stateMutex.Unlock()

	switch tc.Pattern {
	case PatternUniform:
		return g.rng.Float64()*(tc.Max-tc.Min) + tc.Min

	case PatternNormal:
		mean := tc.Mean
		if mean == 0 {
			mean = (tc.Min + tc.Max) / 2
		}
		stddev := tc.StdDev
		if stddev == 0 {
			stddev = (tc.Max - tc.Min) / 6 // 3-sigma rule
		}
		value := g.rng.NormFloat64()*stddev + mean
		// Clamp to min/max
		if value < tc.Min {
			value = tc.Min
		}
		if value > tc.Max {
			value = tc.Max
		}
		return value

	case PatternSine:
		// Initialize state if needed
		if _, exists := g.state[tc.Tag]; !exists {
			g.state[tc.Tag] = (tc.Min + tc.Max) / 2
		}
		elapsed := time.Since(g.startTime).Seconds()
		period := 60.0 // 60 second period
		amplitude := (tc.Max - tc.Min) / 2
		center := (tc.Min + tc.Max) / 2
		value := center + amplitude*math.Sin(2*math.Pi*elapsed/period)
		return value

	case PatternStep:
		// Initialize state if needed
		if _, exists := g.state[tc.Tag]; !exists {
			g.state[tc.Tag] = tc.Min
		}
		// Toggle between min and max every 10 seconds
		elapsed := int(time.Since(g.startTime).Seconds())
		if elapsed%20 < 10 {
			g.state[tc.Tag] = tc.Min
		} else {
			g.state[tc.Tag] = tc.Max
		}
		return g.state[tc.Tag]

	case PatternRandomWalk:
		// Initialize state if needed
		if _, exists := g.state[tc.Tag]; !exists {
			g.state[tc.Tag] = (tc.Min + tc.Max) / 2
		}
		stepSize := tc.StepSize
		if stepSize == 0 {
			stepSize = (tc.Max - tc.Min) / 100
		}
		// Random walk: add random step
		step := (g.rng.Float64() - 0.5) * 2 * stepSize
		g.state[tc.Tag] += step
		// Clamp to bounds
		if g.state[tc.Tag] < tc.Min {
			g.state[tc.Tag] = tc.Min
		}
		if g.state[tc.Tag] > tc.Max {
			g.state[tc.Tag] = tc.Max
		}
		return g.state[tc.Tag]

	default:
		// Default to uniform
		return g.rng.Float64()*(tc.Max-tc.Min) + tc.Min
	}
}

// generateTraceID generates a simple trace ID
func generateTraceID() string {
	return fmt.Sprintf("%016x", rand.Int63())
}

