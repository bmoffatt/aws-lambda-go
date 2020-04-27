// This package provides a client implementation of the Lambda Runtime API
// https://docs.aws.amazon.com/lambda/latest/dg/runtimes-api.html
package runtimeapi

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"runtime"
)

const (
	AWSRequestIDHeader = "Lambda-Runtime-Aws-Request-Id"
	DeadlineMSHeader = "Lambda-Runtime-Deadline-Ms"
	TraceIDHeader = "Lambda-Runtime-Trace-Id"
	CognitoIdentityHeader = "Lambda-Runtime-Cognito-Identity"
	ClientContextHeader = "Lambda-Runtime-Client-Context"
	InvokedFunctionARNHeader = "Lambda-Runtime-Invoked-Function-Arn"

	apiVersion = "2018-06-01"
)

type Client struct {
	baseURL    string
	userAgent string
	httpClient *http.Client
}

func New(address string) *Client {
	client := &http.Client{
		Timeout: 0, // connections to the runtime API are never expected to time out
	}
	endpoint := "http://" + address + "/" + apiVersion + "/runtime/invocation/"
	userAgent := "aws-lambda-go/" + runtime.Version()
	return &Client{endpoint, userAgent, client}
}

type Invoke struct {
	ID string
	Payload []byte
	Headers http.Header
	Client *Client
}

type ContentType string
const (
	ContentJSON ContentType = "application/json"
)

// Success sends the response payload for an in-progress invocation.
// Notes:
//   * An Invoke is not complete until Next() is called again!
func (i *Invoke) Success(payload []byte, contentType ContentType) error {
	url := i.Client.baseURL + i.ID + "/response"
	return i.Client.post(url, contentType, payload)
}

// Failure sends the payload to the Runtime API. This marks the function's Invoke as a failure.
// Notes:
//    * The execution of the function process continues, and is billed, until Next() is called again!
//    * A Lambda Function continues to be re-used for future invokes even after a failure.
//      If the error is fatal (panic, unrecoverable state), exit the process immediately after calling Failure()
func (i *Invoke) Failure(payload []byte, contentType ContentType) error {
	url := i.Client.baseURL + i.ID + "/error"
	return i.Client.post(url, contentType, payload)
}

// Next connects to the Runtime API and waits for a new Invoke Request to be available.
// Note: After a call to Done() or Error() has been made, a call to Next() will complete the in-flight invoke.
func (c *Client) Next() (*Invoke, error) {
	url := c.baseURL + "next"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to construct GET request to %s: %v", url, err)
	}
	req.Header.Set("User-Agent", c.userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get the next invoke: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to GET %s: got unexpected status code: %d", url, resp.StatusCode)
	}

	invokeBuffer := bytes.NewBuffer(nil)
	if resp.Body != nil {
		if _, err := io.Copy(invokeBuffer, resp.Body); err != nil {
			return nil, fmt.Errorf("failed to read the invoke payload: %v", err)
		}
	}

	return &Invoke{
		ID: resp.Header.Get(AWSRequestIDHeader),
		Payload: invokeBuffer.Bytes(),
		Headers: resp.Header,
		Client: c,
	}, nil
}

func (c *Client) post(url string, contentType ContentType, payload []byte) error {
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("failed to construct POST request to %s: %v", url, err)
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Content-Type", string(contentType))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to POST to %s: %v", url, err)
	}
	if resp.Body != nil {
		_, _ = io.Copy(ioutil.Discard, resp.Body)
		_ = resp.Body.Close()
	}

	if 202 != resp.StatusCode {
		return fmt.Errorf("failed to POST to %s: got unexpected status code: %d", url, resp.StatusCode)
	}

	return nil
}
