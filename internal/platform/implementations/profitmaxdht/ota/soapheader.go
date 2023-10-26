package ota

import (
	"fmt"

	"bitbucket.org/crgw/supplier-hub/internal/schema"
	"bitbucket.org/crgw/supplier-hub/internal/tools/converting"
)

type SoapEnvHeader struct {
	Credentials Credentials `xml:"ns:credentials"`
}

func SoapEnvHeaderBuilder(configuration schema.ProfitMaxDHTConfiguration) SoapEnvHeader {
	return SoapEnvHeader{
		Credentials: Credentials{
			Xmlns: "http://wsg.avis.com/wsbang/authInAny",
			UserId: UserId{
				EncodingType: "xsd:string",
				Value:        fmt.Sprintf("user:%s", converting.Unwrap(&configuration.Username)),
			},
			Password: Password{
				EncodingType: "xsd:string",
				Value:        fmt.Sprintf("password:%s", converting.Unwrap(&configuration.Password)),
			},
			Client: Client{
				EncodingType: "xsd:string",
				Value:        fmt.Sprintf("client:%s", converting.Unwrap(&configuration.Client)),
			},
			Destination: Destination{
				EncodingType: "xsd:string",
				Value:        fmt.Sprintf("destination:%s", converting.Unwrap(&configuration.Destination)),
			},
		},
	}
}

type Credentials struct {
	Xmlns       string      `xml:"xmlns:ns,attr"`
	UserId      UserId      `xml:"ns:userID"`
	Password    Password    `xml:"ns:password"`
	Client      Client      `xml:"ns:client"`
	Destination Destination `xml:"ns:destination"`
}

type UserId struct {
	EncodingType string `xml:"ns:encodingType,attr"`
	Value        string `xml:",chardata"`
}

type Password struct {
	EncodingType string `xml:"ns:encodingType,attr"`
	Value        string `xml:",chardata"`
}

type Client struct {
	EncodingType string `xml:"ns:encodingType,attr"`
	Value        string `xml:",chardata"`
}

type Destination struct {
	EncodingType string `xml:"ns:encodingType,attr"`
	Value        string `xml:",chardata"`
}
