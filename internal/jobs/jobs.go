package jobs

type Step string

const (
	StepValidate  Step = "validate"
	StepTranscode Step = "transcode"
	StepSegment   Step = "segment"
	StepChecksum  Step = "checksum"
	StepThumbs    Step = "thumbnail"
	StepPublish   Step = "publish"
)

type JobPayload struct {
	VideoID string `json:"videoId"`
	Step    Step   `json:"step"`
}
