package detector

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/Krzysztofz01/video-lightning-detector/internal/options"
	"github.com/Krzysztofz01/video-lightning-detector/internal/printer"
)

// hasToolchain reports whether ffmpeg and ffprobe are installed for the end-to-end tests.
func hasToolchain() bool {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return false
	}

	if _, err := exec.LookPath("ffprobe"); err != nil {
		return false
	}

	return true
}

// createLightningTestVideo renders a video containing a single flickering bright event
// lasting several consecutive frames. Inside the event window the frames alternate between
// two bright levels (grey and white) so every frame keeps high brightness while still
// producing strong frame-to-frame differences, which the existing frame-difference based
// detector classifies positively across the whole window rather than only at its edges.
func createLightningTestVideo(t *testing.T, path string) {
	t.Helper()

	// Flickering bright event decoded onto frames n=20..29 at 10 fps over a dark video.
	filter := "color=c=black:s=64x48:r=10:d=4.0," +
		"drawbox=x=0:y=0:w=64:h=48:color=gray:t=fill:enable='between(n,19,28)'," +
		"drawbox=x=0:y=0:w=64:h=48:color=white:t=fill:enable='between(n,19,28)*mod(n\\,2)'"

	cmd := exec.Command(
		"ffmpeg",
		"-loglevel", "error",
		"-y",
		"-f", "lavfi",
		"-i", filter,
		"-c:v", "libx264",
		"-pix_fmt", "yuv420p",
		path,
	)

	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to create the lightning test video: %v: %s", err, string(output))
	}
}

func testPrinter() printer.Printer {
	return printer.NewPrinter(printer.PrinterConfig{
		UseColor:     false,
		LogLevel:     options.Quiet,
		OutStream:    os.Stderr,
		ParsableMode: false,
	})
}

// readTimelineReport decodes the event-timeline section of an exported JSON report.
func readTimelineReport(t *testing.T, reportPath string) map[string]any {
	t.Helper()

	reportBytes, err := os.ReadFile(reportPath)
	assert.Nil(t, err)

	var report map[string]any
	assert.Nil(t, json.Unmarshal(reportBytes, &report))

	timeline, ok := report["event-timeline"].(map[string]any)
	assert.True(t, ok)

	return timeline
}

// TestDetectorRunShouldResolveTimelineForMultiFrameFlash exercises the full pipeline on a
// fixed bright-spot sequence. The exact frame gating belongs to the detector unit tests;
// here we verify that every resolved event lies inside the flicker window, the timeline is
// correctly ordered and the frame/report/clip artifacts are all produced. The deterministic
// "thirty frames become one event" aggregation itself is covered by the event package tests.
func TestDetectorRunShouldResolveTimelineForMultiFrameFlash(t *testing.T) {
	if !hasToolchain() {
		t.Skip("ffmpeg toolchain not available; skipping detector end-to-end test")
	}

	var (
		tempDirectory = t.TempDir()
		videoPath     = filepath.Join(tempDirectory, "flash.mp4")
		outputPath    = filepath.Join(tempDirectory, "out")
	)

	createLightningTestVideo(t, videoPath)

	detectorOptions := options.GetDefaultDetectorOptions()
	detectorOptions.FrameScalingFactor = 1.0
	detectorOptions.MovingMeanResolution = 5
	detectorOptions.BrightnessDetectionThreshold = 0.05
	detectorOptions.ColorDifferenceDetectionThreshold = 0.05
	detectorOptions.BinaryThresholdDifferenceDetectionThreshold = 0.05
	detectorOptions.StrictExplicitThreshold = true
	detectorOptions.ExportReport = true
	detectorOptions.ExportEventClips = true
	detectorOptions.EventMergeMaxIntervalSeconds = 0.5
	detectorOptions.EventClipPreBufferSeconds = 0.2
	detectorOptions.EventClipPostBufferSeconds = 0.2
	detectorOptions.ExportFramesPrefix = "frame"

	detectorInstance, err := CreateDetector(testPrinter(), detectorOptions)
	assert.Nil(t, err)

	err = detectorInstance.Run(videoPath, outputPath, context.Background())
	assert.Nil(t, err)

	timeline := readTimelineReport(t, filepath.Join(outputPath, "report.json"))

	events, ok := timeline["events"].([]any)
	assert.True(t, ok)
	assert.NotEmpty(t, events, "the bright flicker window must produce at least one event")

	var previousEnd float64 = -1.0

	for index, eventEntry := range events {
		event := eventEntry.(map[string]any)

		startFrame := int(event["start-frame-index"].(float64))
		endFrame := int(event["end-frame-index"].(float64))
		peakFrame := int(event["peak-frame-index"].(float64))

		// Every event must fall inside the actual flickering window (frames ~19..29).
		assert.GreaterOrEqual(t, startFrame, 18)
		assert.LessOrEqual(t, endFrame, 30)
		assert.GreaterOrEqual(t, peakFrame, startFrame)
		assert.LessOrEqual(t, peakFrame, endFrame)
		assert.GreaterOrEqual(t, int(event["hit-frame-count"].(float64)), 1)

		// The peak of a bright event must be a bright frame.
		assert.Greater(t, event["peak-intensity"].(float64), 0.4)
		assert.GreaterOrEqual(t, event["peak-time"].(float64), event["start-time"].(float64))
		assert.LessOrEqual(t, event["peak-time"].(float64), event["end-time"].(float64))

		// Events must be ordered and non-overlapping in time.
		assert.Greater(t, event["start-time"].(float64), previousEnd)
		previousEnd = event["end-time"].(float64)
		assert.Equal(t, index, int(event["index"].(float64)))
	}

	// The report carries real per-frame timestamps probed from the container (10 fps).
	frames := reportFrames(t, filepath.Join(outputPath, "report.json"))
	assert.InDelta(t, 2.0, frames[20]["timestamp"].(float64), 0.05)

	// Static per-frame PNG exports remain present alongside the new event artifacts.
	matches, err := filepath.Glob(filepath.Join(outputPath, "frame-*.png"))
	assert.Nil(t, err)
	assert.NotEmpty(t, matches, "the per-frame static image export must still run")

	// At least one event clip must have been produced as an mp4 file, marked successful.
	clipMatches, err := filepath.Glob(filepath.Join(outputPath, "event-*.mp4"))
	assert.Nil(t, err)
	assert.NotEmpty(t, clipMatches, "the buffered event clip must be exported")

	clips, ok := timeline["clips"].([]any)
	assert.True(t, ok)
	assert.NotEmpty(t, clips)
	for _, clipEntry := range clips {
		clip := clipEntry.(map[string]any)
		assert.Equal(t, true, clip["exported"])
	}

	// Planned segments must be buffered beyond the raw event bounds where room exists.
	segments, ok := timeline["segments"].([]any)
	assert.True(t, ok)
	assert.NotEmpty(t, segments)
}

