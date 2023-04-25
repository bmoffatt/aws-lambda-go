// Copyright 2020 Amazon.com, Inc. or its affiliates. All Rights Reserved

package messages

import (
	"reflect"
)

func getErrorType(err interface{}) string {
	errorType := reflect.TypeOf(err)
	if errorType.Kind() == reflect.Ptr {
		return errorType.Elem().Name()
	}
	return errorType.Name()
}

func FromError(err error) *InvokeResponse_Error {
	if err == nil {
		return nil
	}
	if ive, ok := err.(InvokeResponse_Error); ok {
		return &ive
	}
	if ive, ok := err.(*InvokeResponse_Error); ok {
		return ive
	}
	var errorName string
	if errorType := reflect.TypeOf(err); errorType.Kind() == reflect.Ptr {
		errorName = errorType.Elem().Name()
	} else {
		errorName = errorType.Name()
	}
	return &InvokeResponse_Error{
		Message: err.Error(),
		Type:    errorName,
	}
}

func FromRecover(err interface{}) *InvokeResponse_Error {
	if err == nil {
		return nil
	}
	if ive, ok := err.(InvokeResponse_Error); ok {
		return &ive
	}
	if ive, ok := err.(*InvokeResponse_Error); ok {
		return ive
	}
	panicInfo := getPanicInfo(err)
	return &InvokeResponse_Error{
		Message:    panicInfo.Message,
		Type:       getErrorType(err),
		StackTrace: panicInfo.StackTrace,
		ShouldExit: true,
	}
}
