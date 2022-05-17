// Copyright 2018 Amazon.com, Inc. or its affiliates. All Rights Reserved

package main

import (
	"archive/zip"
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/lambda/types"
)

const usage = `build-lambda-zip - Puts an executable and supplemental files into a zip file that works with AWS Lambda.
usage:
  build-lambda-zip [options] handler-exe[.go] [paths...]

notes:
  This tool will run "GOOS=linux go build" if the handler argument ends with the file extension ".go"

  If set to upload to a function, the default credentials provider from aws-sdk-go-v2 will be used

options:
  -o, --output          <output-path>     sets the output file path for the zip. (default: ${handler-exe}.zip)
  -u, --update-function <function-name>   pushes the built zip as a code update to the named function
  -h, --help                              prints usage
`

func main() {
	var outputZip string
	flag.StringVar(&outputZip, "o", "", "")
	flag.StringVar(&outputZip, "output", "", "")
	var functionName string
	flag.StringVar(&functionName, "u", "", "")
	flag.StringVar(&functionName, "update-function", "", "")
	flag.Usage = func() {
		fmt.Fprint(os.Stderr, usage)
	}
	flag.Parse()
	if len(flag.Args()) == 0 {
		log.Fatal("no input provided")
	}
	inputExe := flag.Arg(0)
	if outputZip == "" {
		outputZip = fmt.Sprintf("%s.zip", filepath.Base(inputExe))
	}

	if filepath.Ext(inputExe) == ".go" {
		builtExePath, err := goBuild(inputExe)
		if err != nil {
			log.Fatalf("failed to compile .go file %s: %v", inputExe, err)
		}
		defer func() {
			log.Printf("removing intermediate file %s", builtExePath)
			if err := os.Remove(builtExePath); err != nil {
				log.Printf("cleanup of %s failed: %v", builtExePath, err)
			}
		}()
		log.Printf("compiled %s to %s", inputExe, builtExePath)
		inputExe = builtExePath
	}

	if functionName == "" {
		if err := compressExeAndArgsToFile(outputZip, inputExe, flag.Args()[1:]); err != nil {
			log.Fatalf("failed to compress file: %v", err)
		}
		log.Printf("wrote %s", outputZip)
	} else {
		if err := updateFunctionCode(functionName, inputExe, flag.Args()[1:]); err != nil {
			log.Fatalf("failed to update function code: %v", err)
		}
		log.Printf("updated function code for %s", functionName)
	}
}

func writeExe(writer *zip.Writer, pathInZip string, data []byte) error {
	if pathInZip != "bootstrap" {
		header := &zip.FileHeader{Name: "bootstrap", Method: zip.Deflate}
		header.SetMode(0755 | os.ModeSymlink)
		link, err := writer.CreateHeader(header)
		if err != nil {
			return err
		}
		if _, err := link.Write([]byte(pathInZip)); err != nil {
			return err
		}
	}

	exe, err := writer.CreateHeader(&zip.FileHeader{
		CreatorVersion: 3 << 8,     // indicates Unix
		ExternalAttrs:  0777 << 16, // -rwxrwxrwx file permissions
		Name:           pathInZip,
		Method:         zip.Deflate,
	})
	if err != nil {
		return err
	}

	_, err = exe.Write(data)
	return err
}

func updateFunctionCode(functionName, exePath string, args []string) error {
	buffer := bytes.NewBuffer(nil)
	if err := compressExeAndArgs(buffer, exePath, args); err != nil {
		return err
	}
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return err
	}
	svc := lambda.NewFromConfig(cfg)
	updateResponse, err := svc.UpdateFunctionCode(context.Background(), &lambda.UpdateFunctionCodeInput{
		FunctionName: &functionName,
		ZipFile:      buffer.Bytes(),
	})
	if err != nil {
		return err
	}
	status := updateResponse.LastUpdateStatus
	statusReason := updateResponse.StateReason
	for {
		log.Printf("state[%s] reason[%s]", status, deref(statusReason))
		if status == types.LastUpdateStatusInProgress {
			time.Sleep(250 * time.Millisecond)
			function, err := svc.GetFunction(context.Background(), &lambda.GetFunctionInput{
				FunctionName: &functionName,
			})
			if err != nil {
				return err
			}
			status = function.Configuration.LastUpdateStatus
			statusReason = function.Configuration.LastUpdateStatusReason
			continue
		}
		break
	}
	return nil
}

func deref(s *string) string {
	if s == nil {
		return "<nil>"
	}
	return *s
}

func compressExeAndArgsToFile(outZipPath string, exePath string, args []string) error {
	zipFile, err := os.Create(outZipPath)
	if err != nil {
		return err
	}
	defer func() {
		closeErr := zipFile.Close()
		if closeErr != nil {
			fmt.Fprintf(os.Stderr, "Failed to close zip file: %v\n", closeErr)
		}
	}()

	return compressExeAndArgs(zipFile, exePath, args)
}

func compressExeAndArgs(zipFile io.Writer, exePath string, args []string) error {
	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()
	data, err := ioutil.ReadFile(exePath)
	if err != nil {
		return err
	}

	err = writeExe(zipWriter, filepath.Base(exePath), data)
	if err != nil {
		return err
	}

	for _, arg := range args {
		writer, err := zipWriter.Create(arg)
		if err != nil {
			return err
		}
		data, err := ioutil.ReadFile(arg)
		if err != nil {
			return err
		}
		_, err = writer.Write(data)
		if err != nil {
			return err
		}
	}
	return err
}

func goBuild(in string) (string, error) {
	out := fmt.Sprintf("%s.exe", filepath.Base(in))
	cmd := exec.Command("go", "build", "-o", out, in)
	cmd.Env = append(
		[]string{
			"GOOS=linux",
			"GOARCH=amd64",
		},
		os.Environ()...,
	)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	return out, cmd.Run()
}
