package util

type Response struct {
	Message string `json:"message"`
	Success bool   `json:"success"`
	Data    any    `json:"data"`
	Status  int    `json:"status"`
	Error   error  `json:"error"`
}

func NewResponse(status int, message string, data any, err error) Response {
	return Response{Status: status, Success: status >= 200 && status < 400, Message: message, Data: data, Error: err}
}
