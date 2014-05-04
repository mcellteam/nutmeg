// Copyright 2014 Markus Dittrich. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// test_engine contains the actual test functions analysing
// the output of the run MCell simulations
package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strings"
)

// testRunner analyses the TestDescriptions coming from an MCell run on a
// test and analyses them as requested per the TestDescription.
func testRunner(test *TestDescription, result chan *testResult) {

	for _, c := range test.Checks {

		dataPaths, err := getDataPaths(test.Path, c.DataFile, test.Run.seed,
			test.Run.NumSeeds)
		if err != nil {
			result <- &testResult{test.Path, false, c.TestType, fmt.Sprint(err)}
			continue
		}

		var data []*Columns
		// NOTE: only attempt to parse data for the relevant test cases
		if c.DataFile != "" && c.TestType != "FILE_MATCH_PATTERN" {
			data, err = loadData(dataPaths, c.HaveHeader, c.AverageData)
			if err != nil {
				result <- &testResult{test.Path, false, c.TestType, fmt.Sprint(err)}
				continue
			}
		}

		var testErr error
		switch c.TestType {
		case "CHECK_SUCCESS":
			if test.simStatus == nil {
				result <- &testResult{test.Path, false, "CHECK_SUCCESS",
					"simulations did not run or return an exit status"}
				return // if simulation fails we won't continue testing
			}

			// in order to cut down on the amount of output (particularly in the case of
			// multiple seeds) we return failure if one or more of all runs within a test
			// fails and success otherwise
			for _, testRun := range test.simStatus {
				if !testRun.success {
					message := strings.Join([]string{testRun.exitMessage, testRun.stdErrContent}, "\n")
					result <- &testResult{test.Path, false, "CHECK_SUCCESS", message}
					return // if simulation fails we won't continue testing
				}
			}

		case "CHECK_EXIT_CODE":
			for _, testRun := range test.simStatus {
				if c.ExitCode != testRun.exitCode {
					testErr = errors.New(fmt.Sprintf("Expected exit code %d but got %d instead",
						c.ExitCode, testRun.exitCode))
				}
			}

		case "COUNT_CONSTRAINTS":
			for i, d := range data {
				if testErr = checkCountConstraints(d, dataPaths[i], c.MinTime, c.MaxTime,
					c.CountConstraints); testErr != nil {
					break
				}
			}

		case "COUNT_MINMAX":
			for i, d := range data {
				if testErr = checkCountMinmax(d, dataPaths[i], c.MinTime, c.MaxTime,
					c.CountMaximum, c.CountMinimum); testErr != nil {
					break
				}
			}

		case "FILE_MATCH_PATTERN":
			for _, dataPath := range dataPaths {
				if testErr = fileMatchPattern(dataPath, c.MatchPattern, c.NumMatches); testErr != nil {
					break
				}
			}

		case "COMPARE_COUNTS":
			referencePath := filepath.Join(test.Path, c.ReferenceFile)
			refData, err := readCounts(referencePath, c.HaveHeader)
			if err != nil {
				break
			}
			for i, d := range data {
				if testErr = compareCounts(d, refData, dataPaths[i], c.MinTime,
					c.MaxTime); testErr != nil {
					break
				}
			}

		case "COUNT_EQUILIBRIUM":
			for i, d := range data {
				if testErr = checkCountEquilibrium(d, dataPaths[i], c.MinTime, c.MaxTime,
					c.Means, c.Tolerances); testErr != nil {
					break
				}
			}

		case "POSITIVE_COUNTS":
			for i, d := range data {
				if testErr = checkPositiveCounts(d, dataPaths[i], c.MinTime,
					c.MaxTime); testErr != nil {
					break
				}
			}

		default:
			recordResult(result, "----------------", test.Path,
				errors.New(fmt.Sprintf("Unknown test type: %s", c.TestType)))
		}
		recordResult(result, c.TestType, test.Path, testErr)
	}
}

// recordResults checks if a test was successfull or not, records
// success/failure in testResult object and sends it to the results channel
func recordResult(result chan<- *testResult, testType string, dataPath string,
	err error) {
	if err != nil {
		result <- &testResult{dataPath, false, testType, fmt.Sprint(err)}
	} else {
		result <- &testResult{dataPath, true, testType, ""}
	}
}

// checkCountConstraints tests the provided array of constraints
// on the simulation output data contained in the file filePath
func checkCountConstraints(data *Columns, dataPath string, minTime, maxTime float64,
	constraints []*ConstraintSpec) error {

	// check constraints for each row of data
	for r, time := range data.times {
		if (minTime > 0 && time < minTime) || (maxTime > 0 && time > maxTime) {
			continue
		}

		for _, con := range constraints {
			// sanity check - the number of columns has to match the number of constraints
			if len(con.Query) != len(data.counts) {
				return errors.New(
					fmt.Sprintf("in %s: length of constraints (%d) does not match number of data columns (%d)",
						dataPath, len(data.counts), len(con.Query)))
			}

			result := 0
			for c, q := range con.Query {
				result += (q * data.counts[c][r])
			}

			if result != con.Target {
				return errors.New(
					fmt.Sprintf("in %s: constraint mismatch: result (%d) - actual (%d)",
						dataPath, result, con.Target))
			}
		}
	}

	return nil
}

