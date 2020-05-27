.PHONY: build-ProvidedRuntimeFunction
.DEFAULT_GOAL := help

help:
	@echo "You probably meant to run 'sam build' instead!'"
	@echo 
	@echo "This makefile is used by the 'sam build' command"
	@echo "For every function in the 'template.yml', 'sam build' will run something like:"
	@echo "\t 'make build-{FunctionLogicalID} ARTIFACTS_DIR=.aws-sam/build/{FunctionLogicalID}'"
	@echo
	@echo "To deploy the function after building, run 'sam deploy --guided'"
	exit 1

build-ProvidedRuntimeFunction:
	GOOS=linux GOARCH=amd64 go build -o $(ARTIFACTS_DIR)/bootstrap main.go
