// Copyright 2014 Markus Dittrich. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// nutmeg is a unit and regression test framework for MCell
package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// global settings
// NOTE: With exception of rng these should eventually come from a settings file
var rng *rand.Rand

// command line flags
var listTestsFlag bool
var listCategoriesFlag bool
var cleanTestOutput bool
var testSelection string
var categorySelection string
var descriptionSelectionShort string
var numSimJobs int
var numTestJobs int

// Config keeps track of package configuration settings
type config struct {
	McellPath string // path to mcell executable
	TestDir   string // path to directory with nutmeg tests
}

// initialize list of available unit tests
func init() {
	flag.BoolVar(&listTestsFlag, "l", false, "show available test cases")
	flag.BoolVar(&listCategoriesFlag, "L", false, "show available test categories")
	flag.BoolVar(&cleanTestOutput, "c", false, "clean temporary test data")
	flag.StringVar(&testSelection, "r", "", "run specified tests (i, i:j, 'all')")
	flag.StringVar(&categorySelection, "R", "", "run all tests within the given category")
	flag.StringVar(&descriptionSelectionShort, "d", "",
		"show description for selected tests (i, i:j, or 'all')")
	flag.IntVar(&numSimJobs, "n", 2, "number of concurrent simulation jobs (default: 2)")
	flag.IntVar(&numTestJobs, "m", 2, "number of concurrent test jobs (default: 2)")

	rng = rand.New(rand.NewSource(time.Now().UnixNano()))
}

// main routine
func main() {

	nutmegConf, err := readConfig()
	if err != nil {
		log.Fatal("Error reading nutmeg.conf: ", err)
	}

	startTime := time.Now()

	testNames, err := gatherTests(nutmegConf.TestDir)
	if err != nil {
		log.Fatal("Could not determine list of available test cases")
	}

	flag.Parse()
	if (testSelection != "") && (categorySelection != "") {
		log.Fatal("The r and R flags are mutually exclusive")
	}
	switch {
	case listTestsFlag:
		fmt.Println("Available tests:")
		fmt.Println("----------------")
		for i, t := range testNames {
			fmt.Printf("[%d] %-20s\n", i, t)
		}

	case listCategoriesFlag:
		fmt.Println("Available Categories:")
		fmt.Println("--------------------")
		categories := extractCategories(nutmegConf.TestDir, testNames)
		for k := range categories {
			fmt.Println(" -", k)
		}

	case cleanTestOutput:
		testPaths := make([]string, len(testNames))
		for i, t := range testNames {
			testPaths[i] = filepath.Join(nutmegConf.TestDir, t)
		}
		if err := cleanOutput(testPaths); err != nil {
			log.Fatal(err)
		}

	case descriptionSelectionShort != "":
		tests := extractTestCases(nutmegConf.TestDir, descriptionSelectionShort,
			testNames)
		showTestDescription(tests)

	case categorySelection != "":
		testSelection = strings.TrimSpace(testSelection)
		categoryMap := extractCategories(nutmegConf.TestDir, testNames)
		if ts, ok := categoryMap[testSelection]; ok {
			spawnTests(ts, nutmegConf.McellPath, startTime)
		}

	case testSelection != "":
		testSelection = strings.TrimSpace(testSelection)

		// check if all tests were requested
		var tests []string
		if testSelection == "all" {
			tests = extractAllTestCases(nutmegConf.TestDir, testNames)
		} else {
			tests = extractTestCases(nutmegConf.TestDir, testSelection, testNames)
		}
		spawnTests(tests, nutmegConf.McellPath, startTime)

	default:
		flag.PrintDefaults()
	}
}

// extractTestCases parses the test selection string and assembles the list
// of requested test cases as fully qualified paths.
// NOTE: The form of the selection string is of the form
//              1,2,5:10,55 or testName
//
// Here, each number or range of numbers refers to indexed test cases as
// provided by the -s commandline flag.
func extractTestCases(testDir, selection string, testNames []string) []string {

	var selectedNames []string
	for _, s := range strings.Split(selection, ",") {
		item := strings.TrimSpace(s)

		var items []int
		var err error
		if strings.Contains(item, ":") {
			if items, err = convertRangeToList(item); err != nil {
				log.Printf(fmt.Sprint(err))
				continue
			}
			selectedNames = appendTestCases(items, selectedNames, testNames)
		} else if i, err := strconv.Atoi(item); err == nil {
			selectedNames = appendTestCases([]int{i}, selectedNames, testNames)
		} else { // item provided corresponds to a test name, make sure it exists
			if testNames[sort.SearchStrings(testNames, item)] != item {
				continue // if we can't find the requested testcase we just skip it
			}
			selectedNames = append(selectedNames, item)
		}
	}

	testPaths := make([]string, len(selectedNames))
	for i, name := range selectedNames {
		testPaths[i] = filepath.Join(testDir, name)
	}
	return testPaths
}

// extractAllTestCases returns a list with full paths to all available
// testcases
func extractAllTestCases(testDir string, testNames []string) []string {

	testPaths := make([]string, len(testNames))
	for i, name := range testNames {
		testPaths[i] = filepath.Join(testDir, name)
	}
	return testPaths
}

// appendTestCases appends the test case names corresponding to the provided
// ids to the list of testcases
func appendTestCases(testIDs []int, selection, testNames []string) []string {

	for _, i := range testIDs {
		if i < 0 || i >= len(testNames) {
			log.Printf("test selection %d out of valid range (%d-%d) ... skipping",
				i, 0, len(testNames)-1)
			continue
		}
		selection = append(selection, testNames[i])
	}

	return selection
}

// gatherTests determines the list of available test cases and orders them
// alphabetically
func gatherTests(testDir string) ([]string, error) {
	dirContent, err := ioutil.ReadDir(testDir)
	if err != nil {
		return nil, err
	}

	var tests []string
	for _, c := range dirContent {
		if c.IsDir() {
			tests = append(tests, c.Name())
		}
	}
	sort.Strings(tests)

	return tests, nil
}

// extractCategories extracts the available test categories from the provided
// test selection (x, x:y, 'all', etc.)
func extractCategories(testDir string, testNames []string) map[string][]string {
	tests := extractAllTestCases(testDir, testNames)
	categoryMap := make(map[string][]string)
	for _, t := range tests {
		p, err := ParseJSON(t)
		if err != nil {
			continue
		}
		for _, k := range p.KeyWords {
			if _, ok := categoryMap[k]; !ok {
				categoryMap[k] = []string{t}
			} else {
				categoryMap[k] = append(categoryMap[k], t)
			}
		}
	}
	return categoryMap
}

// spawnTests starts the test engine with the user selected tests and
// prints a status message once they're all finished.
func spawnTests(tests []string, mcellPath string, startTime time.Time) {
	numGoodTests, badTests, _ := runTests(mcellPath, tests)
	numBadTests := len(badTests)
	fmt.Println("-------------------------------------")
	fmt.Printf("Ran %d tests in %f s:  SUCCESSES[%d]  FAILURES[%d]\n",
		(numGoodTests + numBadTests), time.Since(startTime).Seconds(),
		numGoodTests, numBadTests)

	if numBadTests > 0 {
		fmt.Println("")
		for i, t := range badTests {
			fmt.Printf("**** FAILED TEST %d: %s :: %s ****\n", i+1, filepath.Base(t.path), t.testName)
			fmt.Printf("\n\t%s\n\n", t.errorMessage)
		}
	}
}
