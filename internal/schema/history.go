package schema

import (
	"net/http"
	"os"
	"sync"
	"time"

	"bitbucket.org/crgw/supplier-hub/internal/tools/converting"
)

type Key string

const (
	RequestingTypeKey Key = "requestingType"
)

type supplierRequestsBucket struct {
	supplierRequests SupplierRequests
	sync.Mutex
}

func NewSupplierRequestsBucket() supplierRequestsBucket {
	return supplierRequestsBucket{
		supplierRequests: []SupplierRequest{},
	}
}

func (r *supplierRequestsBucket) SupplierRequests() *SupplierRequests {
	return &r.supplierRequests
}

func (r *supplierRequestsBucket) AddRequests(requests SupplierRequests) {
	r.Lock()
	r.supplierRequests = append(r.supplierRequests, requests...)
	r.Unlock()
}

func (r *supplierRequestsBucket) FinishedRequest(
	requestType SupplierRequestName,
	startTime time.Time,
	statusCode int,
	method string,
	url string,
	requestBody string,
	requestHeaders http.Header,
	responseBody string,
	responseHeaders http.Header,
) {
	reqHeaders := converting.ConvertMap(requestHeaders)

	req := RequestContent{
		Url:     &url,
		Method:  &method,
		Body:    &requestBody,
		Headers: &reqHeaders,
	}

	historyRequest := SupplierRequest{
		Name:           &requestType,
		RequestContent: &req,
	}

	resHeaders := converting.ConvertMap(responseHeaders)

	res := ResponseContent{
		StatusCode: &statusCode,
		Headers:    &resHeaders,
		Body:       &responseBody,
	}

	historyRequest.ResponseContent = &res

	if os.Getenv("TEST") != "true" {
		duration := int(time.Since(startTime).Milliseconds())
		historyRequest.Duration = &duration
		historyRequest.StartDateTime = &startTime
	}

	r.Lock()
	r.supplierRequests = append(r.supplierRequests, historyRequest)
	r.Unlock()
}
