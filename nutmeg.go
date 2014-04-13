// Copyright 2014 Markus Dittrich. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// nutmeg is a unit and regression test framework for MCell
package main

import (
	"fmt"
	"log"
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

// TestDescription encapsulates all information needed to describe a unit
// or regression test of an MCell model
type TestDescription struct {
	Description     string
	CommandlineOpts []string
	Path            string
	Checks          []*TestCase
}

type TestCase struct {
	TestType string
}

// initialize list of available unit tests
func init() {
	//tests = []string{}
	tests = []string{
		"/Users/markus/programming/go/src/github.com/haskelladdict/nutmeg/tests/remove_per_species_list_from_ht"}

	mcell_path = "/Users/markus/programming/c/mcell/mcell-trunk/build/mcell"

	rng = rand.New(rand.NewSource(time.Now().UnixNano()))
}

// test_runner runs mcell on the mdl file passed in as an
// absolute path. The working directory is set to the base path
// of the mdl file.
func simRunner(test *TestDescription, output chan runStatus) {

	mdlPath := filepath.Join(test.Path, "test.mdl")
	argList := append(test.CommandlineOpts, "-seed", strconv.Itoa(rng.Intn(10000)),
		"-logfile", "run.log", "-errfile", "err.log", mdlPath)
	cmd := exec.Command(mcell_path, argList...)

	// create outputDir
	outputDir := filepath.Join(test.Path, "output")
	if err := os.Mkdir(outputDir, 0744); err != nil {
		output <- runStatus{false, test.Path, fmt.Sprint(err)}
		return
	}
	cmd.Dir = outputDir

	if err := WriteCmdLine(mcell_path, outputDir, argList); err != nil {
		output <- runStatus{false, test.Path, fmt.Sprint(err)}
		return
	}

	// connect stdout and stderr
	stdOut, err := os.Create(filepath.Join(outputDir, "stdout.log"))
	if err != nil {
		output <- runStatus{false, test.Path, "error opening stdout"}
		return
	}
	cmd.Stdout = stdOut

	stdErr, err := os.Create(filepath.Join(outputDir, "stderr.log"))
	if err != nil {
		output <- runStatus{false, test.Path, "error opening stderr"}
		return
	}
	cmd.Stderr = stdErr

	err = cmd.Run()
	if err != nil {
		output <- runStatus{false, test.Path, fmt.Sprint(err)}
	} else {
		output <- runStatus{true, test.Path, ""}
	}
}

// createSimJobs is responsbile for filling the worker queue with
// jobs to be run via the simulation tool. It parses the test
// description, assembles a TestDescription struct and adds it
// to the simulation job queue.
func createSimJobs(tests []string, simJobs chan *TestDescription) {
	for _, testDir := range tests {
		testDescription, err := ParseJSON(testDir)
		if err != nil {
			log.Printf("Error parsing test description: %s: %v", testDir, err)
			continue
		}
		fmt.Println("Successfully parsed test description:", testDescription)
		testDescription.Path = testDir
		simJobs <- testDescription
	}
	close(simJobs)
}

// runSimJobs loops over all available jobs and runs each of
// them in a simRunner.
func runSimJobs(tests <-chan *TestDescription, simOutput chan runStatus) {
	for test := range tests {
		simRunner(test, simOutput)
	}
}

// main routine
func main() {

	if err := CleanOutput(tests); err != nil {
		fmt.Println("Failed to clean up previous test results", err)
		return
	}

	simOutput := make(chan runStatus, len(tests))
	simJobs := make(chan *TestDescription, numSimJobs)
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

	fmt.Println("done - all good")
}
