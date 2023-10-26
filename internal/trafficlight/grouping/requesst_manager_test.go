package grouping

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"bitbucket.org/crgw/supplier-hub/internal/schema"
	"bitbucket.org/crgw/supplier-hub/internal/tools/slowlog"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

type storageMock struct {
	Storage
	acquireLockMock   func(ctx context.Context, cacheKey string) (bool, error)
	releaseLockMock   func(ctx context.Context, cacheKey string)
	storeResponseMock func(ctx context.Context, responseKey string, response *Response, duration time.Duration)
	fetchResponseMock func(ctx context.Context, responseKey string) (*CachedValue, error)
}

func (s *storageMock) AcquireLock(ctx context.Context, cacheKey string) (bool, error) {
	return s.acquireLockMock(ctx, cacheKey)
}

func (s *storageMock) ReleaseLock(ctx context.Context, cacheKey string) {
	s.releaseLockMock(ctx, cacheKey)
}

func (s *storageMock) StoreResponse(ctx context.Context, responseKey string, response *Response, duration time.Duration) {
	s.storeResponseMock(ctx, responseKey, response, duration)
}

func (s *storageMock) FetchResponse(ctx context.Context, responseKey string) (*CachedValue, error) {
	return s.fetchResponseMock(ctx, responseKey)
}

func createManager(storage *storageMock, cacheKey string) RequestManager {
	out := &bytes.Buffer{}
	log := zerolog.New(out)
	slowLog := slowlog.CreateLogger(&log)

	return &requestManager{
		cache:    storage,
		log:      &log,
		slowLog:  slowLog,
		cacheKey: "cacheKey",
	}
}

