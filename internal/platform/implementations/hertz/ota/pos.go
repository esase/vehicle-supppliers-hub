package ota

import (
	"bitbucket.org/crgw/supplier-hub/internal/schema"
	"bitbucket.org/crgw/supplier-hub/internal/tools/converting"
)

type POS struct {
	Source []Source
}

type Source struct {
	ISOCountry    string      `xml:"ISOCountry,attr,omitempty"`
	AgentDutyCode string      `xml:"AgentDutyCode,attr,omitempty"`
	RequestorID   RequestorID `xml:"RequestorID"`
}

func NewPOS(residenceCountry string, configuration schema.HertzConfiguration, brokerReference string) POS {
	sources := make([]Source, 0)

	sources = append(sources, Source{
		ISOCountry:    residenceCountry,
		AgentDutyCode: converting.Unwrap(configuration.Vc),
		RequestorID: RequestorID{
			Type: "4",
			ID:   converting.Unwrap(configuration.Vn),
			CompanyName: &CompanyName{
				Code:        "CP",
				CodeContext: converting.Unwrap(configuration.Cp),
			},
		},
	})

	sources = append(sources, Source{
		RequestorID: RequestorID{
			Type: "8",
			ID:   configuration.VendorCode,
		},
	})

	if configuration.Taco != nil {
		sources = append(sources, Source{
			RequestorID: RequestorID{
				Type: "5",
				ID:   *configuration.Taco,
			},
		})
	}

	if configuration.BookingAgent != nil {
		sources = append(sources, Source{
			RequestorID: RequestorID{
				Type: "29",
				ID:   *configuration.BookingAgent,
			},
		})
	}

	if brokerReference != "" {
		sources = append(sources, Source{
			RequestorID: RequestorID{
				Type: "16",
				ID:   brokerReference,
			},
		})
	}

	return POS{
		Source: sources,
	}
}
