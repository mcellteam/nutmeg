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

const (
	numSimJobs  = 2
	numTestJobs = 2
)

var tests []string
var mcell_path string
var rng *rand.Rand

// data structure encapsulating the status of running the
// mdl files underlying an mcell test
type runStatus struct {
	success bool // indicates if prepping/running the simulation succeeded
	message string
}

// TestResults encapsulates the results of an individual test
type TestResult struct {
	path         string // path to test with was run
	success      bool   // was test successful
	testName     string // name of test
	errorMessage string // error message if test failed
}

// initialize list of available unit tests
func init() {
	//tests = []string{}
	tests = []string{
		"/Users/markus/programming/go/src/github.com/haskelladdict/nutmeg/tests/remove_per_species_list_from_ht",
		"/Users/markus/programming/go/src/github.com/haskelladdict/nutmeg/tests/orient_flip_flip_rxn",
		"/Users/markus/programming/go/src/github.com/haskelladdict/nutmeg/tests/coincident_surfaces"}

	mcell_path = "/Users/markus/programming/c/mcell/mcell-trunk/build/mcell"

	rng = rand.New(rand.NewSource(time.Now().UnixNano()))
}

// simRunner runs mcell on the mdl file passed in as an
// absolute path. The working directory is set to the base path
// of the mdl file.
func simRunner(test *TestDescription, output chan *TestDescription) {

	mdlPath := filepath.Join(test.Path, "test.mdl")
	argList := append(test.CommandlineOpts, "-seed", strconv.Itoa(rng.Intn(10000)),
		"-logfile", "run.log", "-errfile", "err.log", mdlPath)
	cmd := exec.Command(mcell_path, argList...)

	// create outputDir
	outputDir := filepath.Join(test.Path, "output")
	if err := os.Mkdir(outputDir, 0744); err != nil {
		test.simStatus = runStatus{false, fmt.Sprint(err)}
		output <- test
		return
	}
	cmd.Dir = outputDir

	if err := WriteCmdLine(mcell_path, outputDir, argList); err != nil {
		test.simStatus = runStatus{false, fmt.Sprint(err)}
		output <- test
		return
	}

	// connect stdout and stderr
	stdOut, err := os.Create(filepath.Join(outputDir, "stdout.log"))
	if err != nil {
		test.simStatus = runStatus{false, fmt.Sprint(err)}
		output <- test
		return
	}
	defer stdOut.Close()
	cmd.Stdout = stdOut

	stdErr, err := os.Create(filepath.Join(outputDir, "stderr.log"))
	if err != nil {
		test.simStatus = runStatus{false, fmt.Sprint(err)}
		output <- test
		return
	}
	defer stdErr.Close()
	cmd.Stderr = stdErr

	err = cmd.Run()
	if err != nil {
		test.simStatus = runStatus{false, fmt.Sprint(err)}
	} else {
		test.simStatus = runStatus{true, ""}
	}
	output <- test
}

// createSimJobs is responsible for filling a worker queue with
// jobs to be run via the simulation tool. It parses the test
// description, assembles a TestDescription struct and adds it
// to the simulation job queue.
func createSimJobs(tests []string, simJobs chan *TestDescription) {
	for _, testDir := range tests {
		testDescription, err := ParseJSON(testDir)
		if err != nil {
			log.Printf("Error parsing test description in %s: %v", testDir, err)
			continue
		}
		//fmt.Println("Successfully parsed test description:", testDescription)
		testDescription.Path = testDir
		simJobs <- testDescription
	}
	close(simJobs)
}

// runSimJobs loops over all available jobs and runs each of
// them in a simRunner.
func runSimJobs(simOutput chan *TestDescription, tests <-chan *TestDescription) {
	for test := range tests {
		simRunner(test, simOutput)
	}
	close(simOutput)
}

// runTestJobs loops over all available TestDescriptions coming from the
// simulation engine and submits them to a test engine.
func runTestJobs(result chan *TestResult, tests <-chan *TestDescription) {
	for test := range tests {
		testRunner(test, result)
	}
	close(result)
}

// main routine
func main() {

	if err := CleanOutput(tests); err != nil {
		fmt.Println("Failed to clean up previous test results", err)
		return
	}

	simJobs := make(chan *TestDescription, numSimJobs)
	go createSimJobs(tests, simJobs)

	simOutput := make(chan *TestDescription, len(tests))
	for i := 0; i < numSimJobs; i++ {
		go runSimJobs(simOutput, simJobs)
	}

	testResults := make(chan *TestResult, len(tests))
	for i := 0; i < numTestJobs; i++ {
		go runTestJobs(testResults, simOutput)
	}

	for result := range testResults {
		testName := filepath.Base(result.path)
		if result.success {
			fmt.Printf("%-35s ::   %-20s            [SUCCESS]\n", testName, result.testName)
		} else {
			fmt.Printf("%-35s ::   %-20s         ***[FAILURE]***\n", testName, result.testName)
			if result.errorMessage != "" {
				fmt.Println("\t\t --> ", result.errorMessage)
			}
		}
	}

	fmt.Println("done - all good")
}
