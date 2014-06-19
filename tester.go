// Copyright 2014 Markus Dittrich. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// test_engine contains the actual test functions analysing
// the output of the run MCell simulations
package main

import (
	"fmt"
	"github.com/haskelladdict/datastruct/set/intset"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// testRunner analyses the TestDescriptions coming from an MCell run on a
// test and analyses them as requested per the TestDescription.
func testRunner(test *TestDescription, result chan *testResult) {

	// tests which don't require loading of reaction data output
	nonDataParseTests := []string{"DIFF_FILE_CONTENT", "FILE_MATCH_PATTERN",
		"CHECK_TRIGGERS", "CHECK_EXPRESSIONS", "CHECK_LEGACY_VOL_OUTPUT",
		"CHECK_EMPTY_FILE", "CHECK_ASCII_VIZ_OUTPUT", "CHECK_CHECKPOINT",
		"CHECK_DREAMM_V3_MOLS_BIN", "CHECK_DREAMM_V3_MESH_BIN"}

	for _, c := range test.Checks {

		dataPaths, err := getDataPaths(test.Path, c.DataFile, test.Run.seed,
			test.Run.NumSeeds)
		if err != nil {
			result <- &testResult{test.Path, false, c.TestType, fmt.Sprint(err)}
			continue
		}

		// load the data for test types which need it
		var data []*Columns
		var stringData []*StringColumns
		// NOTE: only attempt to parse data for the test cases which need it
		if c.DataFile != "" && !containsString(nonDataParseTests, c.TestType) {
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
					testErr = fmt.Errorf("Expected exit code %d but got %d instead",
						c.ExitCode, testRun.exitCode)
				}
			}

		case "CHECK_NONEMPTY_FILES":
			if testErr = checkFilesEmpty(test.Path, test.Run.seed, c.FileNames,
				c.IDRange, c.FileSize, false); testErr != nil {
				break
			}

		case "CHECK_EMPTY_FILES":
			if testErr = checkFilesEmpty(test.Path, test.Run.seed, c.FileNames,
				c.IDRange, c.FileSize, true); testErr != nil {
				break
			}

		case "CHECK_CHECKPOINT":
			if testErr = checkCheckPoint(test.Path, c.BaseName, c.Delay, c.Margin); testErr != nil {
				break
			}

		case "CHECK_LEGACY_VOL_OUTPUT":
			for _, p := range dataPaths {
				if testErr = checkLegacyVolOutput(p, c.Xdim, c.Ydim, c.Zdim); testErr != nil {
					break
				}
			}

		case "CHECK_ASCII_VIZ_OUTPUT":
			for _, p := range dataPaths {
				if testErr = checkASCIIVizOutput(p, c.SurfaceStates, c.VolumeStates); testErr != nil {
					break
				}
			}

		case "CHECK_DREAMM_V3_MOLS_BIN":
			if testErr = checkDREAMMV3MolsBin(test.Path, c.VizPath, c.AllIters,
				c.SurfPosIters, c.SurfOrientIters, c.SurfStateIters, c.VolPosIters,
				c.VolOrientIters, c.VolStateIters, c.SurfEmpty, c.VolEmpty); testErr != nil {
				break
			}

		case "CHECK_DREAMM_V3_MESH_BIN":
			if testErr = checkDREAMMV3MeshBin(test.Path, c.VizPath, c.AllIters,
				c.PosIters, c.RegionIters, c.StateIters, c.MeshEmpty); testErr != nil {
				break
			}

		case "DIFF_FILE_CONTENT":
			for _, p := range dataPaths {
				if testErr = diffFileContent(test.Path, p, c.TemplateFile,
					c.TemplateParameters); testErr != nil {
					break
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

		case "CHECK_EXPRESSIONS":
			for _, dataPath := range dataPaths {
				if testErr = checkExpressions(dataPath); testErr != nil {
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
			testErr = fmt.Errorf("Unknown test type: %s", c.TestType)
			break
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
				return fmt.Errorf("in %s: length of constraints (%d) does not match number of data columns (%d)",
					dataPath, len(data.counts), len(con.Query))
			}

			result := 0
			for c, q := range con.Query {
				result += (q * data.counts[c][r])
			}

			if result != con.Target {
				return fmt.Errorf("in %s: constraint mismatch: result (%d) - actual (%d)",
					dataPath, result, con.Target)
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
		return fmt.Errorf(
			"in %s: number of constraints in countMaximum does not match number of data columns",
			dataPath)
	}

	if countMinimum != nil && len(countMinimum) != len(data.counts) {
		return fmt.Errorf(
			"in %s: number of constraints in countMinimum does not match number of data columns",
			dataPath)
	}

	for r, time := range data.times {
		if (minTime > 0 && time < minTime) || (maxTime > 0 && time > maxTime) {
			continue
		}

		for i := 0; i < len(data.counts); i++ {
			c := data.counts[i][r]
			if countMaximum != nil && c > countMaximum[i] {
				return fmt.Errorf("in %s: maximum exceeded: data (%d) > max(%d)", dataPath,
					c, countMaximum[i])
			}
			if countMinimum != nil && c < countMinimum[i] {
				return fmt.Errorf("in %s: minimum undershot: data (%d) < min(%d)", dataPath,
					c, countMinimum[i])
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
		return fmt.Errorf("failed to open file %s", filePath)
	}

	matcher := regexp.MustCompile(matchPattern)
	matches := matcher.FindAll(content, -1)
	if len(matches) != numExpectedMatches {
		return fmt.Errorf("failed pattern match: %s matched %d times instead of %d",
			matchPattern, len(matches), numExpectedMatches)
	}

	return nil
}

// compareCounts checks that the test data matches the provided column counts
// exactly
func compareCounts(data, refData *Columns, dataPath string, minTime,
	maxTime float64) error {

	if len(refData.times) != len(data.times) {
		return fmt.Errorf(
			"in %s: reference and actual data set have different number of rows",
			dataPath)
	}

	if len(refData.counts) != len(data.counts) {
		return fmt.Errorf(
			"in %s: reference and actual data set have different number of columns",
			dataPath)
	}

	numCols := len(data.counts)
	for r, time := range data.times {
		if (minTime > 0 && time < minTime) || (maxTime > 0 && time > maxTime) {
			continue
		}

		for c := 0; c < numCols; c++ {
			if data.counts[c][r] != refData.counts[c][r] {
				return fmt.Errorf("in %s: reference and actual data differ in row %d and col %d",
					dataPath, r, c)
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
		return fmt.Errorf(
			"in %s: number of provided means does not match number of data columns",
			dataPath)
	}

	numCols := len(data.counts)
	averageRate := make([]float64, numCols)
	var numValues int
	for r, time := range data.times {
		if (minTime > 0 && time < minTime) || (maxTime > 0 && time > maxTime) {
			continue
		}

		numValues++
		for c := 0; c < numCols; c++ {
			averageRate[c] += float64(data.counts[c][r]) / (time - baseTime)
		}
	}

	// compare with expected means
	for c := 0; c < numCols; c++ {
		rate := averageRate[c] / float64(numValues)
		if (rate < means[c]-tolerances[c]) || (rate > means[c]+tolerances[c]) {
			return fmt.Errorf(
				"in %s: average reaction rate %f is outside of tolerance %f +/- %f",
				dataPath, rate, means[c], tolerances[c])
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
		firstDataID++
		locationID++
	}
	totalCols := firstDataID

	typeID, err := getTriggerTypeID(triggerType, dataPath)
	if err != nil {
		return err
	}
	totalCols += typeID

	numCols := len(data.values)
	if numCols != totalCols {
		return fmt.Errorf(
			"in %s: incorrect column count of %d (expected %d)", dataPath, totalCols,
			numCols)
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
		return 0, fmt.Errorf("in %s: unknown trigger type %s", dataPath,
			triggerType)
	}
	return typeID, nil
}

// validateExactTime tests that the exact time present in a trigger output
// data falls within the given iteration
func validateExactTime(data *StringColumns, row int, time, outputTime float64,
	dataPath string) error {

	exactTime, err := strconv.ParseFloat(data.values[0][row], 64)
	if err != nil {
		return fmt.Errorf(
			"in %s: exact time value in row %d in not a float value", dataPath, row)
	}
	if (exactTime < time) || (exactTime > time+outputTime) {
		return fmt.Errorf(
			"in %s: exact time out of bounds in row %d", dataPath, row)
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
		return fmt.Errorf(
			"in %s: data value in row %d col %d is not an int", dataPath, row,
			firstDataID)
	}

	if typeID == 0 { // data is a reaction count - nothing to do
		return nil
	} else if typeID == 1 { // data is a hit count
		if value != -1 && value != 1 {
			return fmt.Errorf(
				"in %s: incorrect trigger data %d in row %d (expected -1, or 1)",
				dataPath, value, row)
		}
	} else if typeID == 2 { // data has to be orientation count
		if value != -1 && value != 0 && value != 1 {
			return fmt.Errorf(
				"in %s: incorrect trigger data %d in row %d (expected -1, 0, or 1)",
				dataPath, value, row)
		}
	} else {
		// unknown typeID
		return fmt.Errorf("Unknown trigger typeID of %d", typeID)
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
		return fmt.Errorf(
			"in %s: location data values in row %d are not of type float", dataPath,
			row)
	}

	if xrange != nil && (x < xrange[0] || x > xrange[1]) {
		return fmt.Errorf(
			"in %s: x coordinate %f out of bounds in row %d (expected [%f,%f])",
			dataPath, x, row, xrange[0], xrange[1])
	}

	if yrange != nil && (y < yrange[0] || y > yrange[1]) {
		return fmt.Errorf(
			"in %s: y coordinate %f out of bounds in row %d (expected [%f,%f])",
			dataPath, y, row, yrange[0], yrange[1])
	}

	if zrange != nil && (z < zrange[0] || z > zrange[1]) {
		return fmt.Errorf(
			"in %s: z coordinate %f out of bounds in row %d (expected [%f,%f])",
			dataPath, z, row, zrange[0], zrange[1])
	}
	return nil
}

// checkCountEqulibrium checks that the column means of the test data match the
// provided target mean values withih tne provided tolerances.
func checkCountEquilibrium(data *Columns, dataPath string, minTime, maxTime float64,
	means, tolerances []float64) error {

	if len(means) != len(data.counts) {
		return fmt.Errorf(
			"in %s: number of provided means does not match number of data columns",
			dataPath)
	}

	numCols := len(data.counts)
	averages := make([]float64, numCols)
	var numValues int
	for r, time := range data.times {
		if (minTime > 0 && time < minTime) || (maxTime > 0 && time > maxTime) {
			continue
		}

		numValues++
		for c := 0; c < numCols; c++ {
			averages[c] += float64(data.counts[c][r])
		}
	}

	// compare averages with target means
	for c := 0; c < numCols; c++ {
		average := averages[c] / float64(numValues)
		if (average < means[c]-tolerances[c]) || (average > means[c]+tolerances[c]) {
			return fmt.Errorf("in %s: average value %f of column %d outside of tolerance %f +/- %f",
				dataPath, average, c, means[c], tolerances[c])
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
				return fmt.Errorf("in %s value %d in column %d in row %d is not positive (<= 0)",
					dataPath, data.counts[c][r], c, r)
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
				return fmt.Errorf("in %s value %d in column %d in row %d is non-zero",
					dataPath, data.counts[c][r], c, r)
			}
		}
	}

	return nil
}

// checkFilesEmpty tests that all simulation output files listed were
// created by the run and are either emtpy or non-empty depending on the
// provided switch
func checkFilesEmpty(testDir string, seed int, fileNames []string,
	IDRange []string, fileSize int64, empty bool) error {

	var fileList []string
	for _, fileName := range fileNames {
		files, err := generateFileList(fileName, IDRange)
		if err != nil {
			return err
		}
		fileList = append(fileList, files...)
	}

	if len(fileList) == 0 {
		return fmt.Errorf("no files to test specified")
	}

	var sizeCheck func(int64) bool
	var message string
	if empty {
		sizeCheck = func(s int64) bool { return s == 0 }
		message = "non-empty"
	} else {
		sizeCheck = func(s int64) bool {
			if fileSize == 0 {
				return s != 0
			}
			return s == fileSize
		}
		message = "empty"
	}

	var badFileList []string
	for _, fileName := range fileList {
		filePaths, err := getDataPaths(testDir, fileName, seed, 1)
		if err != nil {
			return fmt.Errorf("failed to construct data path for file %s", fileName)
		}

		for _, filePath := range filePaths {
			fi, err := os.Stat(filePath)
			if err != nil || !sizeCheck(fi.Size()) {
				badFileList = append(badFileList, filePath)
			}
		}
	}

	if len(badFileList) != 0 {
		badFiles := strings.Join(badFileList, "\n\t\t")
		return fmt.Errorf("the following files were either missing, %s, or had "+
			"the wrong size:\n\n\t\t%s", message, badFiles)
	}
	return nil
}

// checkCheckPoint tests that a checkpoint happened at the requested delay
// in seconds (+/- margin)
func checkCheckPoint(testDir, baseName string, delay, margin float64) error {

	path := getOutputDir(testDir)

	stamp := filepath.Join(path, baseName+".stamp")
	stampi, err := os.Stat(stamp)
	if err != nil {
		return fmt.Errorf("Failed to stat file %s", stamp)
	}
	stampTime := stampi.ModTime()

	checkpt := filepath.Join(path, baseName+".cp")
	checkpti, err := os.Stat(checkpt)
	if err != nil {
		return fmt.Errorf("Failed to stat file %s", checkpt)
	}
	checkTime := checkpti.ModTime()

	if checkTime.Sub(stampTime).Seconds() < delay-margin {
		return fmt.Errorf("Realtime checkpoint scheduled for %f seconds but "+
			"time between timestamp and checkpoint is less than %f seconds",
			delay, delay-margin)
	}

	if checkTime.Sub(stampTime).Seconds() > delay+margin {
		return fmt.Errorf("Realtime checkpoint scheduled for %f seconds but "+
			"time between timestamp and checkpoint exceeds %f seconds", delay,
			delay+margin)
	}

	return nil
}

// checkExpressions tests the expressions for exact (==) or statistical
// equality (~=). We expect that the file to be tested contains only
// expressions of the form
// X == Y
// var ~= mean/std
func checkExpressions(filePath string) error {
	content, err := ioutil.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file %s", filePath)
	}

	lines := strings.Split(string(content), "\n")
	for _, line := range lines[:len(lines)-2] { // last element is empty line

		// exact expressions
		if strings.Count(line, "==") > 0 {
			vals := strings.Split(line[4:], "==")
			if len(vals) != 2 {
				return fmt.Errorf("malformed expression %s", line[4:])
			}
			val1, err1 := strconv.ParseFloat(strings.TrimSpace(vals[0]), 64)
			val2, err2 := strconv.ParseFloat(strings.TrimSpace(vals[1]), 64)
			if err1 != nil || err2 != nil {
				fmt.Println(val1, err1)
				return fmt.Errorf("cannot convert expression %s into floats", line[4:])
			}

			if val1 != val2 {
				return fmt.Errorf("target expression %f == %f does not evaluate correctly",
					val1, val2)
			}
		} else if strings.Count(line, "~=") > 0 {
			vals := strings.Split(line[4:], "~=")
			if len(vals) != 2 {
				return fmt.Errorf("malformed expression %s", line[4:])
			}
			vals1 := strings.Split(vals[1], "/")
			if len(vals1) != 2 {
				return fmt.Errorf("malformed expression %s", line[4:])
			}
			val, err1 := strconv.ParseFloat(strings.TrimSpace(vals[0]), 64)
			mean, err2 := strconv.ParseFloat(strings.TrimSpace(vals1[0]), 64)
			std, err3 := strconv.ParseFloat(strings.TrimSpace(vals1[1]), 64)
			if err1 != nil || err2 != nil || err3 != nil {
				return fmt.Errorf("cannot convert expression %s into floats", line[4:])
			}

			if (val < mean-2*std) || (val > mean+2*std) {
				return fmt.Errorf("Warning: Gaussian value %f out of 95%% confidence interval (%f +/- %f)",
					val, mean, std)
			}
		} else {
			return fmt.Errorf("unknown expression type %s", line)
		}
	}

	return nil
}

// diffFileContent matches the content of datafile with the one provided in
// the template file. The template file can contain format string parameters
// which will be filled with the template parameters as requested by the
// test file.
func diffFileContent(path, dataPath, templateFile string,
	templateParams []string) error {

	var tp []interface{}
	for _, t := range templateParams {
		switch t {
		case "TODAY_DAY":
			tp = append(tp, time.Now().Weekday().String())

		default:
			return fmt.Errorf("unknow template parameter %s", t)
		}
	}

	c, err := ioutil.ReadFile(dataPath)
	if err != nil {
		return fmt.Errorf("failed to open file %s", dataPath)
	}
	content := string(c)

	templatePath := filepath.Join(path, templateFile)
	tempContent, err := ioutil.ReadFile(templatePath)
	if err != nil {
		return fmt.Errorf("failed to open file %s", templatePath)
	}
	match := fmt.Sprintf(string(tempContent), tp...)

	if content != match {
		return fmt.Errorf("the test output does not match template.\n\nexpected\n"+
			"\n%s\n\nbut got\n\n%s\n", content, match)
	}
	return nil
}

// checkLegacyVolOutput checks some basic properties of legacy volume output
// files such as presence of a header and the number of data items
// NOTE: The header should look like
//       # nx=25 ny=25 nz=25 time=100
func checkLegacyVolOutput(dataPath string, xdim, ydim, zdim int) error {

	c, err := ioutil.ReadFile(dataPath)
	if err != nil {
		return fmt.Errorf("failed to open file %s", dataPath)
	}
	lines := strings.Split(string(c), "\n")

	// parse header
	if len(lines) == 0 {
		return fmt.Errorf("volume output file %s is empty", dataPath)
	}
	headerRegexp := regexp.MustCompile("# *nx=([0-9]+) *ny=([0-9]+) *nz=([0-9]+)")
	matches := headerRegexp.FindStringSubmatch(lines[0])
	if len(matches) != 4 {
		return fmt.Errorf("could not parse header of file %s", dataPath)
	}

	// check dimension
	if matches[1] != strconv.Itoa(xdim) || matches[2] != strconv.Itoa(ydim) ||
		matches[3] != strconv.Itoa(zdim) {
		return fmt.Errorf("volume output in %s had incorrect x, y, or z dimensions",
			dataPath)
	}

	expectedNumLines := (ydim+1)*zdim + 1 + 1
	if len(lines) != expectedNumLines {
		return fmt.Errorf("volume output in %s had incorrect number of lines "+
			"(%d instead of %d)",
			dataPath, len(lines), expectedNumLines)
	}

	return nil
}

// checkASCIIVizOutput tests some basic facts about legacy VIZ_DATA_OUTPUT
// blocks such as the presence of the proper viz states of surface and
// volume molecules.
// NOTE: This is a pretty lame test and basically a literal port of our
// previous unit test which was incidentally badly broken, so this one
// at least does something useful :)
func checkASCIIVizOutput(dataPath string, surfStates, volStates []int) error {

	c, err := ioutil.ReadFile(dataPath)
	if err != nil {
		return fmt.Errorf("failed to open file %s", dataPath)
	}
	lines := strings.Split(string(c), "\n")

	// loop over all lines
	// NOTE: the last line is an artifact due to Split and the final '\n'
	for i, l := range lines[:len(lines)-1] {
		items := strings.Fields(l)
		if len(items) != 8 {
			return fmt.Errorf("incorrect number of data items in %s row %d."+
				" Expected 8 found %d", dataPath, i, len(items))
		}

		// check if items (2,3,4) and (5,6,7) are vectors of floats
		for _, v := range []int{2, 3, 4} {
			if _, err := strconv.ParseFloat(items[v], 64); err != nil {
				return fmt.Errorf("in file %s: item in row %d and col %d is not a float",
					dataPath, i, v)
			}
		}

		sum := 0.0
		for _, v := range []int{5, 6, 7} {
			x, err := strconv.ParseFloat(items[v], 64)
			if err != nil {
				return fmt.Errorf("in file %s: item in row %d and col %d is not a float",
					dataPath, i, v)
			}
			sum += math.Abs(x)
		}

		// if sum is zero the vector (5,6,7) was identically zero and the
		// molecule thus is a volume molecule
		isVolMol := false
		if math.Nextafter(sum, 0.0) == sum {
			isVolMol = true
		}

		// check if the surface/volume states are in the list of expected ones
		state, err := strconv.Atoi(items[0])
		if err != nil {
			return fmt.Errorf("in file %s: the molecule state is not a integer", dataPath)
		}

		if isVolMol {
			for _, v := range volStates {
				if v == state {
					break
				}
				return fmt.Errorf("in file %s: encountered unknown volume molecule "+
					"state %d", dataPath, state)
			}
		} else {
			for _, v := range surfStates {
				if v == state {
					break
				}
				return fmt.Errorf("in file %s: encountered unknown surface molecule "+
					"state %d", dataPath, state)
			}
		}
	}

	return nil
}

// checkDREAMMV3MolsBin checks the layout for molecule related data within the
// DREAMM v3 viz format
func checkDREAMMV3MolsBin(testDir, dataDir string, allIters, surfPosIters,
	surfOrientIters, surfStateIters, volPosIters, volOrientIters,
	volStateIters intList, surfEmpty, volEmpty bool) error {

	m, err := createMolIters(allIters, surfPosIters, surfOrientIters,
		surfStateIters, volPosIters, volOrientIters, volStateIters)
	if err != nil {
		return err
	}

	dataPath := filepath.Join(getOutputDir(testDir), dataDir)
	lastSurfPos := -1
	lastSurfOrient := -1
	lastSurfState := -1
	lastVolPos := -1
	lastVolOrient := -1
	lastVolState := -1

	for _, i := range m.all {
		iterPath := filepath.Join(dataPath, "frame_data", "iteration_%d")
		hadFrame := false

		// surface positions
		surfPosFile := filepath.Join(iterPath, "surface_molecules_positions.bin")
		if err := checkDREAMMV3IterItems(m.surfPos, m.molIters, i, lastSurfPos,
			surfEmpty, surfPosFile); err != nil {
			return err
		}
		if m.surfPos.Contains(i) {
			lastSurfPos = i
			hadFrame = true
		}

		// surface orientations
		surfOrientFile := filepath.Join(iterPath, "surface_molecules_orientations.bin")
		if err := checkDREAMMV3IterItems(m.surfOrient, m.molIters, i, lastSurfOrient,
			surfEmpty, surfOrientFile); err != nil {
			return err
		}
		if m.surfOrient.Contains(i) {
			lastSurfOrient = i
			hadFrame = true
		}

		// surface states
		surfStateFile := filepath.Join(iterPath, "surface_molecules_states.bin")
		if err := checkDREAMMV3IterItems(m.surfState, m.molIters, i, lastSurfState,
			surfEmpty, surfStateFile); err != nil {
			return err
		}
		if m.surfState.Contains(i) {
			lastSurfState = i
			hadFrame = true
		}

		surfTemplate := filepath.Join(iterPath, "surface_molecules.dx")
		if err := checkDREAMMV3DXItems(i, lastSurfPos, lastSurfOrient, lastSurfState,
			hadFrame, surfTemplate); err != nil {
			return err
		}

		// volume positions
		hadFrame = false
		volPosFile := filepath.Join(iterPath, "volume_molecules_positions.bin")
		if err := checkDREAMMV3IterItems(m.volPos, m.molIters, i, lastVolPos,
			volEmpty, volPosFile); err != nil {
			return err
		}
		if m.volPos.Contains(i) {
			lastVolPos = i
			hadFrame = true
		}

		// volume orientations
		volOrientFile := filepath.Join(iterPath, "volume_molecules_orientations.bin")
		if err := checkDREAMMV3IterItems(m.volOrient, m.molIters, i, lastVolOrient,
			volEmpty, volOrientFile); err != nil {
			return err
		}
		if m.volOrient.Contains(i) {
			lastVolOrient = i
			hadFrame = true
		}

		// volume states
		volStateFile := filepath.Join(iterPath, "volume_molecules_states.bin")
		if err := checkDREAMMV3IterItems(m.volState, m.molIters, i, lastVolState,
			surfEmpty, volStateFile); err != nil {
			return err
		}
		if m.volState.Contains(i) {
			lastVolState = i
			hadFrame = true
		}

		volTemplate := filepath.Join(iterPath, "volume_molecules.dx")
		if err := checkDREAMMV3DXItems(i, lastVolPos, lastVolOrient, lastVolState,
			hadFrame, volTemplate); err != nil {
			return err
		}

	}
	return nil
}

// checkDREAMMV3MolsBin checks the layout for mesh related data within the
// DREAMM v3 viz format
func checkDREAMMV3MeshBin(testDir, dataDir string, allIters, posIters,
	regionIters, stateIters intList, meshEmpty bool) error {

	m, err := createMeshIters(allIters, posIters, regionIters, stateIters)
	if err != nil {
		return err
	}

	dataPath := filepath.Join(getOutputDir(testDir), dataDir)
	lastPos := -1
	lastRegion := -1
	lastState := -1

	for _, i := range m.all {
		iterPath := filepath.Join(dataPath, "frame_data", "iteration_%d")
		hadFrame := false

		// positions
		posFile := filepath.Join(iterPath, "mesh_positions.bin")
		if err := checkDREAMMV3IterItems(m.pos, m.combined, i, lastPos,
			meshEmpty, posFile); err != nil {
			return err
		}
		if m.pos.Contains(i) {
			lastPos = i
			unsetTrackers(i, &lastPos, &lastRegion, &lastState)
			hadFrame = true
		}

		// regions
		regionFile := filepath.Join(iterPath, "region_indices.bin")
		if err := checkDREAMMV3IterItems(m.regions, m.states, i, lastRegion,
			meshEmpty, regionFile); err != nil {
			return err
		}
		if m.regions.Contains(i) {
			lastRegion = i
			unsetTrackers(i, &lastPos, &lastRegion, &lastState)
			hadFrame = true
		}

		// states
		statesFile := filepath.Join(iterPath, "mesh_states.bin")
		emptySet := set.NewIntSet()
		if err := checkDREAMMV3IterItems(m.states, emptySet, i, lastState,
			meshEmpty, statesFile); err != nil {
			return err
		}
		if m.states.Contains(i) {
			lastState = i
			unsetTrackers(i, &lastPos, &lastRegion, &lastState)
			hadFrame = true
		}

		template := filepath.Join(iterPath, "meshes.dx")
		if err := checkDREAMMV3DXItems(i, lastPos, lastRegion, lastState,
			hadFrame, template); err != nil {
			return err
		}
	}
	return nil
}
