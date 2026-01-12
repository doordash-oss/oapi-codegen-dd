package nested

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/doordash-oss/oapi-codegen-dd/v3/pkg/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper to create TimeBasedLocation with AbsoluteTimeRange
func createTimeBasedWithAbsolute(start, end time.Time) PointRequest {
	return PointRequest{
		Location: PointRequestOneOf{
			PointRequestOneOf_OneOf: &PointRequestOneOf_OneOf{
				Either: runtime.NewEitherFromA[TimeBasedLocation, DistanceBasedLocation](
					TimeBasedLocation{
						Time: TimeInterval{
							Interval: TimeIntervalType{
								TimeIntervalType_OneOf: &TimeIntervalType_OneOf{
									Either: runtime.NewEitherFromA[AbsoluteTimeRange, RelativeTimeDuration](
										AbsoluteTimeRange{Start: start, End: end},
									),
								},
							},
						},
					},
				),
			},
		},
	}
}

// Helper to create TimeBasedLocation with RelativeTimeDuration
func createTimeBasedWithRelative(duration int) PointRequest {
	return PointRequest{
		Location: PointRequestOneOf{
			PointRequestOneOf_OneOf: &PointRequestOneOf_OneOf{
				Either: runtime.NewEitherFromA[TimeBasedLocation, DistanceBasedLocation](
					TimeBasedLocation{
						Time: TimeInterval{
							Interval: TimeIntervalType{
								TimeIntervalType_OneOf: &TimeIntervalType_OneOf{
									Either: runtime.NewEitherFromB[AbsoluteTimeRange, RelativeTimeDuration](
										RelativeTimeDuration{Duration: duration},
									),
								},
							},
						},
					},
				),
			},
		},
	}
}

// Helper to create DistanceBasedLocation
func createDistanceBased(distance float32) PointRequest {
	return PointRequest{
		Location: PointRequestOneOf{
			PointRequestOneOf_OneOf: &PointRequestOneOf_OneOf{
				Either: runtime.NewEitherFromB[TimeBasedLocation, DistanceBasedLocation](
					DistanceBasedLocation{Distance: distance},
				),
			},
		},
	}
}

func TestValid_DoubleNesting_TimeBasedWithAbsoluteRange(t *testing.T) {
	// Nesting: PointRequest -> TimeBased -> AbsoluteRange
	request := createTimeBasedWithAbsolute(time.Now(), time.Now().Add(time.Hour))

	err := request.Validate()
	assert.NoError(t, err)
}

func TestValid_DoubleNesting_TimeBasedWithRelativeDuration(t *testing.T) {
	// Nesting: PointRequest -> TimeBased -> RelativeDuration
	request := createTimeBasedWithRelative(5) // Valid: >= 2

	err := request.Validate()
	assert.NoError(t, err)
}

func TestValid_SingleNesting_DistanceBased(t *testing.T) {
	// Nesting: PointRequest -> DistanceBased (no inner union)
	request := createDistanceBased(100.5)

	err := request.Validate()
	assert.NoError(t, err)
}

func TestInvalid_DoubleNesting_InvalidAbsoluteRange(t *testing.T) {
	// Nesting: PointRequest -> TimeBased -> AbsoluteRange (with invalid data)
	request := createTimeBasedWithAbsolute(time.Time{}, time.Now()) // Invalid: zero start time

	err := request.Validate()
	assert.Error(t, err, "Expected validation error for zero start time")
	assert.Contains(t, err.Error(), "Start")
}

func TestInvalid_DoubleNesting_InvalidRelativeDuration(t *testing.T) {
	// Nesting: PointRequest -> TimeBased -> RelativeDuration (with invalid data)
	request := createTimeBasedWithRelative(1) // Invalid: minimum is 2

	err := request.Validate()
	assert.Error(t, err, "Expected validation error for duration < 2")
	assert.Equal(t, "Location.PointRequestOneOf_OneOf.Time.Interval.TimeIntervalType_OneOf.Duration must be greater than or equal to 2", err.Error())
}

func TestInvalid_SingleNesting_InvalidDistance(t *testing.T) {
	// Nesting: PointRequest -> DistanceBased (with invalid data)
	request := createDistanceBased(-10.0) // Invalid: minimum is 0

	err := request.Validate()
	assert.Error(t, err, "Expected validation error for negative distance")
	assert.Equal(t, "Location.PointRequestOneOf_OneOf.Distance must be greater than or equal to 0", err.Error())
}

func TestCritical_InactiveVariantsNotValidated(t *testing.T) {
	// When TimeBased is active, DistanceBased should NOT be validated
	// When AbsoluteRange is active, RelativeDuration should NOT be validated
	request := createTimeBasedWithAbsolute(time.Now(), time.Now().Add(time.Hour))

	err := request.Validate()
	assert.NoError(t, err, "Should not validate inactive variants")

	// Verify correct variants are active
	assert.True(t, request.Location.PointRequestOneOf_OneOf.IsA(), "TimeBased should be active")
	assert.True(t, request.Location.PointRequestOneOf_OneOf.A.Time.Interval.TimeIntervalType_OneOf.IsA(), "AbsoluteRange should be active")
}

func TestJSON_UnmarshalTimeBasedWithAbsoluteRange(t *testing.T) {
	jsonData := `{
		"location": {
			"time": {
				"interval": {
					"start": "2024-01-01T00:00:00Z",
					"end": "2024-01-01T01:00:00Z"
				}
			}
		}
	}`

	var request PointRequest
	err := json.Unmarshal([]byte(jsonData), &request)
	require.NoError(t, err)

	err = request.Validate()
	assert.NoError(t, err)

	// Verify correct variants are active
	assert.NotNil(t, request.Location.PointRequestOneOf_OneOf)
	assert.True(t, request.Location.PointRequestOneOf_OneOf.IsA(), "TimeBased should be active")
	assert.True(t, request.Location.PointRequestOneOf_OneOf.A.Time.Interval.TimeIntervalType_OneOf.IsA(), "AbsoluteRange should be active")
}

func TestJSON_UnmarshalDistanceBased(t *testing.T) {
	jsonData := `{
		"location": {
			"distance": 50.5
		}
	}`

	var request PointRequest
	err := json.Unmarshal([]byte(jsonData), &request)
	require.NoError(t, err)

	err = request.Validate()
	assert.NoError(t, err)

	// Verify correct variant is active
	assert.NotNil(t, request.Location.PointRequestOneOf_OneOf)
	assert.True(t, request.Location.PointRequestOneOf_OneOf.IsB(), "DistanceBased should be active")
}

func TestJSON_RoundTrip(t *testing.T) {
	original := createTimeBasedWithAbsolute(time.Now(), time.Now().Add(time.Hour))

	// Marshal to JSON
	data, err := json.Marshal(original)
	require.NoError(t, err)

	// Unmarshal back
	var decoded PointRequest
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	// Validate decoded
	err = decoded.Validate()
	assert.NoError(t, err)
}
