// Copyright 2020 Amazon.com, Inc. or its affiliates. All Rights Reserved

package main

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"testing"
)

func TestSizes(t *testing.T) {
	if testing.Short() {
		t.Skip()
		return
	}
	t.Log("Test how different arguments affect binary and archive sizes")
	cases := []struct {
		file string
		args []string
	}{
		{"testdata/apigw.go", nil},
		{"testdata/noop.go", nil},
		{"testdata/noop.go", []string{"-tags", "lambda.norpc"}},
		{"testdata/noop.go", []string{"-ldflags=-s -w"}},
		{"testdata/noop.go", []string{"-tags", "lambda.norpc", "-ldflags=-s -w"}},
	}
	testDir, err := os.Getwd()
	require.NoError(t, err)
	tempDir, err := ioutil.TempDir("/tmp", "build-lambda-zip")
	require.NoError(t, err)
	for _, test := range cases {
		os.Chdir(testDir)
		testName := fmt.Sprintf("%s, %v", test.file, test.args)
		t.Run(testName, func(t *testing.T) {
			binPath := path.Join(tempDir, test.file+".bin")
			zipPath := path.Join(tempDir, test.file+".zip")

			buildArgs := []string{"build", "-o", binPath}
			buildArgs = append(buildArgs, test.args...)
			buildArgs = append(buildArgs, test.file)

			gocmd := exec.Command("go", buildArgs...)
			gocmd.Env = append(os.Environ(), "GOOS=linux")
			gocmd.Stderr = os.Stderr
			require.NoError(t, gocmd.Run())
			require.NoError(t, os.Chdir(filepath.Dir(binPath)))
			require.NoError(t, compressExeAndArgs(zipPath, binPath, []string{}))

			binInfo, err := os.Stat(binPath)
			require.NoError(t, err)
			zipInfo, err := os.Stat(zipPath)
			require.NoError(t, err)

			t.Logf("zip size = %d Kb, bin size = %d Kb", zipInfo.Size()/1024, binInfo.Size()/1024)
		})
	}

}
