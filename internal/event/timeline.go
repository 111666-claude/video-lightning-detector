package event

import (
	"fmt"
	"slices"

	"github.com/Krzysztofz01/video-lightning-detector/internal/frame"
)

// Event is a single lightning occurrence aggregated from one or more consecutive (or
// nearby) positively classified frames. A strike lasting thirty frames is represented as
// one event instead of thirty independent frame detections.
type Event struct {
	// Index is the zero-based position of the event within the resolved timeline.
	Index int `json:"index"`
	// StartFrameIndex is the zero-based collection index of the first hit frame.
	StartFrameIndex int `json:"start-frame-index"`
	// EndFrameIndex is the zero-based collection index of the last hit frame.
	EndFrameIndex int `json:"end-frame-index"`
	// PeakFrameIndex is the zero-based collection index of the brightest hit frame.
	PeakFrameIndex int `json:"peak-frame-index"`
	// StartTime is the presentation timestamp (seconds) of the first hit frame.
	StartTime float64 `json:"start-time"`
	// EndTime is the presentation timestamp (seconds) of the last hit frame.
	EndTime float64 `json:"end-time"`
	// PeakTime is the presentation timestamp (seconds) of the peak frame.
	PeakTime float64 `json:"peak-time"`
	// PeakIntensity is the brightness value of the peak frame.
	PeakIntensity float64 `json:"peak-intensity"`
	// HitFrameCount is the number of positively classified frames aggregated into the event.
	HitFrameCount int `json:"hit-frame-count"`
}

// Timeline holds the lightning events resolved from a detection run together with
// diagnostics about the frame timestamps the merging was based on.
type Timeline struct {
	Events []Event `json:"events"`
	// FrameCount is the total number of analyzed frames.
	FrameCount int `json:"frame-count"`
	// NonMonotonicTimestampCount reports how many frames had a duplicated or backwards
	// presentation timestamp. Such frames are merged using a zero time gap.
	NonMonotonicTimestampCount int `json:"non-monotonic-timestamp-count"`
}

// BuildTimeline aggregates the positively classified frame indexes (zero-based collection
// indexes) into events using the actual frame presentation timestamps. Two consecutive hit
// frames belong to the same event while the time gap between them does not exceed
// maxIntervalSeconds, so variable frame-rate material is merged using real elapsed time.
//
// The detection indexes are defensively sorted and de-duplicated and every index is
// validated against the frame collection, guaranteeing a stable, deterministic event
// ordering regardless of the input slice order.
func BuildTimeline(frames frame.FrameCollection, detections []int, maxIntervalSeconds float64) (Timeline, error) {
	if maxIntervalSeconds < 0 {
		return Timeline{}, fmt.Errorf("event: the maximum event merge interval can not be negative")
	}

	if frames == nil {
		return Timeline{}, fmt.Errorf("event: the frame collection reference is nil")
	}

	allFrames := frames.GetAll()
	frameCount := len(allFrames)

	timeline := Timeline{
		Events:     make([]Event, 0),
		FrameCount: frameCount,
	}

	// NOTE: Defensive copy so the caller's slice is never reordered.
	hitIndexes := append(make([]int, 0, len(detections)), detections...)
	slices.Sort(hitIndexes)
	hitIndexes = slices.Compact(hitIndexes)

	for _, index := range hitIndexes {
		if index < 0 || index >= frameCount {
			return Timeline{}, fmt.Errorf("event: detection frame index %d is out of the frame collection range [0, %d)", index, frameCount)
		}
	}

	for index := 1; index < frameCount; index += 1 {
		if allFrames[index].Timestamp <= allFrames[index-1].Timestamp {
			timeline.NonMonotonicTimestampCount += 1
		}
	}

	var current *Event

	for _, index := range hitIndexes {
		f := allFrames[index]

		if current == nil {
			current = newEvent(len(timeline.Events), index, f)
			continue
		}

		previousFrame := allFrames[index-1]
		gap := f.Timestamp - previousFrame.Timestamp
		if gap < 0 {
			// NOTE: Duplicated/backwards timestamps can not express a positive gap, so the
			// frames are treated as simultaneous and stay within the same event.
			gap = 0
		}

		// NOTE: Only consecutive hit frames can be merged. A gap in the hit sequence always
		// starts a new event even if the timestamps happen to be close.
		if index == current.EndFrameIndex+1 && gap <= maxIntervalSeconds {
			extendEvent(current, index, f)
		} else {
			timeline.Events = append(timeline.Events, *current)
			current = newEvent(len(timeline.Events), index, f)
		}
	}

	if current != nil {
		timeline.Events = append(timeline.Events, *current)
	}

	return timeline, nil
}

func newEvent(index, frameIndex int, f *frame.Frame) *Event {
	return &Event{
		Index:           index,
		StartFrameIndex: frameIndex,
		EndFrameIndex:   frameIndex,
		PeakFrameIndex:  frameIndex,
		StartTime:       f.Timestamp,
		EndTime:         f.Timestamp,
		PeakTime:        f.Timestamp,
		PeakIntensity:   f.Brightness,
		HitFrameCount:   1,
	}
}

func extendEvent(event *Event, frameIndex int, f *frame.Frame) {
	event.EndFrameIndex = frameIndex
	event.EndTime = f.Timestamp
	event.HitFrameCount += 1

	// NOTE: On equal brightness the earlier frame keeps the peak so the result is stable.
	if f.Brightness > event.PeakIntensity {
		event.PeakIntensity = f.Brightness
		event.PeakFrameIndex = frameIndex
		event.PeakTime = f.Timestamp
	}
}
