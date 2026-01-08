package gen

import (
	"encoding/json"
	"testing"
)

func TestAggregatedResult_SelfReference(t *testing.T) {
	// Test that we can create a self-referencing structure
	result := AggregatedResult{
		TotalClicks: ptr(100),
		HourlyBreakDown: map[string]AggregatedResult{
			"hour1": {
				TotalClicks: ptr(50),
				HourlyBreakDown: map[string]AggregatedResult{
					"hour1.1": {
						TotalClicks: ptr(25),
					},
				},
			},
			"hour2": {
				TotalClicks: ptr(50),
			},
		},
	}

	// Test JSON marshaling
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	// Test JSON unmarshaling
	var decoded AggregatedResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	// Verify the structure
	if decoded.TotalClicks == nil || *decoded.TotalClicks != 100 {
		t.Errorf("Expected TotalClicks=100, got %v", decoded.TotalClicks)
	}

	if len(decoded.HourlyBreakDown) != 2 {
		t.Errorf("Expected 2 hourly breakdowns, got %d", len(decoded.HourlyBreakDown))
	}

	hour1 := decoded.HourlyBreakDown["hour1"]
	if hour1.TotalClicks == nil || *hour1.TotalClicks != 50 {
		t.Errorf("Expected hour1 TotalClicks=50, got %v", hour1.TotalClicks)
	}

	if len(hour1.HourlyBreakDown) != 1 {
		t.Errorf("Expected 1 nested hourly breakdown, got %d", len(hour1.HourlyBreakDown))
	}
}

func ptr(i int) *int {
	return &i
}