// checkCountMinmax tests that each column of the parsed data is larger
// equal than CountMinimum and smaller equal than CountMaximum.
func checkCountMinmax(data *Columns, dataPath string, minTime, maxTime float64,
	countMaximum, countMinimum []int) error {

	if countMaximum != nil && len(countMaximum) != len(data.counts) {
		return errors.New(fmt.Sprintf(
			"in %s: number of constraints in countMaximum does not match number of data columns",
			dataPath))
	}

	if countMinimum != nil && len(countMinimum) != len(data.counts) {
		return errors.New(fmt.Sprintf(
			"in %s: number of constraints in countMinimum does not match number of data columns",
			dataPath))
	}

	for r, time := range data.times {
		if (minTime > 0 && time < minTime) || (maxTime > 0 && time > maxTime) {
			continue
		}

		for i := 0; i < len(data.counts); i++ {
			c := data.counts[i][r]
			if countMaximum != nil && c > countMaximum[i] {
				return errors.New(
					fmt.Sprintf("in %s: maximum exceeded: data (%d) > max(%d)", dataPath,
						c, countMaximum[i]))
			}
			if countMinimum != nil && c < countMinimum[i] {
				return errors.New(
					fmt.Sprintf("in %s: minimum undershot: data (%d) < min(%d)", dataPath,
						c, countMinimum[i]))
			}
		}
	}

	return nil
}

// fileMatchPattern matches the provided matchPattern against the content of
// the datafile at filePath and checks that it matches numMatches times.
func fileMatchPattern(filePath string, matchPattern string,
	numExpectedMatches int) error {
	content, err := ioutil.ReadFile(filePath)
	if err != nil {
		return errors.New(
			fmt.Sprintf("failed to open file %s", filePath))
	}

	matcher := regexp.MustCompile(matchPattern)
	matches := matcher.FindAll(content, -1)
	if len(matches) != numExpectedMatches {
		return errors.New(
			fmt.Sprintf("failed pattern match: %s matched %d times instead of %d",
				matchPattern, len(matches), numExpectedMatches))
	}

	return nil
}

// compareCounts checks that the test data matches the provided column counts
// exactly
func compareCounts(data, refData *Columns, dataPath string, minTime,
	maxTime float64) error {

	if len(refData.times) != len(data.times) {
		return errors.New(fmt.Sprintf(
			"in %s: reference and actual data set have different number of rows",
			dataPath))
	}

	if len(refData.counts) != len(data.counts) {
		return errors.New(fmt.Sprintf(
			"in %s: reference and actual data set have different number of columns",
			dataPath))
	}

	numCols := len(data.counts)
	for r, time := range data.times {
		if (minTime > 0 && time < minTime) || (maxTime > 0 && time > maxTime) {
			continue
		}

		for c := 0; c < numCols; c++ {
			if data.counts[c][r] != refData.counts[c][r] {
				return errors.New(
					fmt.Sprintf("in %s: reference and actual data differ in row %d and col %d",
						dataPath, r, c))
			}
		}
	}
	return nil
}

// checkCountEqulibrium checks that the column means of the test data match the
// provided target mean values withih tne provided tolerances.
func checkCountEquilibrium(data *Columns, dataPath string, minTime, maxTime float64,
	means, tolerances []float64) error {

	if len(means) != len(data.counts) {
		return errors.New(fmt.Sprintf(
			"in %s: number of provided means does not match number of data columns",
			dataPath))
	}

	numCols := len(data.counts)
	averages := make([]float64, numCols)
	var numValues int
	for r, time := range data.times {
		if (minTime > 0 && time < minTime) || (maxTime > 0 && time > maxTime) {
			continue
		}

		numValues += 1
		for c := 0; c < numCols; c++ {
			averages[c] += float64(data.counts[c][r])
		}
	}

	// compare averages with target means
	for c := 0; c < numCols; c++ {
		average := averages[c] / float64(numValues)
		if (average < means[c]-tolerances[c]) || (average > means[c]+tolerances[c]) {
			return errors.New(
				fmt.Sprintf("in %s: average value %f of column %d outside of tolerance %f +/- %f",
					dataPath, average, c, means[c], tolerances[c]))
		}
	}
	return nil
}

// checkPosititveCounts tests that all counts of the data file are positive > 0
func checkPositiveCounts(data *Columns, dataPath string, minTime,
	maxTime float64) error {

	numCols := len(data.counts)
	for r, time := range data.times {
		if (minTime > 0 && time < minTime) || (maxTime > 0 && time > maxTime) {
			continue
		}

		for c := 0; c < numCols; c++ {
			if data.counts[c][r] <= 0 {
				return errors.New(
					fmt.Sprintf("in %s value %d in column %d in row %d is not positive (<= 0)",
						dataPath, data.counts[c][r], c, r))
			}
		}
	}

	return nil
}
