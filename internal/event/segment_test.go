package event

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPlanSegmentsShouldApplyBufferAndClampToBounds(t *testing.T) {
	timestamps := []float64{0.0, 0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9}
	brightness := make([]float64, len(timestamps))
	brightness[5] = 0.9

	frames := buildTestCollection(t, timestamps, brightness)

	timeline, err := BuildTimeline(frames, []int{5}, 0.5)
	assert.Nil(t, err)

	// 0.2s pre and post buffer around the event at 0.5s -> frames 3..7.
	segments, err := PlanSegments(timeline, timestamps, 0.2, 0.2)
	assert.Nil(t, err)
	assert.Len(t, segments, 1)

	segment := segments[0]
	assert.Equal(t, 3, segment.StartFrameIndex)
	assert.Equal(t, 7, segment.EndFrameIndex)
	assert.InDelta(t, 0.3, segment.StartTime, 1e-9)
	assert.InDelta(t, 0.7, segment.EndTime, 1e-9)
	assert.False(t, segment.Merged)
	assert.Equal(t, []int{0}, segment.EventIndexes)
}

func TestPlanSegmentsShouldClampAtVideoBoundaries(t *testing.T) {
	timestamps := []float64{0.0, 0.1, 0.2}
	brightness := []float64{0.9, 0.1, 0.9}

	frames := buildTestCollection(t, timestamps, brightness)

	timeline, err := BuildTimeline(frames, []int{0, 2}, 5.0)
	assert.Nil(t, err)
	assert.Len(t, timeline.Events, 2)

	segments, err := PlanSegments(timeline, timestamps, 1.0, 1.0)
	assert.Nil(t, err)
	// The large buffers make both segments cover the whole video and get merged into one.
	assert.Len(t, segments, 1)
	assert.Equal(t, 0, segments[0].StartFrameIndex)
	assert.Equal(t, 2, segments[0].EndFrameIndex)
	assert.True(t, segments[0].Merged)
	assert.Equal(t, []int{0, 1}, segments[0].EventIndexes)
}

func TestPlanSegmentsShouldMergeOverlappingBufferedSegments(t *testing.T) {
	// Two close events whose buffered ranges overlap must produce a single merged segment.
	timestamps := []float64{0.0, 0.1, 0.2, 0.3, 0.4, 0.5}
	brightness := []float64{0.9, 0.1, 0.1, 0.1, 0.1, 0.9}

	frames := buildTestCollection(t, timestamps, brightness)

	// Separate events (non-consecutive hits).
	timeline, err := BuildTimeline(frames, []int{0, 5}, 0.05)
	assert.Nil(t, err)
	assert.Len(t, timeline.Events, 2)

	// With a 0.3s buffer the first segment reaches frame 3 and the second starts at frame 2: overlap.
	segments, err := PlanSegments(timeline, timestamps, 0.3, 0.3)
	assert.Nil(t, err)
	assert.Len(t, segments, 1)
	assert.True(t, segments[0].Merged)
	assert.Equal(t, 0, segments[0].StartFrameIndex)
	assert.Equal(t, 5, segments[0].EndFrameIndex)
}

func TestPlanSegmentsShouldKeepDistantSegmentsSeparate(t *testing.T) {
	timestamps := []float64{0.0, 0.1, 1.0, 1.1}
	brightness := []float64{0.9, 0.1, 0.1, 0.9}

	frames := buildTestCollection(t, timestamps, brightness)

	timeline, err := BuildTimeline(frames, []int{0, 3}, 0.05)
	assert.Nil(t, err)

	segments, err := PlanSegments(timeline, timestamps, 0.05, 0.05)
	assert.Nil(t, err)
	assert.Len(t, segments, 2)
	assert.False(t, segments[0].Merged)
	assert.False(t, segments[1].Merged)
	assert.Equal(t, 0, segments[0].StartFrameIndex)
	assert.Equal(t, 0, segments[0].EndFrameIndex)
	assert.Equal(t, 3, segments[1].StartFrameIndex)
	assert.Equal(t, 3, segments[1].EndFrameIndex)
}

func TestPlanSegmentsShouldReturnEmptyForEmptyTimeline(t *testing.T) {
	timestamps := []float64{0.0, 0.1, 0.2}
	timeline := Timeline{Events: []Event{}, FrameCount: 3}

	segments, err := PlanSegments(timeline, timestamps, 0.5, 0.5)
	assert.Nil(t, err)
	assert.Empty(t, segments)
}

func TestPlanSegmentsShouldRejectInvalidArguments(t *testing.T) {
	timestamps := []float64{0.0, 0.1, 0.2}
	timeline := Timeline{FrameCount: 3}

	_, err := PlanSegments(timeline, timestamps, -0.1, 0.0)
	assert.NotNil(t, err)

	_, err = PlanSegments(timeline, timestamps, 0.0, -0.1)
	assert.NotNil(t, err)

	_, err = PlanSegments(timeline, []float64{0.0}, 0.1, 0.1)
	assert.NotNil(t, err)
}
