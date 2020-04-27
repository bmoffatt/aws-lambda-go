package lambda

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aws/aws-lambda-go/runtimeapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testEndpoint  = "lambda"
	testRequestID = "dummy-request-id"
	testNextURL   = "http://lambda/2018-06-01/runtime/invocation/dummy-request-id/response"
)

func TestHandleInovoke(t *testing.T) {
	ts := runtimeAPIServer()
	defer ts.Close()

	endpoint := strings.Split(ts.URL, "://")[1]
	t.Logf("test endpoint is: %s", endpoint)
	client := runtimeapi.New(endpoint)
	invoke, err := client.Next()
	require.NoError(t, err)

	noopFunction := NewFunction(NewHandler(func() error {
		return nil
	}))

	errorFunction := NewFunction(NewHandler(func() error {
		return errors.New("a function error")
	}))

	panicFunction := NewFunction(NewHandler(func() error {
		panic(errors.New("a fatal error"))
	}))

	err = handleInvoke(invoke, noopFunction)
	require.NoError(t, err)

	err = handleInvoke(invoke, errorFunction)
	assert.NoError(t, err)

	err = handleInvoke(invoke, panicFunction)
	assert.EqualError(t, err, "calling the handler function resulted in a panic, the process should exit")
}

func runtimeAPIServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.Header().Add(runtimeapi.AWSRequestIDHeader, "dummy-request-id")
			w.Header().Add(runtimeapi.DeadlineMSHeader, "22")
			w.Header().Add(runtimeapi.InvokedFunctionARNHeader, "anarn")
			w.WriteHeader(http.StatusOK)
		case http.MethodPost:
			w.WriteHeader(http.StatusAccepted)
		default:
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
}
