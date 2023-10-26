package ota

import "encoding/xml"

type SoapEnvelope struct {
	XMLName       xml.Name      `xml:"SOAP-ENV:Envelope"`
	XmlnsSoapEnv  string        `xml:"xmlns:SOAP-ENV,attr"`
	XmlnsXsd      string        `xml:"xmlns:xsd,attr"`
	XmlnsXsi      string        `xml:"xmlns:xsi,attr"`
	SoapEnvHeader SoapEnvHeader `xml:"SOAP-ENV:Header"`
	SoapEnvBody   SoapEnvBody   `xml:"SOAP-ENV:Body"`
}
