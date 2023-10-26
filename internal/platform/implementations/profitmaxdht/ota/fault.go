package ota

import (
	"encoding/xml"
)

type FaultEnvelope struct {
	XMLName   xml.Name  `xml:"Envelope"`
	XmlnsSoap string    `xml:"xmlns:soap,attr"`
	FaultBody FaultBody `xml:"Body"`
}

type FaultBody struct {
	Fault Fault `xml:"Fault"`
}

type Fault struct {
	FaultCode   string `xml:"faultcode"`
	FaultString string `xml:"faultstring"`
}

func (e *FaultEnvelope) FaultMessage() string {
	return e.FaultBody.Fault.FaultString
}
