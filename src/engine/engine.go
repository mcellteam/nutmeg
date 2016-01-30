// Copyright 2014-2016 Markus Dittrich. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package engine contains the actual test functions analysing
// the output of the run MCell simulations
package engine

import (
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"time"

	"github.com/mcellteam/nutmeg/src/file"
	"github.com/mcellteam/nutmeg/src/jsonParser"
	"github.com/mcellteam/nutmeg/src/misc"
	"github.com/mcellteam/nutmeg/src/tester"
)

// initialize random number generator
var rng *rand.Rand

func init() {
	rng = rand.New(rand.NewSource(time.Now().UnixNano()))
}

// RunTests runs the specified list of tests
func RunTests(conf *jsonParser.Config, tests []string,
	numSimJobs, numTestJobs int) (int, []*tester.TestResult, error) {

	if err := misc.CleanOutput(tests); err != nil {
		fmt.Println("Failed to clean up previous test results", err)
		return 0, nil, err
	}

	testResults := make(chan *tester.TestResult, len(tests))
	simJobs := make(chan *jsonParser.TestDescription, numSimJobs)
	go createSimJobs(conf.IncludeDir, tests, simJobs, testResults)

	// framework for running simulations
	simOutput := make(chan *jsonParser.TestDescription, len(tests))
	simsDone := make(chan struct{}, numSimJobs)
	for i := 0; i < numSimJobs; i++ {
		go runSimJobs(conf.McellPath, simOutput, simJobs, simsDone)
	}
	go closeSimOutput(simOutput, simsDone, numSimJobs)

	// framework for collecting simulation results and funneling them into tests
	testInput := make(chan *jsonParser.TestDescription, len(tests))
	go collectSimResults(testInput, simOutput)

	// framework for running tests
	testsDone := make(chan struct{}, numTestJobs)
	for i := 0; i < numTestJobs; i++ {
		go runTestJobs(testResults, testInput, testsDone)
	}
	numGoodTests, badTests := processResults(testResults, testsDone, numTestJobs)
	return numGoodTests, badTests, nil
}

// collectSimResults collects all simulation results (e.g. multiple Seeds) for
// a single test case and dispatches them to the tester once they are done.
func collectSimResults(testInput chan *jsonParser.TestDescription,
	simOutput chan *jsonParser.TestDescription) {

	simMap := make(map[int]int)
	var simResultsAccum []jsonParser.RunStatus
	for sim := range simOutput {

		numSeeds := sim.Run.NumSeeds
		// for a single Seed run we can forward the output to the testing framework right away
		if numSeeds == 1 {
			testInput <- sim
		} else {
			id := sim.Run.RunID
			if v, ok := simMap[id]; ok {
				simMap[id] = v + 1
			} else {
				simMap[id] = 1
			}

			simResultsAccum = append(simResultsAccum, sim.SimStatus...)

			if simMap[id] == numSeeds {
				// append final list of results
				sim.SimStatus = simResultsAccum
				testInput <- sim
			}
		}
	}
	close(testInput)
}

// simRunner runs mcell on the mdl file passed in as an
// absolute path. The working directory is set to the base path
// of the mdl file.
func simRunner(mcellPath string, test *jsonParser.TestDescription,
	output chan *jsonParser.TestDescription) {

	outputDir := file.GetOutputDir(test.Path)
	for i, runFile := range test.Run.MdlFiles {
		// create run command
		mdlPath := filepath.Join(test.Path, runFile)
		runLog := fmt.Sprintf("run_%d.%d.log", test.Run.Seed, i)
		errLog := fmt.Sprintf("err_%d.%d.log", test.Run.Seed, i)
		argList := append(test.Run.CommandlineOpts, "-seed", strconv.Itoa(test.Run.Seed),
			"-logfile", runLog, "-errfile", errLog, mdlPath)
		cmd := exec.Command(mcellPath, argList...)
		cmd.Dir = outputDir

		if err := misc.WriteCmdLine(mcellPath, outputDir, argList); err != nil {
			test.SimStatus = append(test.SimStatus,
				jsonParser.RunStatus{Success: false, ExitMessage: fmt.Sprint(err),
					StdErrContent: "", ExitCode: -1})
			output <- test
			return
		}

		// connect stdout and stderr
		stdOutPath := fmt.Sprintf("stdout_%d.%d.log", test.Run.Seed, i)
		stdOut, err := os.Create(filepath.Join(outputDir, stdOutPath))
		if err != nil {
			test.SimStatus = append(test.SimStatus,
				jsonParser.RunStatus{Success: false, ExitMessage: fmt.Sprint(err),
					StdErrContent: "", ExitCode: -1})
			output <- test
			return
		}
		defer stdOut.Close()
		cmd.Stdout = stdOut

		stdErrPath := fmt.Sprintf("stderr_%d.%d.log", test.Run.Seed, i)
		stdErr, err := os.Create(filepath.Join(outputDir, stdErrPath))
		if err != nil {
			test.SimStatus = append(test.SimStatus,
				jsonParser.RunStatus{Success: false, ExitMessage: fmt.Sprint(err),
					StdErrContent: "", ExitCode: -1})
			output <- test
			return
		}
		defer stdErr.Close()
		cmd.Stderr = stdErr

		err = cmd.Run()
		if err != nil {
			stdErr, _ := ioutil.ReadFile(filepath.Join(outputDir, errLog))
			exitCode, err := misc.DetermineExitCode(err)
			if err != nil {
				exitCode = -1
			}
			test.SimStatus = append(test.SimStatus,
				jsonParser.RunStatus{Success: false, ExitMessage: fmt.Sprint(err),
					StdErrContent: string(stdErr), ExitCode: exitCode})
		} else {
			test.SimStatus = append(test.SimStatus,
				jsonParser.RunStatus{Success: true, ExitMessage: "", StdErrContent: "",
					ExitCode: 0})
		}
	}
	output <- test
}

