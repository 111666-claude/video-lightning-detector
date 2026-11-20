package event

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/Krzysztofz01/video-lightning-detector/internal/frame"
)

// buildTestCollection creates a frame collection where each frame carries an explicit
// presentation timestamp and brightness. It is the deterministic "fixed bright-spot
// sequence" used to verify event counts, peaks and time ranges without any video decoding.
func buildTestCollection(t *testing.T, timestamps, brightness []float64) frame.FrameCollection {
	t.Helper()

	if len(timestamps) != len(brightness) {
		t.Fatalf("timestamps and brightness slices must have equal length")
	}

	collection := frame.NewFrameCollection(len(timestamps))
	factory := frame.CreateFrameFactory(frame.BinaryThresholdParam)

	img := newSolidRgba(4, 4)
	for index := range timestamps {
		f, err := factory.CreateNewFrame(img, img, timestamps[index])
		if err != nil {
			t.Fatalf("failed to create frame %d: %v", index, err)
		}

		f.Brightness = brightness[index]

		if err := collection.Push(f); err != nil {
			t.Fatalf("failed to push frame %d: %v", index, err)
		}
	}

	return collection
}

func TestBuildTimelineShouldReturnEmptyForNoDetections(t *testing.T) {
	frames := buildTestCollection(t,
		[]float64{0, 0.1, 0.2, 0.3},
		[]float64{0.1, 0.1, 0.1, 0.1},
	)

	timeline, err := BuildTimeline(frames, []int{}, 0.5)
	assert.Nil(t, err)
	assert.Empty(t, timeline.Events)
	assert.Equal(t, 4, timeline.FrameCount)
	assert.Equal(t, 0, timeline.NonMonotonicTimestampCount)
}

func TestBuildTimelineShouldMergeThirtyFrameStrikeIntoSingleEvent(t *testing.T) {
	// A single strike lasting thirty consecutive frames at 30 fps must produce one event.
	const frameCount = 30

	timestamps := make([]float64, frameCount)
	brightness := make([]float64, frameCount)
	detections := make([]int, frameCount)

	for index := 0; index < frameCount; index += 1 {
		timestamps[index] = float64(index) / 30.0
		brightness[index] = 0.4
		detections[index] = index
	}

	// The peak sits in the middle of the strike.
	brightness[15] = 0.95

	frames := buildTestCollection(t, timestamps, brightness)

	timeline, err := BuildTimeline(frames, detections, 0.5)
	assert.Nil(t, err)
	assert.Len(t, timeline.Events, 1)

	ev := timeline.Events[0]
	assert.Equal(t, 0, ev.StartFrameIndex)
	assert.Equal(t, 29, ev.EndFrameIndex)
	assert.Equal(t, 15, ev.PeakFrameIndex)
	assert.Equal(t, 0.95, ev.PeakIntensity)
	assert.Equal(t, 30, ev.HitFrameCount)
	assert.InDelta(t, 0.0, ev.StartTime, 1e-9)
	assert.InDelta(t, 29.0/30.0, ev.EndTime, 1e-9)
	assert.InDelta(t, 0.5, ev.PeakTime, 1e-9)
}

func TestBuildTimelineShouldSeparateEventsAcrossConfiguredInterval(t *testing.T) {
	// Two strikes separated by a gap larger than the configured interval become two events.
	timestamps := []float64{0.0, 0.1, 0.2, 1.0, 1.1, 1.2}
	brightness := []float64{0.8, 0.9, 0.7, 0.6, 0.95, 0.5}
	detections := []int{0, 1, 2, 3, 4, 5}

	frames := buildTestCollection(t, timestamps, brightness)

	timeline, err := BuildTimeline(frames, detections, 0.5)
	assert.Nil(t, err)
	assert.Len(t, timeline.Events, 2)

	assert.Equal(t, 0, timeline.Events[0].Index)
	assert.Equal(t, 0, timeline.Events[0].StartFrameIndex)
	assert.Equal(t, 2, timeline.Events[0].EndFrameIndex)
	assert.Equal(t, 1, timeline.Events[0].PeakFrameIndex)
	assert.Equal(t, 0.9, timeline.Events[0].PeakIntensity)

	assert.Equal(t, 1, timeline.Events[1].Index)
	assert.Equal(t, 3, timeline.Events[1].StartFrameIndex)
	assert.Equal(t, 5, timeline.Events[1].EndFrameIndex)
	assert.Equal(t, 4, timeline.Events[1].PeakFrameIndex)
	assert.Equal(t, 0.95, timeline.Events[1].PeakIntensity)

	// A generous interval merges everything back into a single event.
	merged, err := BuildTimeline(frames, detections, 1.0)
	assert.Nil(t, err)
	assert.Len(t, merged.Events, 1)
	assert.Equal(t, 4, merged.Events[0].PeakFrameIndex)
}

func TestBuildTimelineShouldMergeUsingRealTimeForVariableFrameRate(t *testing.T) {
	// Variable frame-rate: frame indexes are consecutive but the time gaps differ. With a
	// frame-count based merge the strikes would split, real-time merging keeps them together.
	timestamps := []float64{0.0, 0.05, 0.4, 0.45, 2.0, 2.05}
	brightness := []float64{0.7, 0.8, 0.9, 0.6, 0.7, 0.8}
	detections := []int{0, 1, 2, 3, 4, 5}

	frames := buildTestCollection(t, timestamps, brightness)

	// Interval of 0.5s: the 0.35s gap stays merged, the 1.55s gap splits into a new event.
	timeline, err := BuildTimeline(frames, detections, 0.5)
	assert.Nil(t, err)
	assert.Len(t, timeline.Events, 2)
	assert.Equal(t, 3, timeline.Events[0].EndFrameIndex)
	assert.Equal(t, 4, timeline.Events[1].StartFrameIndex)
}

