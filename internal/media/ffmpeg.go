package media

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
)

type FFM struct{}
type ProbeFormat struct {
	Duration string `json:"duration"`
}
type ProbeStream struct {
	CodecName string `json:"codec_name"`
	CodecType string `json:"codec_type"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
}
type ProbeResult struct {
	Format  ProbeFormat   `json:"format"`
	Streams []ProbeStream `json:"streams"`
}

func NewFFM() *FFM {
	return &FFM{}
}
func (f *FFM) Probe(ctx context.Context, path string) (*ProbeResult, error) {
	cmd := exec.CommandContext(ctx, "ffprobe", "-v", "error", "-print_format", "json", "-show_streams", "-show_format", path)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var pr ProbeResult
	if err := json.Unmarshal(out, &pr); err != nil {
		return nil, err
	}
	return &pr, nil
}
func (f *FFM) TranscodeH264(ctx context.Context, inPath, outPath string, w, h int) error {

	scaleFilter := fmt.Sprintf("scale=-2:%d", h)

	args := []string{
		"-y", "-i", inPath,
		"-c:v", "libx264",
		"-preset", "veryfast",
		"-crf", "22",
		"-vf", scaleFilter,
		"-c:a", "aac", "-b:a", "128k", "-ac", "2",
		"-movflags", "+faststart",
		outPath,
	}
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errMsg := stderr.String()
		if len(errMsg) > 500 {
			errMsg = errMsg[:500] + "..."
		}
		return fmt.Errorf("ffmpeg transcode failed: %w\nstderr: %s", err, errMsg)
	}
	return nil
}

func (f *FFM) SegmentHLS(ctx context.Context, inPath, outDir, baseName string, segmentDuration int) error {
	if segmentDuration <= 0 {
		segmentDuration = 4
	}
	segmentPath := filepath.Join(outDir, fmt.Sprintf("%s_%%03d.ts", baseName))
	indexFilePath := filepath.Join(outDir, fmt.Sprintf("%s.m3u8", baseName))
	args := []string{
		"-y", "-i", inPath,
		"-c:v", "copy",
		"-c:a", "copy",
		"-start_number", "0",
		"-hls_time", fmt.Sprintf("%d", segmentDuration),
		"-hls_playlist_type", "vod",
		"-hls_segment_filename", segmentPath,
		indexFilePath,
	}

	cmd := exec.CommandContext(ctx, "ffmpeg", args...)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errMsg := stderr.String()
		if len(errMsg) > 500 {
			errMsg = errMsg[:500] + "..."
		}
		return fmt.Errorf("ffmpeg segment failed: %w\nstderr: %s", err, errMsg)
	}
	return nil
}
func (f *FFM) CreateThumbnail(ctx context.Context, inputURL string, outputPath string, timestamp int) error {
	// Build ffmpeg command:
	// -ss <timestamp> : seek to timestamp (e.g. 3 seconds)
	// -i <input>      : input video
	// -frames:v 1     : capture 1 frame
	// -q:v 2          : quality (lower is better, 2 is good)
	cmd := exec.CommandContext(ctx,
		"ffmpeg",
		"-ss", fmt.Sprintf("%d", timestamp),
		"-i", inputURL,
		"-frames:v", "1",
		"-q:v", "2",
		"-y", // overwrite output if exists
		outputPath,
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to generate thumbnail: %w\nffmpeg output: %s", err, string(out))
	}

	return nil
}
