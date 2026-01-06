package tests

import (
	"context"
	"testing"

	"github.com/smartfactory/simulator/internal/core"
)

func TestGeneratorUniformPattern(t *testing.T) {
	config := core.GeneratorConfig{
		SourceID: "test-001",
		Tags: []core.TagConfig{
			{
				Tag:     "temp",
				Pattern: core.PatternUniform,
				Min:     20.0,
				Max:     70.0,
				Unit:    "C",
				Quality: core.QualityGood,
			},
		},
		Seed: 12345, // Fixed seed for reproducibility
	}

	generator := core.NewGenerator(config)
	ctx := context.Background()

	// Generate 100 messages and verify they're within bounds
	for i := 0; i < 100; i++ {
		msg, err := generator.Next(ctx)
		if err != nil {
			t.Fatalf("Failed to generate message: %v", err)
		}

		if msg.SourceID != "test-001" {
			t.Errorf("Expected SourceID 'test-001', got '%s'", msg.SourceID)
		}

		if msg.Tag != "temp" {
			t.Errorf("Expected Tag 'temp', got '%s'", msg.Tag)
		}

		value, ok := msg.Value.(float64)
		if !ok {
			t.Fatalf("Expected float64 value, got %T", msg.Value)
		}

		if value < 20.0 || value > 70.0 {
			t.Errorf("Value %.2f is outside bounds [20.0, 70.0]", value)
		}

		if msg.Unit != "C" {
			t.Errorf("Expected Unit 'C', got '%s'", msg.Unit)
		}

		if msg.Quality != core.QualityGood {
			t.Errorf("Expected Quality 'Good', got '%s'", msg.Quality)
		}

		if msg.Seq != int64(i+1) {
			t.Errorf("Expected Seq %d, got %d", i+1, msg.Seq)
		}
	}
}

func TestGeneratorSequencePerTag(t *testing.T) {
	config := core.GeneratorConfig{
		SourceID: "test-002",
		Tags: []core.TagConfig{
			{
				Tag:     "temp",
				Pattern: core.PatternUniform,
				Min:     20.0,
				Max:     30.0,
			},
			{
				Tag:     "humidity",
				Pattern: core.PatternUniform,
				Min:     40.0,
				Max:     50.0,
			},
		},
		Seed: 54321,
	}

	generator := core.NewGenerator(config)
	ctx := context.Background()

	// Generate messages and track sequences per tag
	tempSeqs := make([]int64, 0)
	humiditySeqs := make([]int64, 0)

	for i := 0; i < 50; i++ {
		msg, err := generator.Next(ctx)
		if err != nil {
			t.Fatalf("Failed to generate message: %v", err)
		}

		if msg.Tag == "temp" {
			tempSeqs = append(tempSeqs, msg.Seq)
		} else if msg.Tag == "humidity" {
			humiditySeqs = append(humiditySeqs, msg.Seq)
		}
	}

	// Verify sequences are independent per tag
	if len(tempSeqs) == 0 || len(humiditySeqs) == 0 {
		t.Fatal("Both tags should have been generated")
	}

	// Verify sequences are increasing within each tag
	for i := 1; i < len(tempSeqs); i++ {
		if tempSeqs[i] <= tempSeqs[i-1] {
			t.Errorf("Temp sequence should be increasing: %d <= %d", tempSeqs[i], tempSeqs[i-1])
		}
	}

	for i := 1; i < len(humiditySeqs); i++ {
		if humiditySeqs[i] <= humiditySeqs[i-1] {
			t.Errorf("Humidity sequence should be increasing: %d <= %d", humiditySeqs[i], humiditySeqs[i-1])
		}
	}
}

func TestGeneratorNormalPattern(t *testing.T) {
	config := core.GeneratorConfig{
		SourceID: "test-003",
		Tags: []core.TagConfig{
			{
				Tag:     "pressure",
				Pattern: core.PatternNormal,
				Min:     900.0,
				Max:     1100.0,
				Mean:    1000.0,
				StdDev:  50.0,
			},
		},
		Seed: 99999,
	}

	generator := core.NewGenerator(config)
	ctx := context.Background()

	// Generate messages and verify they're within bounds
	for i := 0; i < 100; i++ {
		msg, err := generator.Next(ctx)
		if err != nil {
			t.Fatalf("Failed to generate message: %v", err)
		}

		value, ok := msg.Value.(float64)
		if !ok {
			t.Fatalf("Expected float64 value, got %T", msg.Value)
		}

		if value < 900.0 || value > 1100.0 {
			t.Errorf("Value %.2f is outside bounds [900.0, 1100.0]", value)
		}
	}
}

func TestGeneratorJSONSerialization(t *testing.T) {
	config := core.GeneratorConfig{
		SourceID: "test-004",
		Tags: []core.TagConfig{
			{
				Tag:     "temp",
				Pattern: core.PatternUniform,
				Min:     20.0,
				Max:     30.0,
				Unit:    "C",
			},
		},
	}

	generator := core.NewGenerator(config)
	ctx := context.Background()

	msg, err := generator.Next(ctx)
	if err != nil {
		t.Fatalf("Failed to generate message: %v", err)
	}

	// Serialize to JSON
	jsonBytes, err := msg.ToJSON()
	if err != nil {
		t.Fatalf("Failed to serialize message: %v", err)
	}

	// Verify JSON contains required fields
	jsonStr := string(jsonBytes)
	requiredFields := []string{"ts", "sourceId", "tag", "value", "quality", "seq"}
	for _, field := range requiredFields {
		if !contains(jsonStr, field) {
			t.Errorf("JSON missing required field: %s", field)
		}
	}
}


