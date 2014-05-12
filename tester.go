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
	"strconv"
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

		// load the data
		var data []*Columns
		var stringData []*StringColumns
		// NOTE: only attempt to parse data for the relevant test cases
		if c.DataFile != "" &&
			c.TestType != "FILE_MATCH_PATTERN" &&
			c.TestType != "CHECK_TRIGGERS" {
			data, err = loadData(dataPaths, c.HaveHeader, c.AverageData)
			if err != nil {
				result <- &testResult{test.Path, false, c.TestType, fmt.Sprint(err)}
				continue
			}
		} else if c.TestType == "CHECK_TRIGGERS" {
			stringData, err = loadStringData(dataPaths, c.HaveHeader)
			if err != nil {
				result <- &testResult{test.Path, false, c.TestType, fmt.Sprint(err)}
				continue
			}
		}

		// execute requested tests on data
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
				if testErr = checkPositiveOrZeroCounts(d, dataPaths[i], c.MinTime,
					c.MaxTime, false); testErr != nil {
					break
				}
			}

		case "POSITIVE_OR_ZERO_COUNTS":
			for i, d := range data {
				if testErr = checkPositiveOrZeroCounts(d, dataPaths[i], c.MinTime,
					c.MaxTime, true); testErr != nil {
					break
				}
			}

		case "ZERO_COUNTS":
			for i, d := range data {
				if testErr = checkZeroCounts(d, dataPaths[i], c.MinTime,
					c.MaxTime); testErr != nil {
					break
				}
			}

		case "COUNT_RATES":
			for i, d := range data {
				if testErr = countRates(d, dataPaths[i], c.MinTime, c.MaxTime,
					c.BaseTime, c.Means, c.Tolerances); testErr != nil {
					break
				}
			}

		case "CHECK_TRIGGERS":
			for i, d := range stringData {
				if testErr = checkTriggers(d, dataPaths[i], c.MinTime, c.MaxTime,
					c.TriggerType, c.HaveExactTime, c.OutputTime, c.Xrange, c.Yrange,
					c.Zrange); testErr != nil {
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

// countRates checks that the average reaction rates match the provided means
// and tolerances. The rates are computed as
//
// rate = instantaneous_count/(time_now - baseTime)
//
// and then averages accross the interval maxTime - minTime
func countRates(data *Columns, dataPath string, minTime, maxTime, baseTime float64,
	means, tolerances []float64) error {

	if len(means) != len(data.counts) {
		return errors.New(fmt.Sprintf(
			"in %s: number of provided means does not match number of data columns",
			dataPath))
	}

	numCols := len(data.counts)
	averageRate := make([]float64, numCols)
	var numValues int
	for r, time := range data.times {
		if (minTime > 0 && time < minTime) || (maxTime > 0 && time > maxTime) {
			continue
		}

		numValues += 1
		for c := 0; c < numCols; c++ {
			averageRate[c] += float64(data.counts[c][r]) / (time - baseTime)
		}
	}

	// compare with expected means
	for c := 0; c < numCols; c++ {
		rate := averageRate[c] / float64(numValues)
		if (rate < means[c]-tolerances[c]) || (rate > means[c]+tolerances[c]) {
			return errors.New(fmt.Sprintf(
				"in %s: average reaction rate %f is outside of tolerance %f +/- %f",
				dataPath, rate, means[c], tolerances[c]))
		}
	}

	return nil
}

// checkTriggers checks trigger data output. Since a trigger data file typically
// contains a mix of integer, float, and string values the data is passed in
// as a StringColumns struct. This routine tests the following trigger data
// properties
//    - the exact time of a trigger event reported is within the
//      proper iteration interval
//    - ensure that the data column have the expected data values
//      (-1, 0, 1) for orientation data, (-1, 1) for hit data
//    - the location of the trigger events is within the specified range for
//      x, y, and z
func checkTriggers(data *StringColumns, dataPath string, minTime, maxTime float64,
	triggerType string, haveExactTime bool, outputTime float64,
	xrange, yrange, zrange []float64) error {

	// compute column offsets
	firstDataID := 3
	locationID := 0
	if haveExactTime {
		firstDataID += 1
		locationID += 1
	}
	totalCols := firstDataID

	typeID, err := getTriggerTypeID(triggerType, dataPath)
	if err != nil {
		return err
	}
	totalCols += typeID

	numCols := len(data.values)
	if numCols != totalCols {
		return errors.New(fmt.Sprintf(
			"in %s: incorrect column count of %d (expected %d)", dataPath, totalCols,
			numCols))
	}

	for r, time := range data.times {
		if (minTime > 0 && time < minTime) || (maxTime > 0 && time > maxTime) {
			continue
		}

		// validate exact time
		if haveExactTime {
			err := validateExactTime(data, r, time, outputTime, dataPath)
			if err != nil {
				return err
			}
		}

		// validate data columns
		err := validateTriggerData(data, r, firstDataID, typeID, dataPath)
		if err != nil {
			return err
		}

		// validate x, y and z locations
		err = validatePositionRanges(data, r, locationID, xrange, yrange, zrange, dataPath)
		if err != nil {
			return err
		}
	}

	return nil
}

// getTriggerTypeID is a helper function converting the string triggerType
// into a integer typeID
func getTriggerTypeID(triggerType, dataPath string) (int, error) {

	var typeID int
	switch triggerType {
	case "reaction":
		typeID = 0
	case "hits":
		typeID = 1
	case "molCounts":
		typeID = 2
	default:
		return 0, errors.New(fmt.Sprintf("in %s: unknown trigger type %s", dataPath,
			triggerType))
	}
	return typeID, nil
}

// validateExactTime tests that the exact time present in a trigger output
// data falls within the given iteration
func validateExactTime(data *StringColumns, row int, time, outputTime float64,
	dataPath string) error {

	exactTime, err := strconv.ParseFloat(data.values[0][row], 64)
	if err != nil {
		return errors.New(fmt.Sprintf(
			"in %s: exact time value in row %d in not a float value", dataPath, row))
	}
	if (exactTime < time) || (exactTime > time+outputTime) {
		return errors.New(fmt.Sprintf(
			"in %s: exact time out of bounds in row %d", dataPath, row))
	}

	return nil
}

// validateTriggerData tests that the trigger data is of the correct type.
// For orientation data we expect -1, 0, or 1.
// For hit data we expect -1 or 1
func validateTriggerData(data *StringColumns, row, firstDataID, typeID int,
	dataPath string) error {

	value, err := strconv.Atoi(data.values[firstDataID][row])
	if err != nil {
		return errors.New(fmt.Sprintf(
			"in %s: data value in row %d col %d is not an int", dataPath, row,
			firstDataID))
	}

	// data has to be orientation or hit count
	if typeID > 0 {
		if value != -1 && value != 0 && value != 1 {
			return errors.New(fmt.Sprintf(
				"in %s: incorrect trigger data %d in row %d (expected -1, 0, or 1)",
				dataPath, value, row))
		}
	}

	// data has to be a hit count
	if typeID > 1 {
		if value != -1 && value != 1 {
			return errors.New(fmt.Sprintf(
				"in %s: incorrect trigger data %d in row %d (expected -1, or 1)",
				dataPath, value, row))
		}
	}
	return nil
}

// validatePositionRanges tests that trigger events happen within the specified
// ranges for x, y, and z coordinates
func validatePositionRanges(data *StringColumns, row, locationID int,
	xrange, yrange, zrange []float64, dataPath string) error {

	strconv.ParseFloat(data.values[0][row], 64)
	x, errx := strconv.ParseFloat(data.values[locationID][row], 64)
	y, erry := strconv.ParseFloat(data.values[locationID+1][row], 64)
	z, errz := strconv.ParseFloat(data.values[locationID+2][row], 64)

	if errx != nil || erry != nil || errz != nil {
		return errors.New(fmt.Sprintf(
			"in %s: location data values in row %d are not of type float", dataPath,
			row))
	}

	if xrange != nil && (x < xrange[0] || x > xrange[1]) {
		return errors.New(fmt.Sprintf(
			"in %s: x coordinate %f out of bounds in row %d (expected [%f,%f])",
			dataPath, x, row, xrange[0], xrange[1]))
	}

	if yrange != nil && (y < yrange[0] || y > yrange[1]) {
		return errors.New(fmt.Sprintf(
			"in %s: y coordinate %f out of bounds in row %d (expected [%f,%f])",
			dataPath, y, row, yrange[0], yrange[1]))
	}

	if zrange != nil && (z < zrange[0] || z > zrange[1]) {
		return errors.New(fmt.Sprintf(
			"in %s: z coordinate %f out of bounds in row %d (expected [%f,%f])",
			dataPath, z, row, zrange[0], zrange[1]))
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

// checkPositiveOrZeroCounts tests that all counts of the data file are either > 0
// (includeZero = false) or >= 0 (includeZero = true)
func checkPositiveOrZeroCounts(data *Columns, dataPath string, minTime,
	maxTime float64, includeZero bool) error {

	lowerBound := 1
	if includeZero {
		lowerBound = 0
	}

	numCols := len(data.counts)
	for r, time := range data.times {
		if (minTime > 0 && time < minTime) || (maxTime > 0 && time > maxTime) {
			continue
		}

		for c := 0; c < numCols; c++ {
			if data.counts[c][r] < lowerBound {
				return errors.New(
					fmt.Sprintf("in %s value %d in column %d in row %d is not positive (<= 0)",
						dataPath, data.counts[c][r], c, r))
			}
		}
	}

	return nil
}

// checkZeroCounts tests that all data counts are zero
func checkZeroCounts(data *Columns, dataPath string, minTime,
	maxTime float64) error {

	numCols := len(data.counts)
	for r, time := range data.times {
		if (minTime > 0 && time < minTime) || (maxTime > 0 && time > maxTime) {
			continue
		}

		for c := 0; c < numCols; c++ {
			if data.counts[c][r] != 0 {
				return errors.New(
					fmt.Sprintf("in %s value %d in column %d in row %d is non-zero",
						dataPath, data.counts[c][r], c, r))
			}
		}
	}

	return nil
}
