// Copyright 2020 Amazon.com, Inc. or its affiliates. All Rights Reserved

package runtimeapi

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	fmtNextURL  = "http://%s/2018-06-01/runtime/invocation/next"
	fmtDoneURL  = "http://%s/2018-06-01/runtime/invocation/%s/response"
	fmtErrorURL = "http://%s/2018-06-01/runtime/invocation/%s/error"
)

func TestClientNext(t *testing.T) {
	dummyRequestID := "dummy-request-id"
	dummyPayload := `{"hello": "world"}`

	returnsBody := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/2018-06-01/runtime/invocation/next" {
			w.WriteHeader(http.StatusNotImplemented)
		}
		w.Header().Add(AWSRequestIDHeader, dummyRequestID)
		_, _ = w.Write([]byte(dummyPayload))
		return
	}))
	defer returnsBody.Close()

	returnsNoBody := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/2018-06-01/runtime/invocation/next" {
			w.WriteHeader(http.StatusNotImplemented)
		}
		w.Header().Add(AWSRequestIDHeader, dummyRequestID)
		w.WriteHeader(http.StatusOK)
		return
	}))
	defer returnsNoBody.Close()

	t.Run("handles regular response", func(t *testing.T) {
		invoke, err := New(serverAddress(returnsBody)).Next()
		require.NoError(t, err)
		assert.Equal(t, dummyRequestID, invoke.ID)
		assert.Equal(t, dummyPayload, string(invoke.Payload))
	})

	t.Run("handles no body", func(t *testing.T) {
		invoke, err := New(serverAddress(returnsNoBody)).Next()
		require.NoError(t, err)
		assert.Equal(t, dummyRequestID, invoke.ID)
		assert.Equal(t, 0, len(invoke.Payload))
	})
}

func TestClientDoneAndError(t *testing.T) {
	invokeID := "theid"

	var capturedErrors [][]byte
	var capturedResponses [][]byte

	acceptsResponses := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Logf("unexpected method: %s", r.Method)
			w.WriteHeader(http.StatusNotImplemented)
			return
		}
		if r.URL.Path != fmt.Sprintf("/2018-06-01/runtime/invocation/%s/error", invokeID) && r.URL.Path != fmt.Sprintf("/2018-06-01/runtime/invocation/%s/response", invokeID) {
			t.Logf("unexpected url path: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		body, _ := ioutil.ReadAll(r.Body)
		if strings.HasSuffix(r.URL.Path, "/error") {
			capturedErrors = append(capturedErrors, body)
		} else if strings.HasSuffix(r.URL.Path, "/response") {
			capturedResponses = append(capturedErrors, body)
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer acceptsResponses.Close()

	client := New(serverAddress(acceptsResponses))
	inputPayloads := [][]byte{nil, {}, []byte("hello")}
	expectedPayloadsRecived := [][]byte{{}, {}, []byte("hello")} // nil payload expected to be read as empty bytes by the server
	for i, payload := range inputPayloads {
		invoke := &Invoke{
			ID:     invokeID,
			Client: client,
		}
		t.Run(fmt.Sprintf("happy Done with payload[%d]", i), func(t *testing.T) {
			err := invoke.Success(payload, ContentJSON)
			assert.NoError(t, err)
		})
		t.Run(fmt.Sprintf("happy Error with payload[%d]", i), func(t *testing.T) {
			err := invoke.Failure(payload, ContentJSON)
			assert.NoError(t, err)
		})
	}
	assert.Equal(t, expectedPayloadsRecived, capturedErrors)
	assert.Equal(t, expectedPayloadsRecived, capturedResponses)
}

func TestInvalidRequestsForMalformedEndpoint(t *testing.T) {
	_, err := New("🚨").Next()
	require.Error(t, err)
	err = (&Invoke{Client: New("🚨")}).Success(nil, "")
	require.Error(t, err)
	err = (&Invoke{Client: New("🚨")}).Failure(nil, "")
	require.Error(t, err)
}

func TestStatusCodes(t *testing.T) {
	for i := 200; i < 600; i++ {
		t.Run(fmt.Sprintf("status: %d", i), func(t *testing.T) {
			url := fmt.Sprintf("status-%d", i)

			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = ioutil.ReadAll(r.Body)
				w.WriteHeader(i)
			}))

			defer ts.Close()

			client := New(serverAddress(ts))
			invoke := &Invoke{ID: url, Client: client}
			if i == http.StatusOK {
				t.Run("Next should not error", func(t *testing.T) {
					_, err := client.Next()
					require.NoError(t, err)
				})
			} else {
				t.Run("Next should error", func(t *testing.T) {
					_, err := client.Next()
					require.Error(t, err)
					if i != 301 && i != 302 && i != 303 {
						assert.Contains(t, err.Error(), "unexpected status code")
						assert.Contains(t, err.Error(), fmt.Sprintf("%d", i))
					}
				})
			}

			if i == http.StatusAccepted {
				t.Run("Success should not error", func(t *testing.T) {
					err := invoke.Success(nil, "")
					require.NoError(t, err)
				})
				t.Run("Failure should not error", func(t *testing.T) {
					err := invoke.Failure(nil, "")
					require.NoError(t, err)
				})
			} else {
				t.Run("Success should error", func(t *testing.T) {
					err := invoke.Success(nil, "")
					require.Error(t, err)
					if i != 301 && i != 302 && i != 303 {
						assert.Contains(t, err.Error(), "unexpected status code")
						assert.Contains(t, err.Error(), fmt.Sprintf("%d", i))
					}
				})
				t.Run("Failure should error", func(t *testing.T) {
					err := invoke.Failure(nil, "")
					require.Error(t, err)
					if i != 301 && i != 302 && i != 303 {
						assert.Contains(t, err.Error(), "unexpected status code")
						assert.Contains(t, err.Error(), fmt.Sprintf("%d", i))
					}
				})
			}
		})
	}

}

func serverAddress(ts *httptest.Server) string {
	return strings.Split(ts.URL, "://")[1]
}
