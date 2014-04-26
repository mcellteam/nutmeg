// Copyright 2014 Markus Dittrich. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// test_engine contains the actual test functions analysing
// the output of the run MCell simulations
package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
)

// TestResults encapsulates the results of an individual test
type testResult struct {
	path         string // path to test with was run
	success      bool   // was test successful
	testName     string // name of test
	errorMessage string // error message if test failed
}

// TestDescription encapsulates all information needed to describe a unit
// or regression test of an MCell model
type TestDescription struct {
	Description string
	Path        string
	Checks      []*TestCase
	Runs        []*RunSpec   // simulation runs to conduct as part of this test
	simStatus   []*runStatus // status of all simulation runs
}

// RunSpec describes an individual run to be conducted as part of a single
// mcell test.
type RunSpec struct {
	MdlFile         string   // name of mdl file to run
	CommandlineOpts []string // commandline options for this run
}

// TestCase describes an individual test case of an overall test
type TestCase struct {
	TestType         string            // test type - used to dispatch appropriate testing function
	Description      string            // textual description of test case
	HaveHeader       bool              // indicates if DataFile contains a header
	DataFile         string            // name of (output) file to test
	ReferenceFile    string            // name of file with reference counts to compare against
	MinTime          float64           // ignore all data items before MinTime for testing
	MaxTime          float64           // ignore all data items after MaxTime for testing
	CountConstraints []*ConstraintSpec // test if counts fullfill the provided constraints
	CountMaximum     []int             // test if counts are larger than provided minmum
	CountMinimum     []int             // test if counts are smaller than provided maximum
	MatchPattern     string            // test pattern to match file against
	NumMatches       int               // number of expected pattern matches
}

// runStatus encapsulating the status of running of of N mdl files which make
// up a single test case
type runStatus struct {
	success       bool // indicates if prepping/running the simulation succeeded
	exitMessage   string
	stdErrContent string
}

// ConstraintSpec encapsulates a single constraint specification.
type ConstraintSpec struct {
	Target int
	Query  []int
}

// runTests runs the specified list of tests
func runTests(tests []string) {

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

	testResults := make(chan *testResult, len(tests))
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
	// create outputDir
	outputDir := filepath.Join(test.Path, "output")
	if err := os.Mkdir(outputDir, 0744); err != nil {
		test.simStatus = append(test.simStatus, &runStatus{false, fmt.Sprint(err), ""})
		output <- test
		return
	}

	for i, run := range test.Runs {
		// create run command
		mdlPath := filepath.Join(test.Path, run.MdlFile)
		runLog := fmt.Sprintf("run_%d.log", i)
		errLog := fmt.Sprintf("err_%d.log", i)
		argList := append(run.CommandlineOpts, "-seed", strconv.Itoa(rng.Intn(10000)),
			"-logfile", runLog, "-errfile", errLog, mdlPath)
		cmd := exec.Command(mcellPath, argList...)
		cmd.Dir = outputDir

		if err := WriteCmdLine(mcellPath, outputDir, argList); err != nil {
			test.simStatus = append(test.simStatus, &runStatus{false, fmt.Sprint(err), ""})
			output <- test
			return
		}

		// connect stdout and stderr
		stdOutPath := fmt.Sprintf("stdout_%d.log", i)
		stdOut, err := os.Create(filepath.Join(outputDir, stdOutPath))
		if err != nil {
			test.simStatus = append(test.simStatus, &runStatus{false, fmt.Sprint(err), ""})
			output <- test
			return
		}
		defer stdOut.Close()
		cmd.Stdout = stdOut

		stdErrPath := fmt.Sprintf("stderr_%d.log", i)
		stdErr, err := os.Create(filepath.Join(outputDir, stdErrPath))
		if err != nil {
			test.simStatus = append(test.simStatus, &runStatus{false, fmt.Sprint(err), ""})
			output <- test
			return
		}
		defer stdErr.Close()
		cmd.Stderr = stdErr

		err = cmd.Run()
		if err != nil {
			stdErrContent, _ := ioutil.ReadFile(filepath.Join(outputDir, errLog))
			test.simStatus = append(test.simStatus, &runStatus{false, fmt.Sprint(err),
				string(stdErrContent)})
		} else {
			test.simStatus = append(test.simStatus, &runStatus{true, "", ""})
		}
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
func runTestJobs(results chan *testResult, simOutput <-chan *TestDescription,
	testsDone chan struct{}) {
	for test := range simOutput {
		testRunner(test, results)
	}
	testsDone <- struct{}{}
}

// processResults process all produced test results and displays them in the
// fashion requested
func processResults(results chan *testResult, testsDone chan struct{}, numTestJobs int) {

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
func printResult(result *testResult) {

	testName := filepath.Base(result.path)
	if result.success {
		fmt.Printf("%-35s ::   %-20s            [SUCCESS]\n", testName, result.testName)
	} else {
		fmt.Printf("%-35s ::   %-20s         ***[FAILURE]***\n", testName, result.testName)
		if result.errorMessage != "" {
			fmt.Println("\t ERROR: ", result.errorMessage)
			// we also try to retrieve the content of stderr
		}
	}
}
