package options

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestShouldValidateDefaultOptions(t *testing.T) {
	options := GetDefaultDetectorOptions()

	valid, msg := options.AreValid()
	assert.True(t, valid)
	assert.Empty(t, msg)
}

func TestShouldNotValidateInvalidBrightnessDetectionThreshold(t *testing.T) {
	cases := []float64{-0.1, 1.1}

	for _, value := range cases {
		options := GetDefaultDetectorOptions()
		options.BrightnessDetectionThreshold = value

		valid, msg := options.AreValid()
		assert.False(t, valid)
		assert.NotEmpty(t, msg)
	}
}

func TestShouldNotValidateInvalidColorDifferenceDetectionThreshold(t *testing.T) {
	cases := []float64{-0.1, 1.1}

	for _, value := range cases {
		options := GetDefaultDetectorOptions()
		options.ColorDifferenceDetectionThreshold = value

		valid, msg := options.AreValid()
		assert.False(t, valid)
		assert.NotEmpty(t, msg)
	}
}

func TestShouldNotValidateInvalidBinaryThresholdDetectionThreshold(t *testing.T) {
	cases := []float64{-0.1, 1.1}

	for _, value := range cases {
		options := GetDefaultDetectorOptions()
		options.BinaryThresholdDifferenceDetectionThreshold = value

		valid, msg := options.AreValid()
		assert.False(t, valid)
		assert.NotEmpty(t, msg)
	}
}

func TestShouldNotValidateInvalidFrameScalingFactor(t *testing.T) {
	cases := []float64{-0.1, 1.1}

	for _, value := range cases {
		options := GetDefaultDetectorOptions()
		options.FrameScalingFactor = value

		valid, msg := options.AreValid()
		assert.False(t, valid)
		assert.NotEmpty(t, msg)
	}
}

func TestShouldNotValidateNegativeEventMergeInterval(t *testing.T) {
	options := GetDefaultDetectorOptions()
	options.EventMergeMaxIntervalSeconds = -0.1

	valid, msg := options.AreValid()
	assert.False(t, valid)
	assert.NotEmpty(t, msg)
}

func TestShouldNotValidateNegativeEventClipBuffers(t *testing.T) {
	preBuffer := GetDefaultDetectorOptions()
	preBuffer.EventClipPreBufferSeconds = -0.1
	valid, msg := preBuffer.AreValid()
	assert.False(t, valid)
	assert.NotEmpty(t, msg)

	postBuffer := GetDefaultDetectorOptions()
	postBuffer.EventClipPostBufferSeconds = -0.1
	valid, msg = postBuffer.AreValid()
	assert.False(t, valid)
	assert.NotEmpty(t, msg)
}

func TestShouldCloneEventOptions(t *testing.T) {
	options := GetDefaultDetectorOptions()
	options.EventMergeMaxIntervalSeconds = 1.25
	options.EventClipPreBufferSeconds = 2.5
	options.EventClipPostBufferSeconds = 3.75
	options.ExportEventClips = true

	clone := options.Clone()
	assert.Equal(t, 1.25, clone.EventMergeMaxIntervalSeconds)
	assert.Equal(t, 2.5, clone.EventClipPreBufferSeconds)
	assert.Equal(t, 3.75, clone.EventClipPostBufferSeconds)
	assert.True(t, clone.ExportEventClips)
}
