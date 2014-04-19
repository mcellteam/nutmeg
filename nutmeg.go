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
var mcellPath string
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
	rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	testPath := "/Users/markus/programming/go/src/github.com/haskelladdict/nutmeg/tests/"
	mcellPath = "/Users/markus/programming/c/mcell/mcell-trunk/build/mcell"

	testNames := []string{
		"remove_per_species_list_from_ht",
		"orient_flip_flip_rxn",
		"coincident_surfaces",
		"rx_flip_flip"}
	tests = make([]string, len(testNames))
	for i, name := range testNames {
		tests[i] = filepath.Join(testPath, name)
	}
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
	simsDone := make(chan struct{}, numSimJobs)
	for i := 0; i < numSimJobs; i++ {
		go runSimJobs(simOutput, simJobs, simsDone)
	}
	go closeSimOutput(simOutput, simsDone, numSimJobs)

	testResults := make(chan *TestResult, len(tests))
	testsDone := make(chan struct{}, numTestJobs)
	for i := 0; i < numTestJobs; i++ {
		go runTestJobs(testResults, simOutput, testsDone)
	}

	processResults(testResults, testsDone, numTestJobs)
	fmt.Println("done - all good")
}

// simRunner runs mcell on the mdl file passed in as an
// absolute path. The working directory is set to the base path
// of the mdl file.
func simRunner(test *TestDescription, output chan *TestDescription) {

	mdlPath := filepath.Join(test.Path, "test.mdl")
	argList := append(test.CommandlineOpts, "-seed", strconv.Itoa(rng.Intn(10000)),
		"-logfile", "run.log", "-errfile", "err.log", mdlPath)
	cmd := exec.Command(mcellPath, argList...)

	// create outputDir
	outputDir := filepath.Join(test.Path, "output")
	if err := os.Mkdir(outputDir, 0744); err != nil {
		test.simStatus = runStatus{false, fmt.Sprint(err)}
		output <- test
		return
	}
	cmd.Dir = outputDir

	if err := WriteCmdLine(mcellPath, outputDir, argList); err != nil {
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
func createSimJobs(testPaths []string, simJobs chan *TestDescription) {
	for _, testDir := range testPaths {
		testDescription, err := ParseJSON(testDir)
		if err != nil {
			log.Printf("Error parsing test description in %s: %v", testDir, err)
			continue
		}
		testDescription.Path = testDir
		simJobs <- testDescription
	}
	close(simJobs)
}

// runSimJobs loops over all available jobs and runs each of
// them in a simRunner.
func runSimJobs(simOutput chan *TestDescription, simJobs <-chan *TestDescription,
	simsDone chan struct{}) {
	for job := range simJobs {
		simRunner(job, simOutput)
	}
	simsDone <- struct{}{}
}

// closeSimOutput is in charge of closing the simOutput channels once all
// simRunners have finished.
func closeSimOutput(simOutput chan *TestDescription, simsDone chan struct{},
	numSimJobs int) {

	for i := 0; i < numSimJobs; i++ {
		<-simsDone
	}
	close(simOutput)
}

// runTestJobs loops over all available TestDescriptions coming from the
// simulation engine and submits them to a test engine.
func runTestJobs(results chan *TestResult, simOutput <-chan *TestDescription,
	testsDone chan struct{}) {
	for test := range simOutput {
		testRunner(test, results)
	}
	testsDone <- struct{}{}
}

// processResults process all produced test results and displays them in the
// fashion requested
func processResults(results chan *TestResult, testsDone chan struct{}, numTestJobs int) {

	t := 0
	for t < numTestJobs {
		select {
		case r := <-results:
			printResult(r)
		case <-testsDone:
			t += 1
		}
	}

	// clear out remaining test result queue
Done:
	for {
		select {
		case r := <-results:
			printResult(r)
		default:
			break Done
		}
	}
}

// printResults displays the outcome for a single test result
func printResult(result *TestResult) {

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
