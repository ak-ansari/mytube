package media

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
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

func min(vals ...int) int {
	m := vals[0]
	for _, v := range vals[1:] {
		if v < m {
			m = v
		}
	}
	return m
}
