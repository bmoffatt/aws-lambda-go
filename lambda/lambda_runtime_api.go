// Copyright 2020 Amazon.com, Inc. or its affiliates. All Rights Reserved

package lambda

import (
	"encoding/json"
	"fmt"
	"github.com/aws/aws-lambda-go/lambda/messages"
	"log"
	"strconv"
	"time"

	"github.com/aws/aws-lambda-go/runtimeapi"
)

const (
	serializationErrorFormat = `{"errorType": "Runtime.SerializationError", "errorMessage": "%s"}`
	msPerS                   = int64(time.Second / time.Millisecond)
	nsPerMS                  = int64(time.Millisecond / time.Nanosecond)
)

func startRuntimeAPILoop(api string, handler Handler) {
	client := runtimeapi.New(api)
	function := NewFunction(handler)
	for {
		if invoke, err := client.Next(); err != nil {
			log.Fatal(err)
		} else if err := handleInvoke(invoke, function); err != nil {
			log.Fatal(err)
		}
	}
}

func handleInvoke(invoke *runtimeapi.Invoke, function *Function) error {
	functionRequest, err := convertInvokeRequest(invoke)
	if err != nil {
		return fmt.Errorf("unexpected error occured when parsing the invoke: %v", err)
	}

	functionResponse := &messages.InvokeResponse{}
	if err := function.Invoke(functionRequest, functionResponse); err != nil {
		return fmt.Errorf("unexpected error occured when invoking the handler: %v", err)
	}

	if functionResponse.Error != nil {
		payload := safeMarshal(functionResponse.Error)
		if err := invoke.Failure(payload, runtimeapi.ContentJSON); err != nil {
			return fmt.Errorf("unexpected error occured when sending the function error to the API: %v", err)
		}
		if functionResponse.Error.ShouldExit {
			return fmt.Errorf("calling the handler function resulted in a panic, the process should exit")
		}
	}

	if err := invoke.Success(functionResponse.Payload, runtimeapi.ContentJSON); err != nil {
		return fmt.Errorf("unexpected error occured when sending the function functionResponse to the API: %v", err)
	}

	return nil
}

func convertInvokeRequest(invoke *runtimeapi.Invoke) (*messages.InvokeRequest, error) {
	deadlineEpochMS, err := strconv.ParseInt(invoke.Headers.Get(runtimeapi.DeadlineMSHeader), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse contents of header: %s", runtimeapi.DeadlineMSHeader)
	}
	deadlineS := deadlineEpochMS / msPerS
	deadlineNS := (deadlineEpochMS % msPerS) * nsPerMS

	res := &messages.InvokeRequest{
		InvokedFunctionArn: invoke.Headers.Get(runtimeapi.InvokedFunctionARNHeader),
		XAmznTraceId:       invoke.Headers.Get(runtimeapi.TraceIDHeader),
		Deadline: messages.InvokeRequest_Timestamp{
			Seconds: deadlineS,
			Nanos:   deadlineNS,
		},
	}

	clientContextJSON := invoke.Headers.Get(runtimeapi.ClientContextHeader)
	if clientContextJSON != "" {
		res.ClientContext = []byte(clientContextJSON)
	}

	cognitoIdentityJSON := invoke.Headers.Get(runtimeapi.CognitoIdentityHeader)
	if cognitoIdentityJSON != "" {
		if err := json.Unmarshal([]byte(invoke.Headers.Get(runtimeapi.CognitoIdentityHeader)), res); err != nil {
			return nil, fmt.Errorf("failed to unmarshal cognito identity json: %v", err)
		}
	}

	return res, nil
}

func safeMarshal(v interface{}) []byte {
	payload, err := json.Marshal(v)
	if err != nil {
		return []byte(fmt.Sprintf(serializationErrorFormat, err.Error()))
	}
	return payload
}