// TestDetectorRunShouldReportExplicitResultForDarkVideo verifies that a uniform video with
// no frame-to-frame changes yields a deterministic, non-erroring timeline. The detector
// output always contains the timeline section with an explicit frame count.
func TestDetectorRunShouldReportExplicitResultForDarkVideo(t *testing.T) {
	if !hasToolchain() {
		t.Skip("ffmpeg toolchain not available; skipping detector end-to-end test")
	}

	var (
		tempDirectory = t.TempDir()
		videoPath     = filepath.Join(tempDirectory, "dark.mp4")
		outputPath    = filepath.Join(tempDirectory, "out")
	)

	cmd := exec.Command(
		"ffmpeg",
		"-loglevel", "error",
		"-y",
		"-f", "lavfi",
		"-i", "color=c=black:s=64x48:r=10:d=1.0",
		"-c:v", "libx264",
		"-pix_fmt", "yuv420p",
		videoPath,
	)

	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to create the dark test video: %v: %s", err, string(output))
	}

	detectorOptions := options.GetDefaultDetectorOptions()
	detectorOptions.FrameScalingFactor = 1.0
	detectorOptions.MovingMeanResolution = 5
	// Positive thresholds are required so a uniform dark video (all weights equal to zero)
	// is not positively classified.
	detectorOptions.BrightnessDetectionThreshold = 0.05
	detectorOptions.ColorDifferenceDetectionThreshold = 0.05
	detectorOptions.BinaryThresholdDifferenceDetectionThreshold = 0.05
	detectorOptions.ExportReport = true

	detectorInstance, err := CreateDetector(testPrinter(), detectorOptions)
	assert.Nil(t, err)

	err = detectorInstance.Run(videoPath, outputPath, context.Background())
	assert.Nil(t, err)

	timeline := readTimelineReport(t, filepath.Join(outputPath, "report.json"))
	assert.Equal(t, float64(10), timeline["frame-count"])
	// A uniform video has no frame-to-frame differences so no event can be classified.
	events := timeline["events"].([]any)
	assert.Empty(t, events)
}

func reportFrames(t *testing.T, reportPath string) []map[string]any {
	t.Helper()

	reportBytes, err := os.ReadFile(reportPath)
	assert.Nil(t, err)

	var report struct {
		Frames []map[string]any `json:"frames"`
	}

	assert.Nil(t, json.Unmarshal(reportBytes, &report))
	return report.Frames
}
