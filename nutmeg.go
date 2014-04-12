// Copyright 2014 Markus Dittrich. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// nutmeg is a unit and regression test framework for MCell
package main

import (
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"
)

const numSimJobs = 1

var tests []string
var mcell_path string

var rng *rand.Rand

// data structure encapsulating the status of running the
// mdl files underlying an mcell test
type runStatus struct {
	success  bool
	testPath string
	message  string
}

func init() {
	tests = []string{
		"/Users/markus/programming/go/src/github.com/haskelladdict/nutmeg/tests/remove_per_species_list_from_ht"}

	mcell_path = "/Users/markus/programming/c/mcell/mcell-trunk/build/mcell"

	rng = rand.New(rand.NewSource(time.Now().UnixNano()))
}

// test_runner runs mcell on the mdl file passed in as an
// absolute path. The working directory is set to the base path
// of the mdl file.
func simRunner(testPath string, output chan runStatus) {

	mdlPath := filepath.Join(testPath, "test.mdl")
	fmt.Println("running ", mdlPath)
	cmd := exec.Command(mcell_path, "-seed", strconv.Itoa(rng.Intn(10000)),
		"-logfile", "run.log", "-errfile", "err.log", mdlPath)

	// create runDir
	runDir := filepath.Join(testPath, "output")
	if err := os.Mkdir(runDir, 0744); err != nil {
		output <- runStatus{false, testPath, fmt.Sprint(err)}
		return
	}

	// connect stdout and stderr
	stdOut, err := os.Create(filepath.Join(runDir, "stdout.log"))
	if err != nil {
		output <- runStatus{false, testPath, "error opening stdout"}
		return
	}
	cmd.Stdout = stdOut

	stdErr, err := os.Create(filepath.Join(runDir, "stderr.log"))
	if err != nil {
		output <- runStatus{false, testPath, "error opening stderr"}
		return
	}
	cmd.Stderr = stdErr

	cmd.Dir = runDir

	err = cmd.Run()
	if err != nil {
		output <- runStatus{false, testPath, fmt.Sprint(err)}
	} else {
		output <- runStatus{true, testPath, ""}
	}
}

// createSimJobs is responsbile for filling the worker queue with
// jobs to be run via the simulation tool
func createSimJobs(tests []string, simJobs chan string) {
	for _, job := range tests {
		simJobs <- job
	}
	close(simJobs)
}

// runSimJobs loops over all available jobs and runs each of
// them in a simRunner.
func runSimJobs(simJobs <-chan string, simOutput chan runStatus) {
	for job := range simJobs {
		simRunner(job, simOutput)
	}
}

// clean_removes all files leftover from a previous test run
func clean_output(tests []string) error {
	for _, path := range tests {
		outputPath := filepath.Join(path, "output")
		if err := os.RemoveAll(outputPath); err != nil {
			return err
		}
	}
	return nil
}

// main routine
func main() {

	if err := clean_output(tests); err != nil {
		fmt.Println("Failed to clean up previous test results", err)
		return
	}

	simOutput := make(chan runStatus, len(tests))
	simJobs := make(chan string, numSimJobs)
	go createSimJobs(tests, simJobs)

	for i := 0; i < numSimJobs; i++ {
		go runSimJobs(simJobs, simOutput)
	}

	for i := 0; i < len(tests); i++ {
		result := <-simOutput
		if result.success {
			fmt.Println("Success running ", result.testPath)
		} else {
			fmt.Println("Failed running ", result.testPath, "\n", result.message)
		}
	}

	fmt.Println("foo")
}
