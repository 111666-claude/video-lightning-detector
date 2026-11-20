package video

import (
	"bufio"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// ProbeFrameTimestamps resolves the presentation timestamp (in seconds, relative to the
// start of the video) of every frame using ffprobe. The timestamps come straight from the
// container, so both constant and variable frame-rate material is handled using the real
// timing. The returned slice is ordered the same way as the decoded frames.
func ProbeFrameTimestamps(path string) ([]float64, error) {
	cmd := exec.Command(
		ffprobeBinaryName,
		"-select_streams", "v:0",
		"-loglevel", "quiet",
		"-show_entries", "frame=pts_time",
		"-of", "csv=p=0",
		path,
	)

	stdout, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("video: failed to probe the frame presentation timestamps: %w", err)
	}

	timestamps, err := parseFrameTimestamps(stdout)
	if err != nil {
		return nil, fmt.Errorf("video: failed to parse the frame presentation timestamps: %w", err)
	}

	if len(timestamps) == 0 {
		return nil, fmt.Errorf("video: the frame presentation timestamps probe returned no entries")
	}

	return timestamps, nil
}

// SynthesizeFrameTimestamps builds evenly spaced timestamps using the given frame rate.
// It is used as a fallback when the container does not expose per-frame presentation
// timestamps. A non-positive frame rate produces an error.
func SynthesizeFrameTimestamps(frameCount int, fps float64) ([]float64, error) {
	if frameCount < 0 {
		return nil, fmt.Errorf("video: the frame count can not be negative")
	}

	if fps <= 0 {
		return nil, fmt.Errorf("video: the frame rate must be greater than zero to synthesize timestamps")
	}

	timestamps := make([]float64, frameCount)
	for index := range timestamps {
		timestamps[index] = float64(index) / fps
	}

	return timestamps, nil
}

// parseFrameTimestamps parses the ffprobe CSV output where each non-empty line holds a
// single pts_time value. Entries without a resolved timestamp ("N/A") are skipped.
func parseFrameTimestamps(stdout []byte) ([]float64, error) {
	reader := strings.NewReader(string(stdout))
	scanner := bufio.NewScanner(reader)
	scanner.Split(bufio.ScanLines)

	timestamps := make([]float64, 0, 64)

	for scanner.Scan() {
		// NOTE: ffprobe CSV output can contain a trailing field separator even with a single
		// requested entry, so only the first comma-separated field is taken.
		line := strings.TrimSpace(scanner.Text())
		if commaIndex := strings.Index(line, ","); commaIndex >= 0 {
			line = strings.TrimSpace(line[:commaIndex])
		}

		if len(line) == 0 || line == string(notApplicable) {
			continue
		}

		timestamp, err := strconv.ParseFloat(line, 64)
		if err != nil {
			return nil, fmt.Errorf("video: failed to parse the frame presentation timestamp value %q: %w", line, err)
		}

		timestamps = append(timestamps, timestamp)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("video: failed to read the frame presentation timestamps: %w", err)
	}

	return timestamps, nil
}
