package mapping

import "bitbucket.org/crgw/supplier-hub/internal/schema"

func MappedResidenceCountry(c schema.HertzConfiguration, requestCountry string) string {
	if c.ResidenceCountryMapping == nil {
		return requestCountry
	}

	mappedCountry, ok := (*c.ResidenceCountryMapping)["ALL"]
	if ok {
		return mappedCountry
	}

	mappedCountry, ok = (*c.ResidenceCountryMapping)[requestCountry]
	if !ok {
		return requestCountry
	}

	return mappedCountry
}
