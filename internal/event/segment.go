package event

import (
	"fmt"
	"math"
)

// Segment describes a contiguous frame range to export as a video clip around an event.
// The range is expanded by a pre/post buffer measured in seconds of presentation time.
type Segment struct {
	// EventIndex references the event the segment was planned for. After overlap merging,
	// a single segment may cover multiple events; EventIndexes lists all of them.
	EventIndex int `json:"event-index"`
	// EventIndexes lists every event covered by the (possibly merged) segment.
	EventIndexes []int `json:"event-indexes"`
	// StartFrameIndex is the inclusive zero-based start frame after buffering.
	StartFrameIndex int `json:"start-frame-index"`
	// EndFrameIndex is the inclusive zero-based end frame after buffering.
	EndFrameIndex int `json:"end-frame-index"`
	// StartTime is the buffered segment start timestamp in seconds.
	StartTime float64 `json:"start-time"`
	// EndTime is the buffered segment end timestamp in seconds.
	EndTime float64 `json:"end-time"`
	// Merged reports whether the segment absorbed an overlapping neighbouring segment.
	Merged bool `json:"merged"`
}

// PlanSegments expands every event with a pre/post buffer (seconds) using the actual frame
// timestamps, clamps the ranges to the video bounds and merges overlapping or touching
// segments so no two exported clips contain the same frame range. The returned segments
// are ordered by their start frame and keep a deterministic event association.
func PlanSegments(timeline Timeline, frameTimestamps []float64, preBufferSeconds, postBufferSeconds float64) ([]Segment, error) {
	if preBufferSeconds < 0 || postBufferSeconds < 0 {
		return nil, fmt.Errorf("event: the segment buffer durations can not be negative")
	}

	frameCount := len(frameTimestamps)
	if frameCount != timeline.FrameCount {
		return nil, fmt.Errorf("event: the frame timestamps count %d does not match the timeline frame count %d", frameCount, timeline.FrameCount)
	}

	segments := make([]Segment, 0, len(timeline.Events))

	for _, ev := range timeline.Events {
		if ev.StartFrameIndex < 0 || ev.EndFrameIndex >= frameCount || ev.StartFrameIndex > ev.EndFrameIndex {
			return nil, fmt.Errorf("event: event %d references an invalid frame range [%d, %d] for frame count %d", ev.Index, ev.StartFrameIndex, ev.EndFrameIndex, frameCount)
		}

		startTime := math.Max(0, ev.StartTime-preBufferSeconds)
		endTime := ev.EndTime + postBufferSeconds

		startFrame := findFirstFrameAtOrAfter(frameTimestamps, startTime, ev.StartFrameIndex)
		endFrame := findLastFrameAtOrBefore(frameTimestamps, endTime, ev.EndFrameIndex, frameCount)

		segments = append(segments, Segment{
			EventIndex:      ev.Index,
			EventIndexes:    []int{ev.Index},
			StartFrameIndex: startFrame,
			EndFrameIndex:   endFrame,
			StartTime:       frameTimestamps[startFrame],
			EndTime:         frameTimestamps[endFrame],
			Merged:          false,
		})
	}

	return mergeOverlappingSegments(segments), nil
}

// mergeOverlappingSegments joins segments whose frame ranges overlap or touch. The input
// is expected to be ordered by event index which matches the timeline ordering.
func mergeOverlappingSegments(segments []Segment) []Segment {
	if len(segments) <= 1 {
		return segments
	}

	merged := make([]Segment, 0, len(segments))
	merged = append(merged, segments[0])

	for _, segment := range segments[1:] {
		previous := &merged[len(merged)-1]

		if segment.StartFrameIndex <= previous.EndFrameIndex+1 {
			if segment.EndFrameIndex > previous.EndFrameIndex {
				previous.EndFrameIndex = segment.EndFrameIndex
				previous.EndTime = segment.EndTime
			}

			previous.EventIndexes = append(previous.EventIndexes, segment.EventIndexes...)
			previous.Merged = true
		} else {
			merged = append(merged, segment)
		}
	}

	return merged
}

// findFirstFrameAtOrAfter returns the index of the first frame whose timestamp is greater
// than or equal to the target, scanning backwards from the event start as an upper bound.
func findFirstFrameAtOrAfter(timestamps []float64, target float64, upperBound int) int {
	for index := upperBound; index > 0; index -= 1 {
		if timestamps[index-1] < target {
			return index
		}
	}

	return 0
}

// findLastFrameAtOrBefore returns the index of the last frame whose timestamp is less than
// or equal to the target, scanning forwards from the event end as a lower bound.
func findLastFrameAtOrBefore(timestamps []float64, target float64, lowerBound, frameCount int) int {
	for index := lowerBound; index+1 < frameCount; index += 1 {
		if timestamps[index+1] > target {
			return index
		}
	}

	return frameCount - 1
}
