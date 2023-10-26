package hertz_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"bitbucket.org/crgw/supplier-hub/internal/platform/implementations/hertz"
	"bitbucket.org/crgw/supplier-hub/internal/schema"
	"bitbucket.org/crgw/supplier-hub/internal/tools/converting"
	"github.com/go-redis/redismock/v9"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func modifyParams(fileName string, url string) schema.ModifyRequestParams {
	content, _ := os.ReadFile(fileName)
	content = bytes.Replace(content, []byte("{{supplierApiUrl}}"), []byte(url), -1)

	var params schema.ModifyRequestParams
	_ = json.Unmarshal(content, &params)

	return params
}

func TestModifyRequest(t *testing.T) {
	out := &bytes.Buffer{}
	log := zerolog.New(out)

	t.Run("should build modify request based on params", func(t *testing.T) {
		tests := []struct {
			name                string
			requestFile         string
			expectedRequestFile string
		}{
			{
				name:                "general",
				requestFile:         "./testdata/modify/modify_test_1_request.json",
				expectedRequestFile: "./testdata/modify/modify_test_1_request.xml",
			},
		}

		var handlerFunc http.HandlerFunc
		testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			handlerFunc(w, r)
		}))
		defer testServer.Close()

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				handlerFuncCalled := false
				handlerFunc = func(w http.ResponseWriter, r *http.Request) {
					body, _ := io.ReadAll(r.Body)
					xmlBody, _ := os.ReadFile(test.expectedRequestFile)

					assert.Equal(t, "application/xml; charset=utf-8", r.Header.Get("Content-Type"))
					assert.Equal(t, string(xmlBody), string(body)+"\n")

					w.WriteHeader(http.StatusNoContent)
					handlerFuncCalled = true
				}

				params := modifyParams(test.requestFile, testServer.URL)

				redisClient, _ := redismock.NewClientMock()
				service := hertz.New(redisClient)
				ctx := context.Background()

				_, _ = service.ModifyBooking(ctx, params, &log)

				assert.True(t, handlerFuncCalled)
			})
		}
	})

	t.Run("should handle timeout", func(t *testing.T) {
		testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(10 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		}))
		defer testServer.Close()

		params := modifyParams("./testdata/modify/modify_request_connection.json", testServer.URL)

		redisClient, _ := redismock.NewClientMock()
		service := hertz.New(redisClient)
		ctx := context.Background()

		modifyResponse, _ := service.ModifyBooking(ctx, params, &log)

		supplierError := converting.Unwrap(modifyResponse.Errors)[0]

		assert.Len(t, *modifyResponse.Errors, 1)
		assert.Equal(t, schema.TimeoutError, supplierError.Code)
		assert.True(t, len(supplierError.Message) > 0)
	})

	t.Run("should handle connection errors", func(t *testing.T) {
		testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(10 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		}))
		defer testServer.Close()

		params := modifyParams("./testdata/modify/modify_request_connection_valid.json", testServer.URL)

		redisClient, _ := redismock.NewClientMock()
		service := hertz.New(redisClient)

		channel := make(chan schema.ModifyResponse, 1)

		go func() {
			ctx := context.Background()
			modifyResponse, _ := service.ModifyBooking(ctx, params, &log)
			channel <- modifyResponse
		}()
		time.Sleep(5 * time.Millisecond)
		testServer.CloseClientConnections()

		modifyResponse := <-channel

		supplierError := converting.Unwrap(modifyResponse.Errors)[0]

		assert.Len(t, *modifyResponse.Errors, 1)
		assert.Equal(t, schema.ConnectionError, supplierError.Code)
		assert.True(t, len(supplierError.Message) > 0)
	})

	t.Run("should handle status != 200 error", func(t *testing.T) {
		testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer testServer.Close()

		params := modifyParams("./testdata/modify/modify_request_connection_valid.json", testServer.URL)

		redisClient, _ := redismock.NewClientMock()
		service := hertz.New(redisClient)
		ctx := context.Background()

		modifyResponse, _ := service.ModifyBooking(ctx, params, &log)

		supplierError := converting.Unwrap(modifyResponse.Errors)[0]

		assert.Len(t, *modifyResponse.Errors, 1)
		assert.Equal(t, schema.SupplierError, supplierError.Code)
		assert.Equal(t, "supplier returned status code 404", supplierError.Message)
	})

	t.Run("should return errors from supplier response", func(t *testing.T) {
		errorResponse, _ := os.ReadFile("./testdata/modify/modify_outgoing_error_response.xml")

		testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write(errorResponse)
		}))
		defer testServer.Close()

		params := modifyParams("./testdata/modify/modify_request_connection_valid.json", testServer.URL)

		redisClient, _ := redismock.NewClientMock()
		service := hertz.New(redisClient)
		ctx := context.Background()

		modifyResponse, _ := service.ModifyBooking(ctx, params, &log)

		supplierError := converting.Unwrap(modifyResponse.Errors)[0]

		assert.Len(t, converting.Unwrap(modifyResponse.Errors), 1)
		assert.Equal(t, schema.SupplierError, supplierError.Code)
		assert.Equal(t, "INCORRECT SPECIAL EQUIPMENT CODE", supplierError.Message)
	})

	t.Run("should return build supplier requests history array", func(t *testing.T) {
		errorResponse, _ := os.ReadFile("./testdata/modify/modify_outgoing_error_response.xml")

		testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write(errorResponse)
		}))
		defer testServer.Close()

		params := modifyParams("./testdata/modify/modify_request_connection_valid.json", testServer.URL)

		redisClient, _ := redismock.NewClientMock()
		service := hertz.New(redisClient)
		ctx := context.Background()

		modifyResponse, _ := service.ModifyBooking(ctx, params, &log)

		assert.Len(t, (*modifyResponse.SupplierRequests), 1)

		supplierRequest := converting.Unwrap(modifyResponse.SupplierRequests)[0]

		assert.Equal(t, testServer.URL, converting.Unwrap(supplierRequest.RequestContent.Url))
		assert.Equal(t, http.MethodPost, converting.Unwrap(supplierRequest.RequestContent.Method))
		assert.Len(t, converting.Unwrap(supplierRequest.RequestContent.Headers), 1)

		assert.Equal(t, http.StatusOK, converting.Unwrap(supplierRequest.ResponseContent.StatusCode))
		assert.Len(t, converting.Unwrap(supplierRequest.ResponseContent.Headers), 3)
	})
}
