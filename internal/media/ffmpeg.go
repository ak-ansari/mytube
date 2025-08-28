package media

import (
	"context"
	"encoding/json"
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
