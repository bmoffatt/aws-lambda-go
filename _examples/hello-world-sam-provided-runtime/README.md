# AWS SAM + Provided Runtime example

This project shows an example of how to configure `sam build` to build and deploy a Lambda handler as a custom runtime function. 

## Requirements

* [AWS SAM CLI](https://docs.aws.amazon.com/serverless-application-model/latest/developerguide/serverless-sam-cli-install.html)
* [Golang](https://golang.org)

## Build and Deploy

```
# one time, copy the local development tree of aws-lambda-go as specified by go.mod
go mod vendor

# build the function
sam build

# test the function locally
sam local invoke

# deploy the function
sam deploy --guided
```
