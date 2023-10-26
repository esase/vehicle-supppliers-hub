package ota

import (
	"bitbucket.org/crgw/supplier-hub/internal/schema"
)

const (
	hertzPaymentRulePrePay int = 2
)

type feeIncludeType string

const (
	feesIncluded              feeIncludeType = "feesIncluded"
	feesNotIncludedPayNow     feeIncludeType = "feesNotIncludedPayNow"
	feesNotIncludedPayLocally feeIncludeType = "feesNotIncludedPayLocally"
	feesUnknown               feeIncludeType = "feesUnknown"
)

type coverageIncludedType string

const (
	coverageIncluded          coverageIncludedType = "feesIncluded"
	coverageNotIncluded       coverageIncludedType = "coverageNotIncluded"
	coverageNotIncludedPayNow coverageIncludedType = "feesNotIncludedPayNow"
	coverageUnknown           coverageIncludedType = "coverageUnknown"
)

func mapContains(m map[string][]string, key string, e any) bool {
	value, ok := m[key]
	if !ok {
		return false
	}

	return contains(value, e)
}

func contains(s []string, e any) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func isPartialPayment(params schema.RatesRequestParams) bool {
	return params.Contract.PaymentType == int(schema.PaymentTypePartialPrepay)
}

func fullWithoutPaymentRules(params schema.RatesRequestParams, paymentRules []PaymentRule) bool {
	return params.Contract.PaymentType == int(schema.PaymentTypeFullPrepay) && !(len(paymentRules) > 0)
}
