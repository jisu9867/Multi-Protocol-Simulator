package core

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"regexp"
	"strconv"
	"strings"
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
	SourceID      string      `yaml:"source_id" json:"source_id"`
	FactoryID     string      `yaml:"factory_id,omitempty" json:"factory_id,omitempty"`     // Factory ID: Ulsan, Asan, Jeonju, Hwaseong
	EquipmentType string      `yaml:"equipment_type,omitempty" json:"equipment_type,omitempty"` // Equipment type
	EquipmentName string      `yaml:"equipment_name,omitempty" json:"equipment_name,omitempty"` // Equipment name
	Tags          []TagConfig `yaml:"tags" json:"tags"`
	TraceID       string      `yaml:"trace_id,omitempty" json:"trace_id,omitempty"`
	Seed          int64       `yaml:"seed,omitempty" json:"seed,omitempty"` // Random seed (0 = use time)
	RandomFactory bool        `yaml:"random_factory,omitempty" json:"random_factory,omitempty"` // Randomly select factory ID (local only)
	RandomSource  bool        `yaml:"random_source,omitempty" json:"random_source,omitempty"`   // Randomly generate source ID (local only)
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
	// Always use time-based seed to ensure randomness, even if seed is set
	// This ensures different runs produce different sequences
	rng = rand.New(rand.NewSource(time.Now().UnixNano() + int64(time.Now().Nanosecond())))

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

	// Select a random tag - ensure different tags are selected
	tagIdx := g.rng.Intn(len(g.config.Tags))
	tagConfig := g.config.Tags[tagIdx]

	// Set factory_id - random if enabled, otherwise use configured value
	// Ensure different factories are selected each time
	// Use 3 factories: Ulsan, Asan, Jeonju
	var factoryID string
	if g.config.RandomFactory {
		factories := []string{"Ulsan", "Asan", "Jeonju"}
		// Use a fresh random number for factory selection - ensure true randomness
		factoryIdx := g.rng.Intn(len(factories))
		factoryID = factories[factoryIdx]
	} else {
		factoryID = g.config.FactoryID
		if factoryID == "" {
			factoryID = "Ulsan" // Default to Ulsan
		}
	}

	// Get source ID - generate in format "{factory}-line{number}" for SignalR compatibility
	// Ensure different lines are selected each time
	// Use 3 lines: 1, 2, 3
	var sourceID string
	if g.config.RandomSource {
		// Generate random source ID with line number
		// Use 3 lines (1-3) to create variety across different factories and lines
		// Use a fresh random number for line selection - ensure true randomness
		lineNumber := g.rng.Intn(3) + 1 // Random line 1-3
		factoryLower := strings.ToLower(factoryID)
		sourceID = fmt.Sprintf("%s-line%d", factoryLower, lineNumber)
	} else {
		sourceID = g.config.SourceID
		if sourceID == "" {
			// Default: use factory name with line 1
			factoryLower := strings.ToLower(factoryID)
			sourceID = fmt.Sprintf("%s-line1", factoryLower)
		} else {
			// If sourceId doesn't contain line info, try to extract from it or add default line
			if !strings.Contains(strings.ToLower(sourceID), "line") {
				// If sourceId is like "sim-001", convert to "{factory}-line1" format
				factoryLower := strings.ToLower(factoryID)
				sourceID = fmt.Sprintf("%s-line1", factoryLower)
			}
		}
	}

	// Get next sequence number for this source_id:tag
	seqKey := fmt.Sprintf("%s:%s", sourceID, tagConfig.Tag)
	g.seqMutex.Lock()
	g.sequences[seqKey]++
	seq := g.sequences[seqKey]
	g.seqMutex.Unlock()

	// Generate value based on pattern - use stateKey to maintain separate state per source_id:tag
	stateKey := seqKey // Use same key format for state management
	value := g.generateValue(tagConfig, stateKey)

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

	// Set default equipment_type if not configured
	equipmentType := g.config.EquipmentType
	if equipmentType == "" {
		equipmentType = "Sensor" // Default to Sensor
	}

	// Generate equipment_name based on tag and line number (extracted from sourceId)
	// Format: "{Tag Name} {Equipment Type} Line{Number}" (e.g., "Flow Meter Line3")
	equipmentName := g.config.EquipmentName
	if equipmentName == "" {
		// Extract line number from sourceId (format: "{factory}-line{number}" or "sim-xxx")
		lineNumber := extractLineNumber(sourceID)
		
		// Generate equipment name based on tag
		tagName := getTagDisplayName(tagConfig.Tag)
		equipmentTypeName := getEquipmentTypeName(tagConfig.Tag, equipmentType)
		
		if lineNumber > 0 {
			equipmentName = fmt.Sprintf("%s %s Line%d", tagName, equipmentTypeName, lineNumber)
		} else {
			// Fallback if line number cannot be extracted
			equipmentName = fmt.Sprintf("%s %s", tagName, equipmentTypeName)
		}
	}

	return TelemetryMessage{
		TS:            time.Now(),
		SourceID:      sourceID,
		FactoryID:     factoryID,
		EquipmentType: equipmentType,
		EquipmentName: equipmentName,
		Tag:           tagConfig.Tag,
		Value:         value,
		Unit:          tagConfig.Unit,
		Quality:       quality,
		Seq:           seq,
		TraceID:       traceID,
	}, nil
}

