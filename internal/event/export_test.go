package event

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExportClipsShouldProduceClipFiles(t *testing.T) {
	if !ffmpegAvailable() {
		t.Skip("ffmpeg toolchain not available; skipping clip export integration test")
	}

	directory := t.TempDir()
	videoPath := createSyntheticVideo(t, directory, "source.mp4", 30, 10.0)

	segments := []Segment{
		{
			EventIndex:      0,
			EventIndexes:    []int{0},
			StartFrameIndex: 2,
			EndFrameIndex:   8,
			StartTime:       0.2,
			EndTime:         0.8,
		},
		{
			EventIndex:      1,
			EventIndexes:    []int{1},
			StartFrameIndex: 20,
			EndFrameIndex:   26,
			StartTime:       2.0,
			EndTime:         2.6,
		},
	}

	exporter := NewClipExporter()
	results, err := exporter.ExportClips(videoPath, directory, segments)

	assert.Nil(t, err)
	assert.Len(t, results, 2)

	for _, result := range results {
		assert.True(t, result.Exported, "expected clip to be exported: %s", result.Error)
		assert.Empty(t, result.Error)

		info, statErr := os.Stat(result.OutputPath)
		assert.Nil(t, statErr)
		assert.Greater(t, info.Size(), int64(0))
		assert.Equal(t, filepath.Ext(result.OutputPath), ".mp4")
	}
}

func TestExportClipsShouldReportFailureForInvalidInput(t *testing.T) {
	if !ffmpegAvailable() {
		t.Skip("ffmpeg toolchain not available; skipping clip export integration test")
	}

	directory := t.TempDir()

	segments := []Segment{
		{
			EventIndex:      0,
			EventIndexes:    []int{0},
			StartFrameIndex: 0,
			EndFrameIndex:   5,
			StartTime:       0.0,
			EndTime:         0.5,
		},
	}

	exporter := NewClipExporter()

	// A non-existent input file makes ffmpeg fail; the result must be explicit per segment.
	results, err := exporter.ExportClips(filepath.Join(directory, "missing.mp4"), directory, segments)

	assert.NotNil(t, err)
	assert.Len(t, results, 1)
	assert.False(t, results[0].Exported)
	assert.NotEmpty(t, results[0].Error)
}

func TestExportClipsShouldRejectInvalidPath(t *testing.T) {
	exporter := NewClipExporter()

	_, err := exporter.ExportClips("-malicious-path", t.TempDir(), []Segment{{EventIndex: 0}})
	assert.NotNil(t, err)

	_, err = exporter.ExportClips("source.mp4", "", []Segment{{EventIndex: 0}})
	assert.NotNil(t, err)
}
