package ota

import (
	"fmt"

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

func POSBuiler(configuration schema.ProfitMaxDHTConfiguration) POS {
	sources := make([]Source, 0)

	sources = append(sources,
		Source{
			ISOCountry:    "US",
			AgentDutyCode: converting.Unwrap(configuration.AgentDutyCode),
			RequestorID: RequestorID{
				Type: "4",
				ID:   converting.Unwrap(configuration.Vn),
				CompanyName: &CompanyName{
					Code:        "CD:WC",
					CodeContext: fmt.Sprintf("CC:%s", converting.Unwrap(configuration.Cp)),
				},
			},
		})

	if converting.Unwrap(configuration.VendorCode) != "" {
		sources = append(sources,
			Source{
				RequestorID: RequestorID{
					Type: "8",
					ID:   converting.Unwrap(configuration.VendorCode),
				},
			})
	}
	return POS{
		Source: sources,
	}
}