func TestGroupingManager(t *testing.T) {
	validResponseBody, _ := json.Marshal(ratesResponseObject{
		Errors: &[]schema.SupplierResponseError{},
	})

	requester := func() (*Response, error) {
		return &Response{
			Code: 200,
			Body: string(validResponseBody),
			Headers: map[string][]string{
				"Content-Type": {"application/json"},
			},
		}, nil
	}

	cacheValue := "response body from cache"

	t.Run("should pass through the request if not in the cache and store it", func(t *testing.T) {
		responseCache := make(chan *Response, 1)

		groupingManager := createManager(&storageMock{
			fetchResponseMock: func(ctx context.Context, responseKey string) (*CachedValue, error) {
				return nil, nil
			},
			storeResponseMock: func(ctx context.Context, responseKey string, response *Response, duration time.Duration) {
				responseCache <- response
			},
			acquireLockMock: func(ctx context.Context, cacheKey string) (bool, error) {
				return true, nil
			},
			releaseLockMock: func(ctx context.Context, cacheKey string) {},
		}, "cacheKey")

		response, err := groupingManager.HandleRequest(context.TODO(), requester)

		assert.Equal(t, (<-responseCache).Body, string(validResponseBody))

		assert.Nil(t, err)
		assert.Equal(t, http.StatusOK, response.Code)
		assert.Equal(t, string(validResponseBody), response.Body)
		assert.Equal(t, map[string][]string{
			"Content-Type": {"application/json"},
		}, response.Headers)
	})

	t.Run("should get the request from cache", func(t *testing.T) {
		groupingManager := createManager(&storageMock{
			fetchResponseMock: func(ctx context.Context, responseKey string) (*CachedValue, error) {
				return &CachedValue{
					Code: http.StatusOK,
					Body: cacheValue,
				}, nil
			},
			storeResponseMock: func(ctx context.Context, responseKey string, response *Response, duration time.Duration) {},
			releaseLockMock:   func(ctx context.Context, cacheKey string) {},
		}, "cacheKey")

		response, err := groupingManager.HandleRequest(context.TODO(), requester)

		assert.Nil(t, err)
		assert.Equal(t, "response body from cache", response.Body)
		assert.Equal(t, http.StatusOK, response.Code)
	})

	t.Run("should wait for other process to finish requesting", func(t *testing.T) {
		responseChannel := make(chan string, 1)

		groupingManager := createManager(&storageMock{
			fetchResponseMock: func(ctx context.Context, responseKey string) (*CachedValue, error) {
				select {
				case cacheValue := <-responseChannel:
					return &CachedValue{
						Code: http.StatusOK,
						Body: cacheValue,
					}, nil
				default:
					return nil, nil
				}
			},
			storeResponseMock: func(ctx context.Context, responseKey string, response *Response, duration time.Duration) {},
			acquireLockMock: func(ctx context.Context, cacheKey string) (bool, error) {
				// after first acquireLock call, the response is assigned
				responseChannel <- cacheValue

				return false, nil
			},
			releaseLockMock: func(ctx context.Context, cacheKey string) {},
		}, "cacheKey")

		response, err := groupingManager.HandleRequest(context.TODO(), requester)

		assert.Nil(t, err)
		assert.Equal(t, http.StatusOK, response.Code)
		assert.Equal(t, "response body from cache", response.Body)
	})

	t.Run("should start waiting on the cache, but acquires lock while doing it", func(t *testing.T) {
		acquireLockChannel := make(chan bool, 2)

		groupingManager := createManager(&storageMock{
			fetchResponseMock: func(ctx context.Context, responseKey string) (*CachedValue, error) {
				// tries to fetch ones, then enable the lock
				acquireLockChannel <- true

				return nil, nil
			},
			storeResponseMock: func(ctx context.Context, responseKey string, response *Response, duration time.Duration) {},
			acquireLockMock: func(ctx context.Context, cacheKey string) (bool, error) {
				return <-acquireLockChannel, nil
			},
			releaseLockMock: func(ctx context.Context, cacheKey string) {},
		}, "cacheKey")

		acquireLockChannel <- false
		response, err := groupingManager.HandleRequest(context.TODO(), requester)

		assert.Nil(t, err)
		assert.Equal(t, string(validResponseBody), response.Body)
		assert.Equal(t, http.StatusOK, response.Code)
	})

	t.Run("should release lock if done", func(t *testing.T) {
		releasedChannel := make(chan bool, 1)

		tests := []struct {
			name             string
			manager          RequestManager
			requester        func() (*Response, error)
			expectedResponse *Response
			expectedError    error
		}{
			{
				name: "requesting failed",
				manager: createManager(&storageMock{
					fetchResponseMock: func(ctx context.Context, responseKey string) (*CachedValue, error) {
						return nil, nil
					},
					acquireLockMock: func(ctx context.Context, cacheKey string) (bool, error) {
						return true, nil
					},
					storeResponseMock: func(ctx context.Context, responseKey string, response *Response, duration time.Duration) {
						t.Errorf("Should not store the bad request")
					},
					releaseLockMock: func(ctx context.Context, cacheKey string) {
						releasedChannel <- true
					},
				}, "cacheKey"),
				requester: func() (*Response, error) {
					return nil, errors.New("Dial TCP error ::8080")
				},
				expectedResponse: nil,
				expectedError:    errors.New("Dial TCP error ::8080"),
			},
			{
				name: "bad response",
				manager: createManager(&storageMock{
					fetchResponseMock: func(ctx context.Context, responseKey string) (*CachedValue, error) {
						return nil, nil
					},
					acquireLockMock: func(ctx context.Context, cacheKey string) (bool, error) {
						return true, nil
					},
					storeResponseMock: func(ctx context.Context, responseKey string, response *Response, duration time.Duration) {
						assert.Equal(t, time.Minute, duration)
					},
					releaseLockMock: func(ctx context.Context, cacheKey string) {
						releasedChannel <- true
					},
				}, "cacheKey"),
				requester: func() (*Response, error) {
					return &Response{
						Code: http.StatusBadRequest,
						Body: "error",
					}, nil
				},
				expectedResponse: &Response{
					Code: http.StatusBadRequest,
					Body: "error",
				},
				expectedError: nil,
			},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				response, err := test.manager.HandleRequest(context.TODO(), test.requester)

				assert.True(t, <-releasedChannel)
				assert.Equal(t, test.expectedError, err)
				assert.Equal(t, test.expectedResponse, response)
			})
		}
	})

	t.Run("should pass through the reqest if redis is down", func(t *testing.T) {
		tests := []struct {
			name    string
			manager RequestManager
		}{
			{
				name: "fetch from cache fails",
				manager: createManager(&storageMock{
					fetchResponseMock: func(ctx context.Context, responseKey string) (*CachedValue, error) {
						return nil, errors.New("Connection error")
					},
					storeResponseMock: func(ctx context.Context, responseKey string, response *Response, duration time.Duration) {},
					releaseLockMock:   func(ctx context.Context, cacheKey string) {},
				}, "cacheKey"),
			},
			{
				name: "acquire lock fails",
				manager: createManager(&storageMock{
					fetchResponseMock: func(ctx context.Context, responseKey string) (*CachedValue, error) {
						return nil, nil
					},
					acquireLockMock: func(ctx context.Context, cacheKey string) (bool, error) {
						return false, errors.New("Connection error")
					},
					storeResponseMock: func(ctx context.Context, responseKey string, response *Response, duration time.Duration) {},
					releaseLockMock:   func(ctx context.Context, cacheKey string) {},
				}, "cacheKey"),
			},
		}

		for _, test := range tests {
			response, err := test.manager.HandleRequest(context.TODO(), requester)

			assert.Nil(t, err)
			assert.Equal(t, http.StatusOK, response.Code)
			assert.Equal(t, string(validResponseBody), response.Body)
		}
	})
}
