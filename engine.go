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
	"path"
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
	Run         RunSpec // simulation runs to conduct as part of this test
	Checks      []*TestCase
	simStatus   []runStatus // status of all simulation runs
}

// RunSpec describes an individual run to be conducted as part of a single
// mcell test.
type RunSpec struct {
	MdlFiles        []string // name of mdl file to run
	NumSeeds        int      // number of seeds to run
	CommandlineOpts []string // commandline options for this run
	seed            int      // seed value for this particular run
	runID           int      // unique ID for this run needed to collect results for multi seed runs
}

// TestCase describes an individual test case of an overall test
type TestCase struct {
	TestType         string            // test type - used to dispatch appropriate testing function
	Description      string            // textual description of test case
	HaveHeader       bool              // indicates if DataFile contains a header
	AverageData      bool              // test averaged data (only useful for multiple seeds)
	DataFile         string            // name of (output) file to test
	ReferenceFile    string            // name of file with reference counts to compare against
	MinTime          float64           // ignore all data items before MinTime for testing
	MaxTime          float64           // ignore all data items after MaxTime for testing
	Means            []float64         // target column means for count equilibrium tests
	Tolerances       []float64         // tolerances by which actual colummn means may deviate from target
	CountConstraints []*ConstraintSpec // test if counts fullfill the provided constraints
	CountMaximum     []int             // test if counts are larger than provided minmum
	CountMinimum     []int             // test if counts are smaller than provided maximum
	MatchPattern     string            // test pattern to match file against
	NumMatches       int               // number of expected pattern matches
	ExitCode         int               // expected exit code of MCell run
}

// runStatus encapsulating the status of running of of N mdl files which make
// up a single test case
// NOTE: a run might fail for a number of reasons, e.g., during preparation of
// a run and patching in stderr, or during running of MCell itself. If running
// MCell failed we try to figure out the exit code.
type runStatus struct {
	success       bool // indicates if prepping/running the simulation succeeded
	exitMessage   string
	stdErrContent string
	exitCode      int // this is only used if mcell was actually run
}

// ConstraintSpec encapsulates a single constraint specification.
type ConstraintSpec struct {
	Target int
	Query  []int
}

// Copy member function for a TestDescription
func (t *TestDescription) Copy() *TestDescription {
	newT := TestDescription{t.Description, t.Path, t.Run, t.Checks, nil}
	return &newT
}

// runTests runs the specified list of tests
func runTests(tests []string) (int, error) {

	if err := cleanOutput(tests); err != nil {
		fmt.Println("Failed to clean up previous test results", err)
		return 0, err
	}

	simJobs := make(chan *TestDescription, numSimJobs)
	go createSimJobs(tests, simJobs)

	// framework for running simulations
	simOutput := make(chan *TestDescription, len(tests))
	simsDone := make(chan struct{}, numSimJobs)
	for i := 0; i < numSimJobs; i++ {
		go runSimJobs(simOutput, simJobs, simsDone)
	}
	go closeSimOutput(simOutput, simsDone, numSimJobs)

	// framework for collecting simulation results and funneling them into tests
	testInput := make(chan *TestDescription, len(tests))
	go collectSimResults(testInput, simOutput)

	// framework for running tests
	testResults := make(chan *testResult, len(tests))
	testsDone := make(chan struct{}, numTestJobs)
	for i := 0; i < numTestJobs; i++ {
		go runTestJobs(testResults, testInput, testsDone)
	}

	numTests := processResults(testResults, testsDone, numTestJobs)
	return numTests, nil
}

// collectSimResults collects all simulation results (e.g. multiple seeds) for
// a single test case and dispatches them to the tester once they are done.
func collectSimResults(testInput chan *TestDescription,
	simOutput chan *TestDescription) {

	simMap := make(map[int]int)
	simResultsAccum := make([]runStatus, 0)
	for sim := range simOutput {

		numSeeds := sim.Run.NumSeeds
		// for a single seed run we can forward the output to the testing framework right away
		if numSeeds == 1 {
			testInput <- sim
		} else {
			id := sim.Run.runID
			if v, ok := simMap[id]; ok {
				simMap[id] = v + 1
			} else {
				simMap[id] = 1
			}

			simResultsAccum = append(simResultsAccum, sim.simStatus...)

			if simMap[id] == numSeeds {
				// append final list of results
				sim.simStatus = simResultsAccum
				testInput <- sim
			}
		}
	}
	close(testInput)
}

