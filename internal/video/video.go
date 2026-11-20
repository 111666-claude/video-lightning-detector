package video

import (
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/Krzysztofz01/video-lightning-detector/internal/options"
	"github.com/Krzysztofz01/video-lightning-detector/internal/utils"
)

// FIXME: Add support for hardware accelerated decoding

type Video interface {
	GetInputDimensions() (int, int)
	GetOutputDimensions() (int, int)
	SetScale(s float64) error
	SetScaleAlgorithm(a options.ScaleAlgorithm) error
	SetBbox(x, y, w, h int) error
	SetTargetFrames(n ...int) error
	FramesCountApprox() int
	FrameRate() float64
	GetFrameTimestamps(frameCount int) ([]float64, error)
	SetFrameBuffer(buffer []byte) error
	Read() error
	Close()
}

type video struct {
	FilePath       string
	Dim            utils.Vec2i
	BboxAnchor     utils.Vec2i
	BboxDim        utils.Vec2i
	Scale          float64
	ScaleAlgorithm options.ScaleAlgorithm
	TargetFrames   []int
	Duration       float64
	Fps            float64
	FramesCount    int
	FrameBuffer    []byte
	Process        *exec.Cmd
	Pipe           io.ReadCloser
}

func (v *video) GetInputDimensions() (int, int) {
	return v.Dim.X, v.Dim.Y
}

func (v *video) GetScaledDimensions() (int, int) {
	wF := float64(v.Dim.X) * v.Scale
	hF := float64(v.Dim.Y) * v.Scale

	return int(wF), int(hF)
}

func (v *video) GetOutputBbox() (int, int, int, int) {
	xF := float64(v.BboxAnchor.X) * v.Scale
	yF := float64(v.BboxAnchor.Y) * v.Scale
	wF := float64(v.BboxDim.X) * v.Scale
	hF := float64(v.BboxDim.Y) * v.Scale

	return int(xF), int(yF), int(wF), int(hF)
}

func (v *video) GetOutputDimensions() (int, int) {
	_, _, w, h := v.GetOutputBbox()

	return w, h
}

func (v *video) SetScaleAlgorithm(a options.ScaleAlgorithm) error {
	if v.IsInitialized() {
		return fmt.Errorf("video: can not change scale algorithm after initialization")
	}

	if !options.IsValidScaleAlgorithm(a) {
		return fmt.Errorf("video: the specified scale algorithm is invalid")
	}

	v.ScaleAlgorithm = a
	return nil
}

func (v *video) SetScale(s float64) error {
	if v.IsInitialized() {
		return fmt.Errorf("video: can not change scale after initialization")
	}

	if s <= 0 || s > 1 {
		return fmt.Errorf("video: provided scale factor is out of range")
	}

	v.Scale = s
	return nil
}

func (v *video) SetBbox(x, y, w, h int) error {
	if v.IsInitialized() {
		return fmt.Errorf("video: can not change bbox after initialization")
	}

	if w <= 0 || h <= 0 {
		return fmt.Errorf("video: the video bbox sizes can not be negative or zero")
	}

	if x < 0 || x >= v.Dim.X || y < 0 || y >= v.Dim.Y {
		return fmt.Errorf("video: the video bbox anchor is out of the video range")
	}

	if x+w >= v.Dim.X {
		return fmt.Errorf("video: the video bbox horizontaly overflows the video bounds")
	}

	if y+h >= v.Dim.Y {
		return fmt.Errorf("video: the video bbox verticaly overflows the video bounds")
	}

	v.BboxAnchor = utils.Vec2i{X: x, Y: y}
	v.BboxDim = utils.Vec2i{X: w, Y: h}
	return nil
}

func (v *video) SetTargetFrames(n ...int) error {
	if v.IsInitialized() {
		return fmt.Errorf("video: can not change target frames after initialization")
	}

	if len(n) == 0 {
		return fmt.Errorf("video: no frames indexes were specified to extract")
	}

	for _, nVal := range n {
		if nVal >= v.FramesCount {
			return fmt.Errorf("video: the provided frame indexes are not in the frame count range")
		}
	}

	v.TargetFrames = n
	return nil
}

func (v *video) IsBboxUsed() bool {
	return v.Dim.X != v.BboxDim.X || v.Dim.Y != v.BboxDim.Y
}

func (v *video) FramesCountApprox() int {
	return v.FramesCount
}

func (v *video) FrameRate() float64 {
	return v.Fps
}

