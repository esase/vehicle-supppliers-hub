package ota

import "strings"

type ErrorsMixin struct {
	Error  string `xml:"error"`
	Errors Errors `xml:"Errors"`
}

type Errors struct {
	Error []Error `xml:"Error"`
}

type Error struct {
	Type      string `xml:"Type,attr"`
	Code      string `xml:"Code,attr"`
	RecordID  string `xml:"RecordID,attr"`
	ShortText string `xml:"ShortText,attr"`
}

func (e *ErrorsMixin) ErrorMessage() string {
	if e.Error != "" {
		return e.Error
	}

	if len(e.Errors.Error) > 0 {
		var shortTexts []string
		for _, err := range e.Errors.Error {
			shortTexts = append(shortTexts, err.ShortText)
		}
		return strings.Join(shortTexts, ", ")
	}

	return ""
}
