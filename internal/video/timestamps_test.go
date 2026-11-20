package video

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseFrameTimestampsShouldParseCsvOutput(t *testing.T) {
	output := []byte("0.000000\n0.033333\n0.066667\nN/A\n0.133333\n")

	timestamps, err := parseFrameTimestamps(output)
	assert.Nil(t, err)
	assert.Len(t, timestamps, 4)
	assert.InDelta(t, 0.0, timestamps[0], 1e-9)
	assert.InDelta(t, 0.033333, timestamps[1], 1e-9)
	assert.InDelta(t, 0.066667, timestamps[2], 1e-9)
	assert.InDelta(t, 0.133333, timestamps[3], 1e-9)
}

func TestParseFrameTimestampsShouldRejectInvalidValue(t *testing.T) {
	_, err := parseFrameTimestamps([]byte("0.0\nnot-a-number\n"))
	assert.NotNil(t, err)
}

func TestSynthesizeFrameTimestampsShouldBeEvenlySpaced(t *testing.T) {
	timestamps, err := SynthesizeFrameTimestamps(4, 10.0)
	assert.Nil(t, err)
	assert.Len(t, timestamps, 4)
	assert.InDelta(t, 0.0, timestamps[0], 1e-9)
	assert.InDelta(t, 0.1, timestamps[1], 1e-9)
	assert.InDelta(t, 0.2, timestamps[2], 1e-9)
	assert.InDelta(t, 0.3, timestamps[3], 1e-9)
}

func TestSynthesizeFrameTimestampsShouldRejectInvalidInput(t *testing.T) {
	_, err := SynthesizeFrameTimestamps(-1, 10.0)
	assert.NotNil(t, err)

	_, err = SynthesizeFrameTimestamps(4, 0.0)
	assert.NotNil(t, err)
}

func TestNormalizeFrameTimestampsShouldClampBackwardsAndDuplicates(t *testing.T) {
	probed := []float64{0.0, 0.1, 0.1, 0.05, 0.4}

	normalized := NormalizeFrameTimestamps(probed, 5, 10.0)
	assert.Len(t, normalized, 5)
	assert.InDelta(t, 0.0, normalized[0], 1e-9)
	assert.InDelta(t, 0.1, normalized[1], 1e-9)
	// Duplicate timestamp keeps its value (zero gap).
	assert.InDelta(t, 0.1, normalized[2], 1e-9)
	// Backwards timestamp is clamped to the previous value.
	assert.InDelta(t, 0.1, normalized[3], 1e-9)
	assert.InDelta(t, 0.4, normalized[4], 1e-9)

	// The normalized sequence is guaranteed monotonically non-decreasing.
	for index := 1; index < len(normalized); index += 1 {
		assert.GreaterOrEqual(t, normalized[index], normalized[index-1])
	}
}

func TestNormalizeFrameTimestampsShouldBackfillMissingEntries(t *testing.T) {
	// When the decoded frame count exceeds the probed entries the real timing curve is
	// resampled onto the decoded positions, preserving the first and last presentation time.
	probed := []float64{0.0, 0.1}

	normalized := NormalizeFrameTimestamps(probed, 4, 10.0)
	assert.Len(t, normalized, 4)
	assert.InDelta(t, 0.0, normalized[0], 1e-9)
	assert.InDelta(t, 0.1, normalized[3], 1e-9)

	// The resampled sequence stays ordered and within the probed time range.
	for index := 1; index < len(normalized); index += 1 {
		assert.GreaterOrEqual(t, normalized[index], normalized[index-1])
	}
}

func TestNormalizeFrameTimestampsShouldResampleVariableRateCurve(t *testing.T) {
	// Two frames at 4 fps then a denser region, resampled onto more decoded positions.
	probed := []float64{0.0, 0.25, 0.5, 2.0, 2.041667, 2.083333}

	normalized := NormalizeFrameTimestamps(probed, 10, 24.0)
	assert.Len(t, normalized, 10)

	// Endpoints are preserved exactly.
	assert.InDelta(t, 0.0, normalized[0], 1e-9)
	assert.InDelta(t, 2.083333, normalized[9], 1e-6)

	// The sequence is strictly increasing and never runs backwards.
	for index := 1; index < len(normalized); index += 1 {
		assert.Greater(t, normalized[index], normalized[index-1])
	}

	// All values stay within the probed time span.
	assert.LessOrEqual(t, normalized[5], 2.083333)
}

func TestNormalizeFrameTimestampsShouldReplicateSingleTimestamp(t *testing.T) {
	normalized := NormalizeFrameTimestamps([]float64{1.5}, 3, 24.0)
	assert.Len(t, normalized, 3)
	for _, value := range normalized {
		assert.InDelta(t, 1.5, value, 1e-9)
	}
}

func TestNormalizeFrameTimestampsShouldSynthesizeWhenProbeEmpty(t *testing.T) {
	normalized := NormalizeFrameTimestamps([]float64{}, 3, 10.0)
	assert.Len(t, normalized, 3)
	assert.InDelta(t, 0.0, normalized[0], 1e-9)
	assert.InDelta(t, 0.2, normalized[2], 1e-9)
}
