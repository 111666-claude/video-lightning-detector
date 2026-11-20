package event

import (
	"os/exec"
	"path/filepath"
	"testing"
)

// ffmpegAvailable reports whether the ffmpeg/ffprobe toolchain is present so the
// video-backed integration tests can skip gracefully in restricted environments.
func ffmpegAvailable() bool {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return false
	}

	if _, err := exec.LookPath("ffprobe"); err != nil {
		return false
	}

	return true
}

// createSyntheticVideo renders a short constant frame-rate test video using the lavfi
// color source. The produced file is only a test fixture and lives under the test's own
// temporary directory.
func createSyntheticVideo(t *testing.T, directory, name string, frames int, fps float64) string {
	t.Helper()

	outputPath := filepath.Join(directory, name)
	duration := float64(frames) / fps

	cmd := exec.Command(
		"ffmpeg",
		"-loglevel", "error",
		"-y",
		"-f", "lavfi",
		"-i", "color=c=black:s=64x48:r="+floatToString(fps),
		"-t", floatToString(duration),
		"-c:v", "libx264",
		"-pix_fmt", "yuv420p",
		outputPath,
	)

	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to create synthetic video: %v: %s", err, string(output))
	}

	return outputPath
}

func floatToString(value float64) string {
	return formatSeconds(value)
}