// createSimJobs is responsible for filling a worker queue with
// jobs to be run via the simulation tool. It parses the test
// description, assembles a TestDescription struct and adds it
// to the simulation job queue.
func createSimJobs(includePath string, testPaths []string,
	simJobs chan *jsonParser.TestDescription, testResults chan *tester.TestResult) {
	runID := 0
	for _, testDir := range testPaths {
		testFile := filepath.Join(testDir, "test_description.json")
		testDescription, err := jsonParser.Parse(testFile, includePath)
		if err != nil {
			msg := fmt.Sprintf("Error parsing test description in %s: %v", testDir, err)
			testResults <- &tester.TestResult{Path: testFile, Success: false,
				TestName: "parse description", ErrorMessage: msg}
			continue
		}

		// create output directory
		outputDir := file.GetOutputDir(testDir)
		if err := os.Mkdir(outputDir, 0744); err != nil {
			continue
		}

		// set path and pick a Seed value for run
		testDescription.Path = testDir
		testDescription.Run.RunID = runID

		// schedule requested number of Seeds; if there is just a single
		// Seed requested we pick one randomly
		switch testDescription.Run.NumSeeds {
		case 0: // user didn't set number of Seeds -- assume single Seed
			testDescription.Run.NumSeeds = 1
			testDescription.Run.Seed = rng.Intn(10000)
		case 1:
			testDescription.Run.Seed = rng.Intn(10000)
		default:
			for i := 1; i < testDescription.Run.NumSeeds; i++ {
				newTest := testDescription.Copy()
				newTest.Run.Seed = i
				testDescription.Run.Seed = i + 1
				simJobs <- newTest
			}
		}
		simJobs <- testDescription
		runID++
	}
	close(simJobs)
}

// ShowTestDescription shows the description for the selected set of
// tests.
func ShowTestDescription(conf *jsonParser.Config, testPaths []string) {
	for _, testDir := range testPaths {
		testDescription, err := jsonParser.Parse(filepath.Join(testDir, "test_description.json"),
			conf.IncludeDir)
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
func runSimJobs(mcellPath string, simOutput chan *jsonParser.TestDescription,
	simJobs <-chan *jsonParser.TestDescription,
	simsDone chan struct{}) {
	for job := range simJobs {
		simRunner(mcellPath, job, simOutput)
	}
	simsDone <- struct{}{}
}

// closeSimOutput is in charge of closing the simOutput channels once all
// simRunners have finished.
func closeSimOutput(simOutput chan *jsonParser.TestDescription, simsDone chan struct{},
	numSimJobs int) {

	for i := 0; i < numSimJobs; i++ {
		<-simsDone
	}
	close(simOutput)
}

// runTestJobs loops over all available TestDescriptions coming from the
// simulation engine and submits them to a test engine.
func runTestJobs(results chan *tester.TestResult,
	simOutput <-chan *jsonParser.TestDescription, testsDone chan struct{}) {
	for test := range simOutput {
		tester.Run(test, results)
	}
	testsDone <- struct{}{}
}

// processResults process all produced test results and displays them in the
// fashion requested
func processResults(results chan *tester.TestResult, testsDone chan struct{},
	numTestJobs int) (int, []*tester.TestResult) {

	numGoodTests := 0
	var badTests []*tester.TestResult
	t := 0
	for t < numTestJobs {
		select {
		case r := <-results:
			if r.Success {
				numGoodTests++
			} else {
				badTests = append(badTests, r)
			}
			printResult(r)
		case <-testsDone:
			t++
		}
	}

	// clear out remaining test result queue
Done:
	for {
		select {
		case r := <-results:
			if r.Success {
				numGoodTests++
			} else {
				badTests = append(badTests, r)
			}
			printResult(r)
		default:
			break Done
		}
	}

	return numGoodTests, badTests
}

// printResults displays the outcome for a single test result
func printResult(result *tester.TestResult) {

	testName := filepath.Base(result.Path)
	if result.Success {
		fmt.Printf("%-43s ::   %-25s       [SUCCESS]\n", testName, result.TestName)
	} else {
		fmt.Printf("%-43s ::   %-25s    ***[FAILURE]***\n", testName, result.TestName)
		if result.ErrorMessage != "" {
			fmt.Println("\t ERROR: ", result.ErrorMessage)
			// we also try to retrieve the content of stderr
		}
	}
}