// GetFrameTimestamps returns the presentation timestamp (seconds) for each decoded frame.
// It first attempts to read the real per-frame timestamps from the container (required for
// variable frame-rate material) and falls back to frame-rate based synthesis when probing
// is unavailable. When the decoded frame count differs from the probed timestamp count
// (common when the decoder duplicates frames to satisfy a constant output rate), the real
// timing is resampled onto the decoded frame positions while preserving the total duration.
// Duplicated or backwards timestamps are normalized into a monotonically non-decreasing
// sequence so downstream event merging never observes time travel.
func (v *video) GetFrameTimestamps(frameCount int) ([]float64, error) {
	if frameCount < 0 {
		return nil, fmt.Errorf("video: the frame count can not be negative")
	}

	timestamps, err := ProbeFrameTimestamps(v.FilePath)
	if err != nil {
		fallback, fallbackErr := SynthesizeFrameTimestamps(frameCount, v.Fps)
		if fallbackErr != nil {
			return nil, fmt.Errorf("video: failed to probe frame timestamps (%w) and the fallback failed: %w", err, fallbackErr)
		}

		return fallback, nil
	}

	return NormalizeFrameTimestamps(timestamps, frameCount, v.Fps), nil
}

// NormalizeFrameTimestamps guarantees that the returned slice has exactly frameCount
// entries and is monotonically non-decreasing.
//
// When the probed timestamp count matches the decoded frame count each probed value is used
// directly, clamping any backwards entry to the previous value while keeping duplicates.
// When the counts differ (decoder-inserted or dropped frames), the probed timing curve is
// resampled onto the decoded positions using linear interpolation, preserving the first and
// last presentation time and the real variable frame-rate timing in between.
func NormalizeFrameTimestamps(timestamps []float64, frameCount int, fps float64) []float64 {
	normalized := make([]float64, frameCount)

	if frameCount == 0 {
		return normalized
	}

	if len(timestamps) == frameCount {
		previous := 0.0
		for index := 0; index < frameCount; index += 1 {
			value := timestamps[index]
			if value < previous {
				value = previous
			}

			normalized[index] = value
			previous = value
		}

		return normalized
	}

	if len(timestamps) == 0 {
		return synthesizeOrReplicate(normalized, frameCount, fps)
	}

	// NOTE: Resample the probed presentation-time curve onto the decoded frame positions.
	firstTimestamp := timestamps[0]
	lastTimestamp := timestamps[len(timestamps)-1]

	if len(timestamps) == 1 || lastTimestamp <= firstTimestamp {
		for index := range normalized {
			normalized[index] = firstTimestamp
		}

		return normalized
	}

	scale := float64(len(timestamps)-1) / float64(frameCount-1)
	for index := 0; index < frameCount; index += 1 {
		position := float64(index) * scale
		lower := int(position)
		if lower >= len(timestamps)-1 {
			lower = len(timestamps) - 2
		}

		upper := lower + 1
		fraction := position - float64(lower)
		if fraction > 1.0 {
			fraction = 1.0
		}

		normalized[index] = timestamps[lower] + (timestamps[upper]-timestamps[lower])*fraction
	}

	return normalized
}

// synthesizeOrReplicate fills the destination slice when no usable probed timestamps exist.
// It uses the nominal frame rate when available, otherwise it holds a zero timestamp.
func synthesizeOrReplicate(destination []float64, frameCount int, fps float64) []float64 {
	if fps > 0 {
		for index := range destination {
			destination[index] = float64(index) / fps
		}
	}

	return destination
}

func (v *video) SetFrameBuffer(buffer []byte) error {
	if v.IsInitialized() {
		return fmt.Errorf("video: can not change the frame buffer after initialization")
	}

	w, h := v.GetOutputDimensions()
	size := w * h * frameChannelDepth

	if len(buffer) != size {
		return fmt.Errorf("video: the target buffer size of %d does not match the required buffer length of %d", len(buffer), size)
	}

	v.FrameBuffer = buffer
	return nil
}

func (v *video) Read() error {
	if !v.IsInitialized() {
		if err := v.Init(); err != nil {
			return fmt.Errorf("video: failed to initliaze frame reading video stream: %w", err)
		}
	}

	if _, err := io.ReadFull(v.Pipe, v.FrameBuffer); err == nil {
		return nil
	} else if errors.Is(err, io.EOF) {
		return io.EOF
	} else if errors.Is(err, io.ErrUnexpectedEOF) {
		return fmt.Errorf("video: failed to read the video frame data via the process pipe due to invalid data length")
	} else {
		return fmt.Errorf("video: failed to read the video frame data via the process pipe: %w", err)
	}
}

