package frame

import (
	"fmt"
	"image"
)

const (
	BinaryThresholdParam float64 = 200.0 / 255.0
)

// Strucutre representing a single video frame and its calculated parameters.
type Frame struct {
	OrdinalNumber             int     `json:"ordinal-number"`
	ColorDifference           float64 `json:"color-difference"`
	BinaryThresholdDifference float64 `json:"binary-threshold-difference"`
	Brightness                float64 `json:"brightness"`
	// Timestamp is the presentation time of the frame in seconds relative to the
	// beginning of the video. It is derived from the actual per-frame presentation
	// timestamps probed from the container, so variable frame-rate material is handled
	// correctly. A zero value means the timestamp has not been resolved.
	Timestamp float64 `json:"timestamp"`
}

// Factory used to create frame instances. The factory controlls the ordinal numbers which are 1-indexed.
type FrameFactory interface {
	CreateNewFrame(frameImage0, frameImage1 *image.RGBA, timestamp float64) (*Frame, error)
}

type frameFactory struct {
	BinaryThresholdParam float64
	OrdinalNumber        int
}

// Create a new frame based on the N and N-1 frame images. The timestamp is the frame
// presentation time in seconds relative to the beginning of the video.
func (ff *frameFactory) CreateNewFrame(frameImage0, frameImage1 *image.RGBA, timestamp float64) (*Frame, error) {
	if frameImage0 == nil {
		return nil, fmt.Errorf("frame: invalid current frame image nil reference")
	}

	if ff.OrdinalNumber > 1 && frameImage1 == nil {
		return nil, fmt.Errorf("frame: invalid previous frame image reference in context of ordinal number")
	}

	if timestamp < 0 {
		return nil, fmt.Errorf("frame: the frame timestamp can not be negative")
	}

	var (
		pf = processFrame(frameImage0, frameImage1, ff.OrdinalNumber, ff.BinaryThresholdParam)
		f  = &Frame{
			OrdinalNumber:             ff.OrdinalNumber,
			ColorDifference:           pf.ColorDifference,
			BinaryThresholdDifference: pf.BinaryThresholdDifference,
			Brightness:                pf.Brightness,
			Timestamp:                 timestamp,
		}
	)

	ff.OrdinalNumber += 1
	return f, nil
}

// Create a new frame factory instance with the specified binary threshold argument used for segmentation operations.
func CreateFrameFactory(binaryThresholdParam float64) FrameFactory {
	return &frameFactory{
		BinaryThresholdParam: binaryThresholdParam,
		OrdinalNumber:        1,
	}
}
