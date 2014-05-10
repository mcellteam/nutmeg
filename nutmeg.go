// Copyright 2014 Markus Dittrich. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// nutmeg is a unit and regression test framework for MCell
package main

import (
  "errors"
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
var mcellPath string
var testDir string
var rng *rand.Rand

// command line flags
var listTestsFlag bool
var cleanTestOutput bool
var testSelection string
var descriptionSelectionShort string
var numSimJobs int
var numTestJobs int

// initialize list of available unit tests
func init() {
  flag.BoolVar(&listTestsFlag, "l", false, "show available test cases")
  flag.BoolVar(&cleanTestOutput, "c", false, "clean temporary test data")
  flag.StringVar(&testSelection, "r", "", "run specified tests (i, i:j, or 'all')")
  flag.StringVar(&descriptionSelectionShort, "d", "", "show description for selected tests (i, i:j, or 'all')")
  flag.IntVar(&numSimJobs, "n", 2, "number of concurrent simulation jobs (default: 2)")
  flag.IntVar(&numTestJobs, "m", 2, "number of concurrent test jobs (default: 2)")

  rng = rand.New(rand.NewSource(time.Now().UnixNano()))
  testDir = "/Users/markus/programming/go/src/github.com/haskelladdict/nutmeg/tests/"
  mcellPath = "/Users/markus/programming/c/mcell/mcell-trunk/build/mcell"
}

// main routine
func main() {

  startTime := time.Now()

  testNames, err := gatherTests(testDir)
  if err != nil {
    log.Fatal("Could not determine list of available test cases")
  }

  flag.Parse()

  switch {
  case listTestsFlag:
    fmt.Println("Available tests:")
    fmt.Println("----------------")
    for i, t := range testNames {
      fmt.Printf("[%d] %-20s\n", i, t)
    }

  case cleanTestOutput:
    tests := extractTestCases(testSelection, testNames)
    if err := cleanOutput(tests); err != nil {
      log.Fatal(err)
    }

  case descriptionSelectionShort != "":
    tests := extractTestCases(descriptionSelectionShort, testNames)
    showTestDescription(tests)

  case testSelection != "":
    tests := extractTestCases(testSelection, testNames)
    numGoodTests, numBadTests, _ := runTests(tests)
    fmt.Println("-------------------------------------")
    fmt.Printf("Ran %d tests in %f s:  SUCCESS[%d]  FAILURE[%d]\n",
      (numGoodTests + numBadTests), time.Now().Sub(startTime).Seconds(),
      numGoodTests, numBadTests)

  default:
    flag.PrintDefaults()
  }

}

// extractTestCases parses the test selection string and assembles the list
// of requested test cases as fully qualified paths.
// NOTE: The form of the selection string is of the form
//              1,2,5:10,55
//
// Here, each number or range of numbers refers to indexed test cases as
// provided by the -s commandline flag.
// A special case is "all" which refers to all tests.
func extractTestCases(selection string, testNames []string) []string {

  var selectedNames []string
  if selection == "all" {
    selectedNames = testNames
  } else {
    for _, s := range strings.Split(selection, ",") {
      item := strings.TrimSpace(s)

      var items []int
      var err error
      if strings.Contains(item, ":") {
        if items, err = convertRangeToList(item); err != nil {
          log.Printf(fmt.Sprint(err))
          continue
        }
      } else {
        i, err := strconv.Atoi(item)
        if err != nil {
          log.Printf("invalid test selection %s ... skipping", item)
          continue
        }
        items = []int{i}
      }

      for _, i := range items {
        if i < 0 || i >= len(testNames) {
          log.Printf("test selection %d out of valid range (%d-%d) ... skipping",
            i, 0, len(testNames)-1)
          continue
        }
        selectedNames = append(selectedNames, testNames[i])
      }
    }
  }

  testPaths := make([]string, len(selectedNames))
  for i, name := range selectedNames {
    testPaths[i] = filepath.Join(testDir, name)
  }
  return testPaths
}

// convertRangeToList converts a single string containing a range statement
// of the form "4:9" into an explicit integer list describing the
// range [4, 5, 6, 7, 8, 9]
func convertRangeToList(rangeStatement string) ([]int, error) {

  rangeEndpoints := strings.Split(rangeStatement, ":")
  if len(rangeEndpoints) != 2 {
    return nil, errors.New(
      fmt.Sprintf("range selection %s not valid", rangeStatement))
  }

  var rangeBegin int
  var err error
  if rangeBegin, err = strconv.Atoi(rangeEndpoints[0]); err != nil {
    return nil, errors.New(
      fmt.Sprintf("invalid range start character %s", rangeEndpoints[0]))
  }

  var rangeEnd int
  if rangeEnd, err = strconv.Atoi(rangeEndpoints[1]); err != nil {
    return nil, errors.New(
      fmt.Sprintf("invalid range end character %s", rangeEndpoints[1]))
  }

  var newRange []int
  for i := rangeBegin; i <= rangeEnd; i++ {
    newRange = append(newRange, i)
  }

  return newRange, nil
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