// simRunner runs mcell on the mdl file passed in as an
// absolute path. The working directory is set to the base path
// of the mdl file.
func simRunner(test *TestDescription, output chan *TestDescription) {

	outputDir := filepath.Join(test.Path, "output")
	for i, runFile := range test.Run.MdlFiles {
		// create run command
		mdlPath := filepath.Join(test.Path, runFile)
		runLog := fmt.Sprintf("run_%d.%d.log", test.Run.seed, i)
		errLog := fmt.Sprintf("err_%d.%d.log", test.Run.seed, i)
		argList := append(test.Run.CommandlineOpts, "-seed", strconv.Itoa(test.Run.seed),
			"-logfile", runLog, "-errfile", errLog, mdlPath)
		cmd := exec.Command(mcellPath, argList...)
		cmd.Dir = outputDir

		if err := WriteCmdLine(mcellPath, outputDir, argList); err != nil {
			test.simStatus = append(test.simStatus, runStatus{false, fmt.Sprint(err), "", -1})
			output <- test
			return
		}

		// connect stdout and stderr
		stdOutPath := fmt.Sprintf("stdout_%d.%d.log", test.Run.seed, i)
		stdOut, err := os.Create(filepath.Join(outputDir, stdOutPath))
		if err != nil {
			test.simStatus = append(test.simStatus, runStatus{false, fmt.Sprint(err), "", -1})
			output <- test
			return
		}
		defer stdOut.Close()
		cmd.Stdout = stdOut

		stdErrPath := fmt.Sprintf("stderr_%d.%d.log", test.Run.seed, i)
		stdErr, err := os.Create(filepath.Join(outputDir, stdErrPath))
		if err != nil {
			test.simStatus = append(test.simStatus, runStatus{false, fmt.Sprint(err), "", -1})
			output <- test
			return
		}
		defer stdErr.Close()
		cmd.Stderr = stdErr

		err = cmd.Run()
		if err != nil {
			stdErrContent, _ := ioutil.ReadFile(filepath.Join(outputDir, errLog))
			exitCode, err := determineExitCode(err)
			if err != nil {
				exitCode = -1
			}
			test.simStatus = append(test.simStatus, runStatus{false, fmt.Sprint(err),
				string(stdErrContent), exitCode})
		} else {
			test.simStatus = append(test.simStatus, runStatus{true, "", "", 0})
		}
	}
	output <- test
}

// createSimJobs is responsible for filling a worker queue with
// jobs to be run via the simulation tool. It parses the test
// description, assembles a TestDescription struct and adds it
// to the simulation job queue.
func createSimJobs(testPaths []string, simJobs chan *TestDescription) {
	runID := 0
	for _, testDir := range testPaths {
		testDescription, err := ParseJSON(testDir)
		if err != nil {
			log.Printf("Error parsing test description in %s: %v", testDir, err)
			continue
		}

		// create output directory
		outputDir := filepath.Join(testDir, "output")
		if err := os.Mkdir(outputDir, 0744); err != nil {
			log.Print(err)
			continue
		}

		// set path and pick a seed value for run
		testDescription.Path = testDir
		testDescription.Run.runID = runID

		// schedule requested number of seeds; if there is just a single
		// seed requested we pick one randomly
		switch testDescription.Run.NumSeeds {
		case 0: // user didn't set number of seeds -- assume single seed
			testDescription.Run.NumSeeds = 1
			testDescription.Run.seed = rng.Intn(10000)
		case 1:
			testDescription.Run.seed = rng.Intn(10000)
		default:
			for i := 1; i < testDescription.Run.NumSeeds; i++ {
				newTest := testDescription.Copy()
				newTest.Run.seed = i
				testDescription.Run.seed = i + 1
				simJobs <- newTest
			}
		}
		simJobs <- testDescription
		runID++
	}
	close(simJobs)
}

// showTestDescription shows the description for the selected set of
// tests.
func showTestDescription(testPaths []string) {
	for _, testDir := range testPaths {
		testDescription, err := ParseJSON(testDir)
		if err != nil {
			log.Printf("Error parsing test description in %s: %v", testDir, err)
			continue
		}
		fmt.Println("test name: ", path.Base(testDir))
		fmt.Println("--------------------------------------------------------------")
		fmt.Println(testDescription.Description)
		fmt.Println()
	}
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
func processResults(results chan *testResult, testsDone chan struct{},
	numTestJobs int) int {

	numTests := 0
	t := 0
	for t < numTestJobs {
		select {
		case r := <-results:
			numTests += 1
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
			numTests += 1
			printResult(r)
		default:
			break Done
		}
	}

	return numTests
}

// printResults displays the outcome for a single test result
func printResult(result *testResult) {

	testName := filepath.Base(result.path)
	if result.success {
		fmt.Printf("%-43s ::   %-20s            [SUCCESS]\n", testName, result.testName)
	} else {
		fmt.Printf("%-43s ::   %-20s         ***[FAILURE]***\n", testName, result.testName)
		if result.errorMessage != "" {
			fmt.Println("\t ERROR: ", result.errorMessage)
			// we also try to retrieve the content of stderr
		}
	}
}