func (v *video) Close() {
	defer func() {
		if err := recover(); err != nil {
			v.Process = nil
			v.Pipe = nil
			v.FrameBuffer = nil
		}
	}()

	if v.Process != nil {
		if v.Process.ProcessState == nil || !v.Process.ProcessState.Exited() {
			v.Process.Process.Kill()
		}

		v.Process.Wait()
		v.Process = nil
	}

	if v.Pipe != nil {
		v.Pipe.Close()
		v.Pipe = nil
	}

	if v.FrameBuffer != nil {
		v.FrameBuffer = nil
	}
}

func (v *video) IsInitialized() bool {
	return v.Process != nil && v.Pipe != nil && v.FrameBuffer != nil
}

func (v *video) Init() error {
	if v.IsInitialized() {
		return fmt.Errorf("video: video reading process can not be reinitialized")
	}

	var (
		filters []string = make([]string, 0, 16)
		args    []string = make([]string, 0, 32)
	)

	if v.TargetFrames != nil {
		frames := strings.Builder{}
		for index, n := range v.TargetFrames {
			if index != 0 {
				frames.WriteRune('+')
			}

			frames.WriteString(fmt.Sprintf("eq(n\\,%d)", n))
		}

		filters = append(filters, fmt.Sprintf("select='%s'", frames.String()))
	}

	if v.Scale != 1 {
		w, h := v.GetScaledDimensions()

		var algorithm string
		switch v.ScaleAlgorithm {
		case options.Default:
		case options.Bilinear:
			algorithm = "bilinear"
		case options.Bicubic:
			algorithm = "bicubic"
		case options.NearestNeighbour:
			algorithm = "nearest"
		case options.Lanczos:
			algorithm = "lanczos"
		case options.Area:
			algorithm = "area"
		default:
			return fmt.Errorf("video: invalid corrupted scale algorithm value")
		}

		if v.ScaleAlgorithm == options.Default {
			filters = append(filters, fmt.Sprintf("scale=%d:%d", w, h))
		} else {
			filters = append(filters, fmt.Sprintf("scale=%d:%d:flags=%s", w, h, algorithm))
		}
	}

	if v.IsBboxUsed() {
		x, y, w, h := v.GetOutputBbox()
		filters = append(filters, fmt.Sprintf("crop=%d:%d:%d:%d", w, h, x, y))
	}

	args = append(args, "-protocol_whitelist", "file")
	args = append(args, "-i", v.FilePath)
	args = append(args, "-loglevel", "quiet")
	args = append(args, "-hide_banner")
	args = append(args, "-f", "image2pipe")
	args = append(args, "-pix_fmt", "rgba")
	args = append(args, "-vcodec", "rawvideo")
	args = append(args, "-map", "0:v:0")

	if len(filters) > 0 {
		filter := strings.Join(filters, ",")
		args = append(args, "-vf", filter)
	}

	if v.TargetFrames != nil {
		args = append(args, "-vsync", "0")
	}

	args = append(args, "-")

	cmd := exec.Command(ffmpegBinaryName, args...)

	pipe, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("video: failed to access the video reading process pipe: %w", err)
	}

	if v.FrameBuffer == nil {
		w, h := v.GetOutputDimensions()
		size := w * h * frameChannelDepth

		v.FrameBuffer = make([]byte, size)
	}

	v.Process = cmd
	v.Pipe = pipe

	if err := v.Process.Start(); err != nil {
		return fmt.Errorf("video: failed to start the video reading proces: %w", err)
	}

	return nil
}

func NewVideo(path string) (Video, error) {
	if strings.HasPrefix(path, "-") {
		return nil, fmt.Errorf("video: the video file path format is invalid")
	}

	if !utils.FileExists(path) {
		return nil, fmt.Errorf("video: the video file specified by the path does not exist")
	}

	if ok, err := AreBinariesAvailable(); !ok && err != nil {
		return nil, fmt.Errorf("video: the required video processing binaries are not available: %w", err)
	}

	probe, err := probeVideoFile(path)
	if err != nil {
		return nil, fmt.Errorf("video: failed to probe the target video file: %w", err)
	}

	if probe.Duration == 0 || probe.Frames == 0 {
		return nil, fmt.Errorf("video: failed to probe the target video duration or frames count")
	}

	return &video{
		FilePath:       path,
		Dim:            utils.Vec2i{X: probe.Width, Y: probe.Height},
		BboxAnchor:     utils.Vec2i{X: 0, Y: 0},
		BboxDim:        utils.Vec2i{X: probe.Width, Y: probe.Height},
		Scale:          1,
		Duration:       probe.Duration,
		Fps:            probe.Fps,
		FramesCount:    probe.Frames,
		FrameBuffer:    nil,
		Process:        nil,
		Pipe:           nil,
		ScaleAlgorithm: options.Default,
		TargetFrames:   nil,
	}, nil
}
