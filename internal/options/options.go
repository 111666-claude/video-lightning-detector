package options

import (
	"github.com/Krzysztofz01/video-lightning-detector/internal/utils"
)

// Structure representing the options for the detector.
type DetectorOptions struct {
	AutoThresholds                              bool
	BrightnessDetectionThreshold                float64
	ColorDifferenceDetectionThreshold           float64
	BinaryThresholdDifferenceDetectionThreshold float64
	MovingMeanResolution                        int32
	// Deprecated: The CSV export functionality will be removed in the future.
	ExportCsvReport bool
	// Deprecated: The JSON export specific flag will be removed and the ExportReport should be used instead.
	ExportJsonReport bool
	// Deprecated: The confusion matrix is exported by default and is depended on the presence of ConfusionMatrixActualDetectionsExpression
	ExportConfusionMatrix                     bool
	ExportReport                              bool
	ConfusionMatrixActualDetectionsExpression string
	SkipFramesExport                          bool
	Denoise                                   DenoiseAlgorithm
	FrameScalingFactor                        float64
	ImportPreanalyzed                         bool
	StrictExplicitThreshold                   bool
	DetectionBoundsExpression                 string
	ScaleAlgorithm                            ScaleAlgorithm
	ExportFramesPrefix                        string
	// EventMergeMaxIntervalSeconds is the maximum allowed real-time gap between two
	// consecutive hit frames for them to be aggregated into a single lightning event.
	EventMergeMaxIntervalSeconds float64
	// EventClipPreBufferSeconds expands each exported event clip backwards by this amount.
	EventClipPreBufferSeconds float64
	// EventClipPostBufferSeconds expands each exported event clip forwards by this amount.
	EventClipPostBufferSeconds float64
	// ExportEventClips enables exporting buffered video clips around the detected events.
	ExportEventClips bool
}

// Return a boolean value representing if the detector options are valid. If any validation errors occured
// a message will be stored in the string return value.
func (options *DetectorOptions) AreValid() (bool, string) {
	if options.StrictExplicitThreshold && (options.BrightnessDetectionThreshold < 0.0 || options.BrightnessDetectionThreshold > 1.0) {
		return false, "the frame brightness detection threshold must be between zero and one"
	}

	if options.StrictExplicitThreshold && (options.ColorDifferenceDetectionThreshold < 0.0 || options.ColorDifferenceDetectionThreshold > 1.0) {
		return false, "the frame color difference detection threshold must be between zero and one"
	}

	if options.StrictExplicitThreshold && (options.BinaryThresholdDifferenceDetectionThreshold < 0.0 || options.BinaryThresholdDifferenceDetectionThreshold > 1.0) {
		return false, "the frame binary threshold difference detection threshold must be between zero and one"
	}

	if options.FrameScalingFactor < 0.0 || options.FrameScalingFactor > 1.0 {
		return false, "the scaling factor must be between zero and one"
	}

	if options.ExportConfusionMatrix && len(options.ConfusionMatrixActualDetectionsExpression) == 0 {
		return false, "the confusion matrix actual detections expressions must be specified to export the confusion matrix"
	}

	if len(options.ConfusionMatrixActualDetectionsExpression) != 0 && !utils.IsRangeExpressionValid(options.ConfusionMatrixActualDetectionsExpression) {
		return false, "the confusion matrix actual detections expression has a invalid format"
	}

	if options.MovingMeanResolution < 0 {
		return false, "the moving mean/stddev resolution can not be negative"
	}

	if !IsValidDenoiseAlgorithm(options.Denoise) {
		return false, "the specified denoise algorithm is invalid"
	}

	if len(options.DetectionBoundsExpression) != 0 && !utils.IsBoundsExpressionValid(options.DetectionBoundsExpression) {
		return false, "the detection bounds expression has a invalid format"
	}

	if !IsValidScaleAlgorithm(options.ScaleAlgorithm) {
		return false, "the specified scale algorithm is invalid"
	}

	if options.EventMergeMaxIntervalSeconds < 0 {
		return false, "the event merge maximum interval can not be negative"
	}

	if options.EventClipPreBufferSeconds < 0 || options.EventClipPostBufferSeconds < 0 {
		return false, "the event clip buffer durations can not be negative"
	}

	return true, ""
}

// Create a copy of the detector option
func (options *DetectorOptions) Clone() DetectorOptions {
	return DetectorOptions{
		AutoThresholds:                              options.AutoThresholds,
		BrightnessDetectionThreshold:                options.BrightnessDetectionThreshold,
		ColorDifferenceDetectionThreshold:           options.ColorDifferenceDetectionThreshold,
		BinaryThresholdDifferenceDetectionThreshold: options.BinaryThresholdDifferenceDetectionThreshold,
		MovingMeanResolution:                        options.MovingMeanResolution,
		ExportCsvReport:                             options.ExportCsvReport,
		ExportJsonReport:                            options.ExportJsonReport,
		ExportConfusionMatrix:                       options.ExportConfusionMatrix,
		ExportReport:                                options.ExportReport,
		ConfusionMatrixActualDetectionsExpression:   options.ConfusionMatrixActualDetectionsExpression,
		SkipFramesExport:                            options.SkipFramesExport,
		Denoise:                                     options.Denoise,
		FrameScalingFactor:                          options.FrameScalingFactor,
		ImportPreanalyzed:                           options.ImportPreanalyzed,
		StrictExplicitThreshold:                     options.StrictExplicitThreshold,
		DetectionBoundsExpression:                   options.DetectionBoundsExpression,
		ScaleAlgorithm:                              options.ScaleAlgorithm,
		ExportFramesPrefix:                          options.ExportFramesPrefix,
		EventMergeMaxIntervalSeconds:                options.EventMergeMaxIntervalSeconds,
		EventClipPreBufferSeconds:                   options.EventClipPreBufferSeconds,
		EventClipPostBufferSeconds:                  options.EventClipPostBufferSeconds,
		ExportEventClips:                            options.ExportEventClips,
	}
}

// Return the default detector options.
func GetDefaultDetectorOptions() DetectorOptions {
	return DetectorOptions{
		AutoThresholds:                              false,
		BrightnessDetectionThreshold:                0.0,
		ColorDifferenceDetectionThreshold:           0.0,
		BinaryThresholdDifferenceDetectionThreshold: 0.0,
		MovingMeanResolution:                        50,
		ExportCsvReport:                             false,
		ExportJsonReport:                            false,
		ExportConfusionMatrix:                       false,
		ExportReport:                                false,
		ConfusionMatrixActualDetectionsExpression:   "",
		SkipFramesExport:                            false,
		Denoise:                                     NoDenoise,
		FrameScalingFactor:                          0.5,
		ImportPreanalyzed:                           false,
		StrictExplicitThreshold:                     true,
		DetectionBoundsExpression:                   "",
		ScaleAlgorithm:                              Default,
		ExportFramesPrefix:                          "frame",
		EventMergeMaxIntervalSeconds:                0.5,
		EventClipPreBufferSeconds:                   1.0,
		EventClipPostBufferSeconds:                  1.0,
		ExportEventClips:                            false,
	}
}