func TestBuildTimelineShouldStartNewEventForNonConsecutiveHits(t *testing.T) {
	// Even with timestamps close together, a missing hit frame forces a new event.
	timestamps := []float64{0.0, 0.1, 0.2, 0.3}
	brightness := []float64{0.9, 0.1, 0.1, 0.9}
	detections := []int{0, 3}

	frames := buildTestCollection(t, timestamps, brightness)

	timeline, err := BuildTimeline(frames, detections, 5.0)
	assert.Nil(t, err)
	assert.Len(t, timeline.Events, 2)
	assert.Equal(t, 0, timeline.Events[0].StartFrameIndex)
	assert.Equal(t, 3, timeline.Events[1].StartFrameIndex)
}

func TestBuildTimelineShouldHandleIsolatedBrightNoiseAsSingleFrameEvent(t *testing.T) {
	// An isolated bright noise point is a valid single-frame event with equal bounds.
	timestamps := []float64{0.0, 0.1, 0.2}
	brightness := []float64{0.1, 0.99, 0.1}
	detections := []int{1}

	frames := buildTestCollection(t, timestamps, brightness)

	timeline, err := BuildTimeline(frames, detections, 0.5)
	assert.Nil(t, err)
	assert.Len(t, timeline.Events, 1)

	ev := timeline.Events[0]
	assert.Equal(t, 1, ev.StartFrameIndex)
	assert.Equal(t, 1, ev.EndFrameIndex)
	assert.Equal(t, 1, ev.PeakFrameIndex)
	assert.Equal(t, 0.99, ev.PeakIntensity)
	assert.Equal(t, 1, ev.HitFrameCount)
}

func TestBuildTimelineShouldHandleDuplicatedAndBackwardsTimestamps(t *testing.T) {
	timestamps := []float64{0.0, 0.0, 0.0, 0.3}
	brightness := []float64{0.7, 0.8, 0.9, 0.6}
	detections := []int{0, 1, 2, 3}

	frames := buildTestCollection(t, timestamps, brightness)

	timeline, err := BuildTimeline(frames, detections, 0.5)
	assert.Nil(t, err)
	// Two duplicated timestamps (frames 1 and 2) are non-monotonic relative to the previous.
	assert.Equal(t, 2, timeline.NonMonotonicTimestampCount)
	// Zero gaps never exceed the interval, so all four frames remain one event.
	assert.Len(t, timeline.Events, 1)

	// Backwards timestamps are likewise tolerated.
	backwards := []float64{0.5, 0.4, 0.3}
	backwardsBrightness := []float64{0.7, 0.8, 0.9}
	backwardsFrames := buildTestCollection(t, backwards, backwardsBrightness)

	backwardsTimeline, err := BuildTimeline(backwardsFrames, []int{0, 1, 2}, 0.5)
	assert.Nil(t, err)
	assert.Equal(t, 2, backwardsTimeline.NonMonotonicTimestampCount)
	assert.Len(t, backwardsTimeline.Events, 1)
}

func TestBuildTimelineShouldBeStableRegardlessOfDetectionOrder(t *testing.T) {
	timestamps := []float64{0.0, 0.1, 1.0, 1.1}
	brightness := []float64{0.7, 0.8, 0.9, 0.6}

	frames := buildTestCollection(t, timestamps, brightness)

	shuffled, err := BuildTimeline(frames, []int{3, 1, 2, 0}, 0.5)
	assert.Nil(t, err)

	ordered, err := BuildTimeline(frames, []int{0, 1, 2, 3}, 0.5)
	assert.Nil(t, err)

	assert.Equal(t, ordered.Events, shuffled.Events)
}

func TestBuildTimelineShouldKeepEarliestPeakOnEqualBrightness(t *testing.T) {
	timestamps := []float64{0.0, 0.1, 0.2}
	brightness := []float64{0.9, 0.9, 0.9}

	frames := buildTestCollection(t, timestamps, brightness)

	timeline, err := BuildTimeline(frames, []int{0, 1, 2}, 0.5)
	assert.Nil(t, err)
	assert.Len(t, timeline.Events, 1)
	assert.Equal(t, 0, timeline.Events[0].PeakFrameIndex)
}

func TestBuildTimelineShouldRejectInvalidArguments(t *testing.T) {
	frames := buildTestCollection(t, []float64{0.0, 0.1}, []float64{0.1, 0.2})

	_, err := BuildTimeline(frames, []int{0}, -1.0)
	assert.NotNil(t, err)

	_, err = BuildTimeline(frames, []int{5}, 0.5)
	assert.NotNil(t, err)

	_, err = BuildTimeline(frames, []int{-1}, 0.5)
	assert.NotNil(t, err)

	_, err = BuildTimeline(nil, []int{0}, 0.5)
	assert.NotNil(t, err)
}

func TestBuildTimelineShouldHandleEmptyCollection(t *testing.T) {
	// A video that decodes to zero frames yields an explicit empty timeline, not an error.
	collection := frame.NewFrameCollection(1)
	timeline, err := BuildTimeline(collection, []int{}, 0.5)
	assert.Nil(t, err)
	assert.Empty(t, timeline.Events)
	assert.Equal(t, 0, timeline.FrameCount)
	assert.Equal(t, 0, timeline.NonMonotonicTimestampCount)
}

func TestBuildTimelineShouldTolerateNilDetections(t *testing.T) {
	frames := buildTestCollection(t, []float64{0.0, 0.1}, []float64{0.1, 0.2})

	timeline, err := BuildTimeline(frames, nil, 0.5)
	assert.Nil(t, err)
	assert.Empty(t, timeline.Events)
}