// generateValue generates a value based on the tag's pattern configuration
// stateKey is used to maintain separate state for each source_id:tag combination
func (g *DefaultGenerator) generateValue(tc TagConfig, stateKey string) interface{} {
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
		// Initialize state if needed - use stateKey to maintain separate state per source_id:tag
		if _, exists := g.state[stateKey]; !exists {
			g.state[stateKey] = (tc.Min + tc.Max) / 2
		}
		elapsed := time.Since(g.startTime).Seconds()
		period := 60.0 // 60 second period
		amplitude := (tc.Max - tc.Min) / 2
		center := (tc.Min + tc.Max) / 2
		// Add some variation based on stateKey hash to make different sources have different phases
		phaseOffset := float64(len(stateKey)) * 0.1
		value := center + amplitude*math.Sin(2*math.Pi*elapsed/period+phaseOffset)
		return value

	case PatternStep:
		// Initialize state if needed - use stateKey to maintain separate state per source_id:tag
		if _, exists := g.state[stateKey]; !exists {
			g.state[stateKey] = tc.Min
		}
		// Toggle between min and max every 10 seconds
		elapsed := int(time.Since(g.startTime).Seconds())
		if elapsed%20 < 10 {
			g.state[stateKey] = tc.Min
		} else {
			g.state[stateKey] = tc.Max
		}
		return g.state[stateKey]

	case PatternRandomWalk:
		// Initialize state if needed - use stateKey to maintain separate state per source_id:tag
		if _, exists := g.state[stateKey]; !exists {
			g.state[stateKey] = (tc.Min + tc.Max) / 2
		}
		stepSize := tc.StepSize
		if stepSize == 0 {
			stepSize = (tc.Max - tc.Min) / 100
		}
		// Random walk: add random step
		step := (g.rng.Float64() - 0.5) * 2 * stepSize
		g.state[stateKey] += step
		// Clamp to bounds
		if g.state[stateKey] < tc.Min {
			g.state[stateKey] = tc.Min
		}
		if g.state[stateKey] > tc.Max {
			g.state[stateKey] = tc.Max
		}
		return g.state[stateKey]

	default:
		// Default to uniform
		return g.rng.Float64()*(tc.Max-tc.Min) + tc.Min
	}
}

// generateTraceID generates a simple trace ID
func generateTraceID() string {
	return fmt.Sprintf("%016x", rand.Int63())
}

// extractLineNumber extracts line number from sourceId
// Supports formats: "{factory}-line{number}" (e.g., "ulsan-line3") or "line{number}" (e.g., "line3")
func extractLineNumber(sourceID string) int {
	// Try pattern: "-line{number}" or "line{number}"
	re := regexp.MustCompile(`(?i)(?:-)?line(\d+)`)
	matches := re.FindStringSubmatch(sourceID)
	if len(matches) > 1 {
		if num, err := strconv.Atoi(matches[1]); err == nil {
			return num
		}
	}
	return 0
}

// getTagDisplayName returns the display name for a tag
func getTagDisplayName(tag string) string {
	switch strings.ToLower(tag) {
	case "temp":
		return "Temperature"
	case "humidity":
		return "Humidity"
	case "pressure":
		return "Pressure"
	case "vibration":
		return "Vibration"
	case "power":
		return "Power"
	case "flow":
		return "Flow"
	default:
		return strings.Title(tag)
	}
}

// getEquipmentTypeName returns the equipment type name based on tag
func getEquipmentTypeName(tag, defaultType string) string {
	switch strings.ToLower(tag) {
	case "power", "flow":
		return "Meter"
	case "temp", "humidity", "pressure", "vibration":
		return "Sensor"
	default:
		if defaultType != "" {
			return defaultType
		}
		return "Sensor"
	}
}

