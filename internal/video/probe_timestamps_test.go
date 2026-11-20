package video

import (
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func toolchainAvailable() bool {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return false
	}

	if _, err := exec.LookPath("ffprobe"); err != nil {
		return false
	}

	return true
}

func createProbeTestVideo(t *testing.T, directory, name string, frames int, fps float64) string {
	t.Helper()

	outputPath := filepath.Join(directory, name)
	cmd := exec.Command(
		"ffmpeg",
		"-loglevel", "error",
		"-y",
		"-f", "lavfi",
		"-i", "color=c=black:s=64x48:r=10",
		"-frames:v", "30",
		"-c:v", "libx264",
		"-pix_fmt", "yuv420p",
		outputPath,
	)

	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to create probe test video: %v: %s", err, string(output))
	}

	return outputPath
}

func TestProbeFrameTimestampsShouldReturnRealPresentationTimes(t *testing.T) {
	if !toolchainAvailable() {
		t.Skip("ffmpeg toolchain not available; skipping frame timestamp probe integration test")
	}

	videoPath := createProbeTestVideo(t, t.TempDir(), "probe.mp4", 30, 10.0)

	timestamps, err := ProbeFrameTimestamps(videoPath)
	assert.Nil(t, err)
	assert.Len(t, timestamps, 30)
	assert.InDelta(t, 0.0, timestamps[0], 1e-6)

	// The timestamps must be strictly increasing and spaced around the 10 fps frame period.
	for index := 1; index < len(timestamps); index += 1 {
		assert.Greater(t, timestamps[index], timestamps[index-1])
		assert.InDelta(t, 0.1, timestamps[index]-timestamps[index-1], 0.02)
	}
}

func TestProbeFrameTimestampsShouldFailForMissingFile(t *testing.T) {
	if !toolchainAvailable() {
		t.Skip("ffmpeg toolchain not available; skipping frame timestamp probe integration test")
	}

	_, err := ProbeFrameTimestamps(filepath.Join(t.TempDir(), "missing.mp4"))
	assert.NotNil(t, err)
}
