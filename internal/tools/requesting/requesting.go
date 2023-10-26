package requesting

import (
	"fmt"
	"net/http"
	"os"

	"bitbucket.org/crgw/supplier-hub/internal/schema"
)

func isValidResponse(code int) bool {
	return code >= 200 && code <= 299
}

func RequestErrors(response *http.Response, err error) (*http.Response, *schema.SupplierResponseError) {
	if err != nil {
		if os.IsTimeout(err) {
			e := schema.NewTimeoutError(err.Error())
			return nil, &e
		}

		e := schema.NewConnectionError(err.Error())
		return nil, &e
	}

	if !isValidResponse(response.StatusCode) {
		e := schema.NewSupplierError(fmt.Sprintf("supplier returned status code %d", response.StatusCode))
		return nil, &e
	}

	return response, nil
}
