// Copyright 2020 Amazon.com, Inc. or its affiliates. All Rights Reserved
// +build !lambda.norpc

package lambda

import (
	"log"
	"net"
	"net/rpc"
)

func init() {
	startFunctions["_LAMBDA_SERVER_PORT"] = startFunctionRPC
}

func startFunctionRPC(port string, handler Handler) {
	lis, err := net.Listen("tcp", "localhost:"+port)
	if err != nil {
		log.Fatal(err)
	}
	err = rpc.Register(NewFunction(handler))
	if err != nil {
		log.Fatal("failed to register handler function")
	}
	rpc.Accept(lis)
	log.Fatal("accept should not have returned")
}
