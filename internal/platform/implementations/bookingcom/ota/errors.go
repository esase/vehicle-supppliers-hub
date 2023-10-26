package ota

type Errors struct {
	Error ErrorMessage `xml:"Error"`
}

type ErrorMessage struct {
	Id      int    `xml:"id,attr"`
	Message string `xml:"Message"`
}

func (e *Errors) ErrorMessage() string {
	if e.Error.Message != "" {
		return e.Error.Message
	}

	return ""
}
