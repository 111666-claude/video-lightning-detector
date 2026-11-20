package export

import (
	"fmt"
	"image"
	"io"
	"math"
	"path"
	"slices"
	"time"

	"github.com/Krzysztofz01/video-lightning-detector/internal/event"
	"github.com/Krzysztofz01/video-lightning-detector/internal/frame"
	"github.com/Krzysztofz01/video-lightning-detector/internal/options"
	"github.com/Krzysztofz01/video-lightning-detector/internal/printer"
	"github.com/Krzysztofz01/video-lightning-detector/internal/statistics"
	"github.com/Krzysztofz01/video-lightning-detector/internal/utils"
	"github.com/Krzysztofz01/video-lightning-detector/internal/video"
)

type Exporter interface {
	Export(fc frame.FrameCollection, ds statistics.DescriptiveStatistics, detections []int, timeline event.Timeline) error
}

type exporter struct {
	InputVideoPath string
	OutputDirPath  string
	Options        options.DetectorOptions
	Printer        printer.Printer
}

type exporterArguments struct {
	FrameCollection    frame.FrameCollection
	ExpectedDetections []int
	ActualDetections   []int
	Statistics         statistics.DescriptiveStatistics
	HasConfusionMatrix bool
	ConfusionMatrix    statistics.ConfusionMatrix
	Timeline           event.Timeline
	Segments           []event.Segment
	ClipResults        []event.ClipResult
}

func (exporter *exporter) buildExporterArguments(fc frame.FrameCollection, ds statistics.DescriptiveStatistics, detections []int, timeline event.Timeline) (*exporterArguments, error) {
	var (
		hasConfusionMatrix bool = len(exporter.Options.ConfusionMatrixActualDetectionsExpression) > 0
		confusionMatrix    statistics.ConfusionMatrix
		expectedDetections []int
		err                error
	)

	if hasConfusionMatrix {
		expectedDetections, err = utils.ParseRangeExpression(exporter.Options.ConfusionMatrixActualDetectionsExpression)
		if err != nil {
			return nil, fmt.Errorf("export: failed to parse the confusion matrix actual detections range expression: %w", err)
		}

		confusionMatrix = statistics.CreateConfusionMatrix(expectedDetections, detections, fc.Count())
	}

	frameTimestamps := make([]float64, fc.Count())
	for index, f := range fc.GetAll() {
		frameTimestamps[index] = f.Timestamp
	}

	segments, err := event.PlanSegments(timeline, frameTimestamps, exporter.Options.EventClipPreBufferSeconds, exporter.Options.EventClipPostBufferSeconds)
	if err != nil {
		return nil, fmt.Errorf("export: failed to plan the event clip segments: %w", err)
	}

	return &exporterArguments{
		FrameCollection:    fc,
		ExpectedDetections: expectedDetections,
		ActualDetections:   detections,
		Statistics:         ds,
		HasConfusionMatrix: hasConfusionMatrix,
		ConfusionMatrix:    confusionMatrix,
		Timeline:           timeline,
		Segments:           segments,
		ClipResults:        make([]event.ClipResult, 0),
	}, nil
}

func (exporter *exporter) Export(fc frame.FrameCollection, ds statistics.DescriptiveStatistics, detections []int, timeline event.Timeline) error {
	exportTime := time.Now()

	args, err := exporter.buildExporterArguments(fc, ds, detections, timeline)
	if err != nil {
		return fmt.Errorf("export: failed to build the export arguments: %w", err)
	}

	if err := tableDescriptiveStatistics(exporter.Printer, args.Statistics, options.Verbose); err != nil {
		return fmt.Errorf("export: failed to export descriptive statistics: %w", err)
	}

	if !exporter.Options.SkipFramesExport && len(detections) > 0 {
		if err := exporter.ExportPngFrameImages(args.ActualDetections, fc.Count()); err != nil {
			return fmt.Errorf("export: failed to perform the detected frames images export: %w", err)
		}
	}

	// NOTE: A clip export failure must not prevent the remaining reports from being written;
	// the error is retained and returned after all other artifacts have been produced.
	var clipExportErr error
	if err := exporter.exportEventTimeline(args); err != nil {
		clipExportErr = err
	}

	if args.HasConfusionMatrix {
		exporter.Printer.Debug("Frames used as expected detection classification: %v", args.ExpectedDetections)

		if err := tableConfusionMatrix(exporter.Printer, args.ConfusionMatrix, options.Verbose); err != nil {
			return fmt.Errorf("export: failed to export the confusion matrix: %w", err)
		}
	}

	if exporter.Options.ExportCsvReport {
		csvProgressFinalize := exporter.Printer.Progress("Exporting reports in CSV format")
		defer csvProgressFinalize()

		if path, err := exportCsvFrames(exporter.OutputDirPath, args.FrameCollection); err != nil {
			return fmt.Errorf("export: failed to export csv frames report: %w", err)
		} else {
			exporter.Printer.Info("Frames report in CSV format exported to: %s", path)
		}

		if path, err := exportCsvDescriptiveStatistics(exporter.OutputDirPath, args.Statistics); err != nil {
			return fmt.Errorf("export: failed to export csv descriptive statistics report: %w", err)
		} else {
			exporter.Printer.Info("Descriptive statistics in CSV format exported to %s", path)
		}

		if args.HasConfusionMatrix {
			if path, err := exportCsvConfusionMatrix(exporter.OutputDirPath, args.ConfusionMatrix); err != nil {
				return fmt.Errorf("export: failed to export csv confusion matrix report: %w", err)
			} else {
				exporter.Printer.Info("Confusion matrix in CSV format exported to %s", path)
			}
		}

		if path, err := exportCsvDetectionThresholds(exporter.OutputDirPath, exporter.Options); err != nil {
			return fmt.Errorf("export: failed to export csv detection thresholds report: %w", err)
		} else {
			exporter.Printer.Info("Detections thresholds in CSV format exported to %s", path)
		}

		csvProgressFinalize()
	}

	if exporter.Options.ExportJsonReport || exporter.Options.ExportReport {
		jsonProgressFinalize := exporter.Printer.Progress("Exporting report in JSON format")
		defer jsonProgressFinalize()

		reportPath, err := exportJsonReport(exporter.OutputDirPath, exporter.Options, args)
		if err != nil {
			return fmt.Errorf("export: failed to export the json report: %w", err)
		} else {
			exporter.Printer.Info("Report in JSON format exported to: %s", reportPath)
		}

		jsonProgressFinalize()
	}

	exporter.Printer.Debug("Export finished. Stage took: %s", time.Since(exportTime))

	if clipExportErr != nil {
		return clipExportErr
	}

	return nil
}

