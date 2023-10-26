package json

type Errors struct {
	Error ErrorInfo `json:"error"`
}

type ErrorInfo struct {
	Code     string `json:"code"`
	HttpCode int    `json:"http_code"`
	Message  string `json:"message"`
}

func (e *Errors) ErrorMessage() string {
	if e.Error.Message != "" {
		return e.Error.Message
	}

	return ""
}
