package dto

type UploadVideoDto struct {
	Size     int64  `json:"size"`
	Filename string `json:"file_name"`
}