func (exporter *exporter) ExportPngFrameImages(detections []int, totalFrameCount int) error {
	framesExportTime := time.Now()
	exporter.Printer.Debug("Starting the frames export stage.")
	exporter.Printer.Info("About to export %d frames.", len(detections))

	slices.Sort(detections)

	video, err := video.NewVideo(exporter.InputVideoPath)
	if err != nil {
		return fmt.Errorf("export: failed to open the video file for the frame export stage: %w", err)
	}

	defer video.Close()

	targetWidth, targetHeight := video.GetOutputDimensions()

	frame := image.NewRGBA(image.Rect(0, 0, targetWidth, targetHeight))
	if err := video.SetFrameBuffer(frame.Pix); err != nil {
		return fmt.Errorf("export: failed to apply the given buffer as the video frame buffer: %w", err)
	}

	if err := video.SetTargetFrames(detections...); err != nil {
		return fmt.Errorf("export: failed to set the detection frames as the video target frames: %w", err)
	}

	progressStep, progressFinalize := exporter.Printer.ProgressSteps("Video frames export stage.", len(detections))

	zeroPadding := 1 + int(math.Floor(math.Log10(float64(totalFrameCount))))
	prefix := exporter.Options.ExportFramesPrefix
	if len(exporter.Options.ExportFramesPrefix) > 0 {
		prefix += "-"
	}

	for _, frameIndex := range detections {
		if err := video.Read(); err == io.EOF {
			break
		} else if err != nil {
			return fmt.Errorf("export: failed to read the video export frame: %w", err)
		}

		frameImageName := fmt.Sprintf("%s%0*d.png", prefix, zeroPadding, frameIndex+1)
		frameImagePath := path.Join(exporter.OutputDirPath, frameImageName)
		if err := utils.ExportImageAsPng(frameImagePath, frame); err != nil {
			return fmt.Errorf("export: failed to export the frame image: %w", err)
		}

		progressStep()
		exporter.Printer.Info("Frame: [%d/%d]. Frame image exported at: %s", frameIndex+1, video.FramesCountApprox(), frameImagePath)
	}

	progressFinalize()
	exporter.Printer.Debug("Frames export stage finished. Stage took: %s", time.Since(framesExportTime))
	return nil
}

func NewExporter(inputVideo, outputDir string, o options.DetectorOptions, p printer.Printer) Exporter {
	return &exporter{
		InputVideoPath: inputVideo,
		OutputDirPath:  outputDir,
		Options:        o,
		Printer:        p,
	}
}

// exportEventTimeline prints the resolved events to the terminal and, when enabled,
// exports the buffered video clips. A clip export failure is recorded per segment and
// surfaced as an error without discarding the remaining export artifacts.
func (exporter *exporter) exportEventTimeline(args *exporterArguments) error {
	if len(args.Timeline.Events) == 0 {
		exporter.Printer.Info("No lightning events were resolved for this video.")
		return nil
	}

	tableData := make([][]string, 0, len(args.Timeline.Events)+1)
	tableData = append(tableData, []string{"Event", "Frames", "Start [s]", "Peak [s]", "End [s]", "Peak intensity", "Hit frames"})

	for _, ev := range args.Timeline.Events {
		tableData = append(tableData, []string{
			fmt.Sprintf("%d", ev.Index+1),
			fmt.Sprintf("%d-%d", ev.StartFrameIndex+1, ev.EndFrameIndex+1),
			fmt.Sprintf("%.3f", ev.StartTime),
			fmt.Sprintf("%.3f", ev.PeakTime),
			fmt.Sprintf("%.3f", ev.EndTime),
			fmt.Sprintf("%.4f", ev.PeakIntensity),
			fmt.Sprintf("%d", ev.HitFrameCount),
		})
	}

	exporter.Printer.Table(tableData)

	if !exporter.Options.ExportEventClips {
		return nil
	}

	if len(args.Segments) == 0 {
		exporter.Printer.Info("No event clip segments were planned for export.")
		return nil
	}

	clipProgressFinalize := exporter.Printer.Progress(fmt.Sprintf("Exporting %d event video clip(s).", len(args.Segments)))
	defer clipProgressFinalize()

	clipExporter := event.NewClipExporter()
	results, err := clipExporter.ExportClips(exporter.InputVideoPath, exporter.OutputDirPath, args.Segments)
	args.ClipResults = results

	for _, result := range results {
		if result.Exported {
			exporter.Printer.Info("Event clip exported to: %s", result.OutputPath)
		} else {
			exporter.Printer.Error("Event clip export failed: %s", result.Error)
		}
	}

	clipProgressFinalize()

	if err != nil {
		return fmt.Errorf("export: one or more event clips failed to export: %w", err)
	}

	return nil
}
