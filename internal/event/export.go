package event

import (
	"fmt"
	"os/exec"
	"path"
	"strings"
)

const ffmpegBinaryName string = "ffmpeg"

// ClipResult reports the outcome of exporting a single event segment clip.
type ClipResult struct {
	Segment Segment `json:"segment"`
	// OutputPath is the file path of the exported clip.
	OutputPath string `json:"output-path"`
	// Exported reports whether the clip file was produced successfully.
	Exported bool `json:"exported"`
	// Error holds the failure message when Exported is false.
	Error string `json:"error,omitempty"`
}

// ClipExporter exports planned event segments as re-encoded video clips using ffmpeg.
type ClipExporter interface {
	ExportClips(inputVideoPath, outputDirectoryPath string, segments []Segment) ([]ClipResult, error)
}

type clipExporter struct {
	// FileExtension is the container extension of the produced clips (defaults to mp4).
	FileExtension string
}

// NewClipExporter creates a clip exporter. The mp4 container with re-encoded H.264 video
// is used so that cuts land on exact frame boundaries regardless of the source keyframes.
func NewClipExporter() ClipExporter {
	return &clipExporter{FileExtension: "mp4"}
}

// ExportClips exports every segment as an independent clip. A failure of one segment does
// not abort the remaining ones; the per-segment outcome is returned together with an
// aggregate error describing the failed exports.
func (exporter *clipExporter) ExportClips(inputVideoPath, outputDirectoryPath string, segments []Segment) ([]ClipResult, error) {
	if strings.HasPrefix(inputVideoPath, "-") {
		return nil, fmt.Errorf("event: the input video file path format is invalid")
	}

	if len(outputDirectoryPath) == 0 {
		return nil, fmt.Errorf("event: the output directory path can not be empty")
	}

	results := make([]ClipResult, 0, len(segments))
	failures := make([]string, 0)

	for _, segment := range segments {
		outputPath := exporter.clipPath(outputDirectoryPath, segment)

		// NOTE: -ss before -i enables fast input seeking while re-encoding guarantees frame
		// accurate cuts. -to is expressed relative to the start of the input.
		cmd := exec.Command(
			ffmpegBinaryName,
			"-loglevel", "error",
			"-y",
			"-ss", formatSeconds(segment.StartTime),
			"-to", formatSeconds(segment.EndTime),
			"-i", inputVideoPath,
			"-map", "0:v:0",
			"-c:v", "libx264",
			"-preset", "veryfast",
			"-pix_fmt", "yuv420p",
			"-an",
			outputPath,
		)

		result := ClipResult{
			Segment:    segment,
			OutputPath: outputPath,
			Exported:   false,
		}

		if output, err := cmd.CombinedOutput(); err != nil {
			message := fmt.Sprintf("event: ffmpeg clip export failed for segment starting at frame %d: %s", segment.StartFrameIndex, strings.TrimSpace(string(output)))
			result.Error = message
			failures = append(failures, message)
		} else {
			result.Exported = true
		}

		results = append(results, result)
	}

	if len(failures) > 0 {
		return results, fmt.Errorf("%s", strings.Join(failures, "; "))
	}

	return results, nil
}

func (exporter *clipExporter) clipPath(outputDirectoryPath string, segment Segment) string {
	extension := exporter.FileExtension
	if len(extension) == 0 {
		extension = "mp4"
	}

	fileName := fmt.Sprintf("event-%02d_frames-%06d-%06d.%s", segment.EventIndex+1, segment.StartFrameIndex+1, segment.EndFrameIndex+1, extension)
	return path.Join(outputDirectoryPath, fileName)
}

func formatSeconds(value float64) string {
	return fmt.Sprintf("%.6f", value)
}
