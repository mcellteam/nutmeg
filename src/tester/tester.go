// Copyright 2014 Markus Dittrich. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// test_engine contains the actual test functions analysing
// the output of the run MCell simulations
package tester

import (
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/haskelladdict/datastruct/set/intset"
	"github.com/haskelladdict/nutmeg/src/file"
	"github.com/haskelladdict/nutmeg/src/jsonParser"
	"github.com/haskelladdict/nutmeg/src/misc"
)

// TestResults encapsulates the results of an individual test
type TestResult struct {
	Path         string // path to test which was run
	Success      bool   // was test successful
	TestName     string // name of test
	ErrorMessage string // error message if test failed
}

// Run analyses the TestDescriptions coming from an MCell run on a
// test and analyses them as requested per the TestDescription.
func Run(test *jsonParser.TestDescription, result chan *TestResult) {

	// tests which don't require loading of reaction data output
	nonDataParseTests := []string{"DIFF_FILE_CONTENT", "FILE_MATCH_PATTERN",
		"CHECK_TRIGGERS", "CHECK_EXPRESSIONS", "CHECK_LEGACY_VOL_OUTPUT",
		"CHECK_EMPTY_FILE", "CHECK_ASCII_VIZ_OUTPUT", "CHECK_CHECKPOINT",
		"CHECK_DREAMM_V3_MOLS_BIN", "CHECK_DREAMM_V3_MESH_BIN",
		"CHECK_DREAMM_V3_MESH_ASCII", "CHECK_DREAMM_V3_MOLS_ASCII",
		"CHECK_DREAMM_V3_GROUPED"}

	for _, c := range test.Checks {

		dataPaths, err := file.GetDataPaths(test.Path, c.DataFile, test.Run.Seed,
			test.Run.NumSeeds)
		if err != nil {
			result <- &TestResult{test.Path, false, c.TestType, fmt.Sprint(err)}
			continue
		}

		// load the data for test types which need it
		var data []*file.Columns
		var stringData []*file.StringColumns
		// NOTE: only attempt to parse data for the test cases which need it
		if c.DataFile != "" && !misc.ContainsString(nonDataParseTests, c.TestType) {
			data, err = file.LoadData(dataPaths, c.HaveHeader, c.AverageData)
			if err != nil {
				result <- &TestResult{test.Path, false, c.TestType, fmt.Sprint(err)}
				continue
			}
		} else if c.TestType == "CHECK_TRIGGERS" {
			stringData, err = file.LoadStringData(dataPaths, c.HaveHeader)
			if err != nil {
				result <- &TestResult{test.Path, false, c.TestType, fmt.Sprint(err)}
				continue
			}
		}

		// execute requested tests on data
		var testErr error
		switch c.TestType {
		case "CHECK_SUCCESS":
			if test.SimStatus == nil {
				result <- &TestResult{test.Path, false, "CHECK_SUCCESS",
					"simulations did not run or return an exit status"}
				return // if simulation fails we won't continue testing
			}

			// in order to cut down on the amount of output (particularly in the case of
			// multiple seeds) we return failure if one or more of all runs within a test
			// fails and success otherwise
			for _, testRun := range test.SimStatus {
				if !testRun.Success {
					message := strings.Join([]string{testRun.ExitMessage, testRun.StdErrContent}, "\n")
					result <- &TestResult{test.Path, false, "CHECK_SUCCESS", message}
					return // if simulation fails we won't continue testing
				}
			}

		case "CHECK_EXIT_CODE":
			for _, testRun := range test.SimStatus {
				if c.ExitCode != testRun.ExitCode {
					testErr = fmt.Errorf("Expected exit code %d but got %d instead",
						c.ExitCode, testRun.ExitCode)
				}
			}

		case "CHECK_NONEMPTY_FILES":
			if testErr = checkFilesEmpty(test, c, false); testErr != nil {
				break
			}

		case "CHECK_EMPTY_FILES":
			if testErr = checkFilesEmpty(test, c, true); testErr != nil {
				break
			}

		case "CHECK_CHECKPOINT":
			if testErr = checkCheckPoint(test.Path, c); testErr != nil {
				break
			}

		case "CHECK_LEGACY_VOL_OUTPUT":
			for _, p := range dataPaths {
				if testErr = checkLegacyVolOutput(p, c); testErr != nil {
					break
				}
			}

		case "CHECK_ASCII_VIZ_OUTPUT":
			for _, p := range dataPaths {
				if testErr = checkASCIIVizOutput(p); testErr != nil {
					break
				}
			}

		case "CHECK_DREAMM_V3_MOLS_BIN":
			if testErr = checkDREAMMV3MolsBin(test.Path, c); testErr != nil {
				break
			}

		case "CHECK_DREAMM_V3_MOLS_ASCII":
			if testErr = checkDREAMMV3MolsASCII(test.Path, c); testErr != nil {
				break
			}

		case "CHECK_DREAMM_V3_MESH_BIN":
			if testErr = checkDREAMMV3MeshBin(test.Path, c.VizPath, c.AllIters,
				c.PosIters, c.RegionIters, c.StateIters, c.MeshEmpty); testErr != nil {
				break
			}

		case "CHECK_DREAMM_V3_MESH_ASCII":
			if testErr = checkDREAMMV3MeshASCII(test.Path, c.VizPath, c.AllIters,
				c.PosIters, c.RegionIters, c.StateIters, c.MeshEmpty, c.Objects,
				c.ObjectRegions); testErr != nil {
				break
			}

		case "CHECK_DREAMM_V3_GROUPED":
			if testErr = checkDREAMMV3Grouped(test.Path, c.VizPath, c.NumIters,
				c.NumTimes, c.HaveMeshPos, c.HaveRgnIdx, c.HaveMeshState, c.NoMeshes,
				c.HaveMolPos, c.HaveMolOrient, c.HaveMolState, c.NoMols); testErr != nil {
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
			refData, err := file.ReadCounts(referencePath, c.HaveHeader)
			if err != nil {
				testErr = err
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
// success/failure in TestResult object and sends it to the results channel
func recordResult(result chan<- *TestResult, testType string,
	dataPath string, err error) {
	if err != nil {
		result <- &TestResult{dataPath, false, testType, fmt.Sprint(err)}
	} else {
		result <- &TestResult{dataPath, true, testType, ""}
	}
}

// checkCountConstraints tests the provided array of constraints
// on the simulation output data contained in the file filePath
func checkCountConstraints(data *file.Columns, dataPath string, minTime,
	maxTime float64, constraints []*jsonParser.ConstraintSpec) error {

	// check constraints for each row of data
	for r, time := range data.Times {
		if (minTime > 0 && time < minTime) || (maxTime > 0 && time > maxTime) {
			continue
		}

		for _, con := range constraints {
			// sanity check - the number of columns has to match the number of constraints
			if len(con.Query) != len(data.Counts) {
				return fmt.Errorf("in %s: length of constraints (%d) does not match number of data columns (%d)",
					dataPath, len(data.Counts), len(con.Query))
			}

			result := 0
			for c, q := range con.Query {
				result += (q * data.Counts[c][r])
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
func checkCountMinmax(data *file.Columns, dataPath string, minTime, maxTime float64,
	countMaximum, countMinimum []int) error {

	if countMaximum != nil && len(countMaximum) != len(data.Counts) {
		return fmt.Errorf(
			"in %s: number of constraints in countMaximum does not match number of data columns",
			dataPath)
	}

	if countMinimum != nil && len(countMinimum) != len(data.Counts) {
		return fmt.Errorf(
			"in %s: number of constraints in countMinimum does not match number of data columns",
			dataPath)
	}

	for r, time := range data.Times {
		if (minTime > 0 && time < minTime) || (maxTime > 0 && time > maxTime) {
			continue
		}

		for i := 0; i < len(data.Counts); i++ {
			c := data.Counts[i][r]
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
	// need to normalize since Windows has different EOL character
	normalizedContent := strings.Replace(string(content), "\r\n", "\n", -1)

	matcher := regexp.MustCompile(matchPattern)
	matches := matcher.FindAllString(normalizedContent, -1)
	if len(matches) != numExpectedMatches {
		return fmt.Errorf("failed pattern match: %s matched %d times instead of %d",
			matchPattern, len(matches), numExpectedMatches)
	}

	return nil
}

// compareCounts checks that the test data matches the provided column counts
// exactly
func compareCounts(data, refData *file.Columns, dataPath string, minTime,
	maxTime float64) error {

	if len(refData.Times) != len(data.Times) {
		return fmt.Errorf(
			"in %s: reference and actual data set have different number of rows",
			dataPath)
	}

	if len(refData.Counts) != len(data.Counts) {
		return fmt.Errorf(
			"in %s: reference and actual data set have different number of columns",
			dataPath)
	}

	numCols := len(data.Counts)
	for r, time := range data.Times {
		if (minTime > 0 && time < minTime) || (maxTime > 0 && time > maxTime) {
			continue
		}

		for c := 0; c < numCols; c++ {
			if data.Counts[c][r] != refData.Counts[c][r] {
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
func countRates(data *file.Columns, dataPath string, minTime, maxTime, baseTime float64,
	means, tolerances []float64) error {

	if len(means) != len(data.Counts) {
		return fmt.Errorf(
			"in %s: number of provided means does not match number of data columns",
			dataPath)
	}

	numCols := len(data.Counts)
	averageRate := make([]float64, numCols)
	var numValues int
	for r, time := range data.Times {
		if (minTime > 0 && time < minTime) || (maxTime > 0 && time > maxTime) {
			continue
		}

		numValues++
		for c := 0; c < numCols; c++ {
			averageRate[c] += float64(data.Counts[c][r]) / (time - baseTime)
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
func checkTriggers(data *file.StringColumns, dataPath string, minTime, maxTime float64,
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

	numCols := len(data.Values)
	if numCols != totalCols {
		return fmt.Errorf(
			"in %s: incorrect column count of %d (expected %d)", dataPath, totalCols,
			numCols)
	}

	for r, time := range data.Times {
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
func validateExactTime(data *file.StringColumns, row int, time, outputTime float64,
	dataPath string) error {

	exactTime, err := strconv.ParseFloat(data.Values[0][row], 64)
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
func validateTriggerData(data *file.StringColumns, row, firstDataID, typeID int,
	dataPath string) error {

	value, err := strconv.Atoi(data.Values[firstDataID][row])
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
func validatePositionRanges(data *file.StringColumns, row, locationID int,
	xrange, yrange, zrange []float64, dataPath string) error {

	strconv.ParseFloat(data.Values[0][row], 64)
	x, errx := strconv.ParseFloat(data.Values[locationID][row], 64)
	y, erry := strconv.ParseFloat(data.Values[locationID+1][row], 64)
	z, errz := strconv.ParseFloat(data.Values[locationID+2][row], 64)

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
func checkCountEquilibrium(data *file.Columns, dataPath string, minTime, maxTime float64,
	means, tolerances []float64) error {

	if len(means) != len(data.Counts) {
		return fmt.Errorf(
			"in %s: number of provided means does not match number of data columns",
			dataPath)
	}

	numCols := len(data.Counts)
	averages := make([]float64, numCols)
	var numValues int
	for r, time := range data.Times {
		if (minTime > 0 && time < minTime) || (maxTime > 0 && time > maxTime) {
			continue
		}

		numValues++
		for c := 0; c < numCols; c++ {
			averages[c] += float64(data.Counts[c][r])
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
func checkPositiveOrZeroCounts(data *file.Columns, dataPath string, minTime,
	maxTime float64, includeZero bool) error {

	lowerBound := 1
	if includeZero {
		lowerBound = 0
	}

	numCols := len(data.Counts)
	for r, time := range data.Times {
		if (minTime > 0 && time < minTime) || (maxTime > 0 && time > maxTime) {
			continue
		}

		for c := 0; c < numCols; c++ {
			if data.Counts[c][r] < lowerBound {
				return fmt.Errorf("in %s value %d in column %d in row %d is not positive (<= 0)",
					dataPath, data.Counts[c][r], c, r)
			}
		}
	}

	return nil
}

// checkZeroCounts tests that all data counts are zero
func checkZeroCounts(data *file.Columns, dataPath string, minTime,
	maxTime float64) error {

	numCols := len(data.Counts)
	for r, time := range data.Times {
		if (minTime > 0 && time < minTime) || (maxTime > 0 && time > maxTime) {
			continue
		}

		for c := 0; c < numCols; c++ {
			if data.Counts[c][r] != 0 {
				return fmt.Errorf("in %s value %d in column %d in row %d is non-zero",
					dataPath, data.Counts[c][r], c, r)
			}
		}
	}

	return nil
}

// checkFilesEmpty tests that all simulation output files listed were
// created by the run and are either emtpy or non-empty depending on the
// provided switch
func checkFilesEmpty(test *jsonParser.TestDescription, c *jsonParser.TestCase,
	empty bool) error {

	var fileList []string
	for _, fileName := range c.FileNames {
		files, err := misc.GenerateFileList(fileName, c.IDRange)
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
			if c.FileSize == 0 {
				return s != 0
			}
			return s == c.FileSize
		}
		message = "empty"
	}

	var badFileList []string
	for _, fileName := range fileList {
		filePaths, err := file.GetDataPaths(test.Path, fileName, test.Run.Seed, 1)
		if err != nil {
			return fmt.Errorf("failed to construct data path for file %s:\n%s",
				fileName, err)
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
func checkCheckPoint(testDir string, c *jsonParser.TestCase) error {
	path := file.GetOutputDir(testDir)

	stamp := filepath.Join(path, c.BaseName+".stamp")
	stampi, err := os.Stat(stamp)
	if err != nil {
		return fmt.Errorf("Failed to stat file %s", stamp)
	}
	stampTime := stampi.ModTime()

	checkpt := filepath.Join(path, c.BaseName+".cp")
	checkpti, err := os.Stat(checkpt)
	if err != nil {
		return fmt.Errorf("Failed to stat file %s", checkpt)
	}
	checkTime := checkpti.ModTime()

	if checkTime.Sub(stampTime).Seconds() < c.Delay-c.Margin {
		return fmt.Errorf("Realtime checkpoint scheduled for %f seconds but "+
			"time between timestamp and checkpoint is less than %f seconds",
			c.Delay, c.Delay-c.Margin)
	}

	if checkTime.Sub(stampTime).Seconds() > c.Delay+c.Margin {
		return fmt.Errorf("Realtime checkpoint scheduled for %f seconds but "+
			"time between timestamp and checkpoint exceeds %f seconds", c.Delay,
			c.Delay+c.Margin)
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
	// need to normalize since Windows has different EOL character
	normalizedContent := strings.Replace(content, "\r\n", "\n", -1)

	templatePath := filepath.Join(path, templateFile)
	tempContent, err := ioutil.ReadFile(templatePath)
	if err != nil {
		return fmt.Errorf("failed to open file %s", templatePath)
	}
	match := fmt.Sprintf(string(tempContent), tp...)

	if normalizedContent != match {
		return fmt.Errorf("the test output does not match template.\n\nexpected\n"+
			"\n%s\n\nbut got\n\n%s\n", content, match)
	}
	return nil
}

// checkLegacyVolOutput checks some basic properties of legacy volume output
// files such as presence of a header and the number of data items
// NOTE: The header should look like
//       # nx=25 ny=25 nz=25 time=100
func checkLegacyVolOutput(dataPath string, c *jsonParser.TestCase) error {

	file, err := ioutil.ReadFile(dataPath)
	if err != nil {
		return fmt.Errorf("failed to open file %s", dataPath)
	}
	lines := strings.Split(string(file), "\n")

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
	if matches[1] != strconv.Itoa(c.Xdim) || matches[2] != strconv.Itoa(c.Ydim) ||
		matches[3] != strconv.Itoa(c.Zdim) {
		return fmt.Errorf("volume output in %s had incorrect x, y, or z dimensions",
			dataPath)
	}

	expectedNumLines := (c.Ydim+1)*c.Zdim + 1 + 1
	if len(lines) != expectedNumLines {
		return fmt.Errorf("volume output in %s had incorrect number of lines "+
			"(%d instead of %d)",
			dataPath, len(lines), expectedNumLines)
	}

	return nil
}

// checkASCIIVizOutput tests some basic facts about the ASCII output
// mode within the VIZ_OUTPUT data format.
// NOTE: This is a pretty lame test and basically a literal port of our
// previous unit test which was incidentally badly broken, so this one
// at least does something useful :)
func checkASCIIVizOutput(dataPath string) error {

	file, err := ioutil.ReadFile(dataPath)
	if err != nil {
		return fmt.Errorf("failed to open file %s", dataPath)
	}
	lines := strings.Split(string(file), "\n")

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
	}

	return nil
}

// checkDREAMMV3MolsBin checks the layout for molecule viz data as part of the
// binary DREAMM v3 format
func checkDREAMMV3MolsBin(testDir string, c *jsonParser.TestCase) error {

	s, err := misc.CreateMolMeshIters(c.AllIters, c.SurfPosIters, c.SurfOrientIters,
		c.SurfStateIters)
	if err != nil {
		return err
	}

	v, err := misc.CreateMolMeshIters(c.AllIters, c.VolPosIters, c.VolOrientIters,
		c.VolStateIters)
	if err != nil {
		return err
	}
	molIters := s.AllCombined.Clone().Union(v.AllCombined)

	dataPath := filepath.Join(file.GetOutputDir(testDir), c.VizPath)
	lastSurfPos := -1
	lastSurfOrient := -1
	lastSurfState := -1
	lastVolPos := -1
	lastVolOrient := -1
	lastVolState := -1

	for _, i := range s.All {
		iterPath := filepath.Join(dataPath, "frame_data", "iteration_%d")
		hadFrame := false

		// surface positions
		surfPosFile := filepath.Join(iterPath, "surface_molecules_positions.bin")
		if err := misc.CheckDREAMMV3IterItems(s.Pos, molIters, i, lastSurfPos,
			c.SurfEmpty, surfPosFile); err != nil {
			return err
		}
		if s.Pos.Contains(i) {
			lastSurfPos = i
			hadFrame = true
		}

		// surface orientations
		surfOrientFile := filepath.Join(iterPath, "surface_molecules_orientations.bin")
		if err := misc.CheckDREAMMV3IterItems(s.Others, molIters, i, lastSurfOrient,
			c.SurfEmpty, surfOrientFile); err != nil {
			return err
		}
		if s.Others.Contains(i) {
			lastSurfOrient = i
			hadFrame = true
		}

		// surface states
		surfStateFile := filepath.Join(iterPath, "surface_molecules_states.bin")
		if err := misc.CheckDREAMMV3IterItems(s.States, molIters, i, lastSurfState,
			c.SurfEmpty, surfStateFile); err != nil {
			return err
		}
		if s.States.Contains(i) {
			lastSurfState = i
			hadFrame = true
		}

		surfTemplate := filepath.Join(iterPath, "surface_molecules.dx")
		if err := misc.CheckDREAMMV3DXItems(i, lastSurfPos, lastSurfOrient, lastSurfState,
			hadFrame, surfTemplate); err != nil {
			return err
		}

		// volume positions
		hadFrame = false
		volPosFile := filepath.Join(iterPath, "volume_molecules_positions.bin")
		if err := misc.CheckDREAMMV3IterItems(v.Pos, molIters, i, lastVolPos,
			c.VolEmpty, volPosFile); err != nil {
			return err
		}
		if v.Pos.Contains(i) {
			lastVolPos = i
			hadFrame = true
		}

		// volume orientations
		volOrientFile := filepath.Join(iterPath, "volume_molecules_orientations.bin")
		if err := misc.CheckDREAMMV3IterItems(v.Others, molIters, i, lastVolOrient,
			c.VolEmpty, volOrientFile); err != nil {
			return err
		}
		if v.Others.Contains(i) {
			lastVolOrient = i
			hadFrame = true
		}

		// volume states
		volStateFile := filepath.Join(iterPath, "volume_molecules_states.bin")
		if err := misc.CheckDREAMMV3IterItems(v.States, molIters, i, lastVolState,
			c.SurfEmpty, volStateFile); err != nil {
			return err
		}
		if v.States.Contains(i) {
			lastVolState = i
			hadFrame = true
		}

		volTemplate := filepath.Join(iterPath, "volume_molecules.dx")
		if err := misc.CheckDREAMMV3DXItems(i, lastVolPos, lastVolOrient, lastVolState,
			hadFrame, volTemplate); err != nil {
			return err
		}

	}
	return nil
}

// checkDREAMMV3MolsASCII checks the layout for molecule viz data as part of the
// ASCII DREAMM v3 format
func checkDREAMMV3MolsASCII(testDir string, c *jsonParser.TestCase) error {

	m, err := misc.CreateMolMeshIters(c.AllIters, c.PosIters, c.OrientIters, c.StateIters)
	if err != nil {
		return err
	}

	dataPath := filepath.Join(file.GetOutputDir(testDir), c.VizPath)
	lastPos := -1
	lastOrient := -1
	lastState := -1

	for _, i := range m.All {
		iterPath := filepath.Join(dataPath, "frame_data", "iteration_%d")
		hadFrame := false

		// positions
		for _, obj := range c.MolNames {
			posFile := filepath.Join(iterPath, obj+".positions.dat")
			if err := misc.CheckDREAMMV3IterItems(m.Pos, m.Combined, i, lastPos,
				true, posFile); err != nil {
				return err
			}
		}
		if m.Pos.Contains(i) {
			lastPos = i
			misc.UnsetTrackers(i, &lastPos, &lastOrient, &lastState)
			hadFrame = true
		}

		// orientations
		for _, obj := range c.MolNames {
			orientFile := filepath.Join(iterPath, obj+".orientations.dat")
			if err := misc.CheckDREAMMV3IterItems(m.Others, m.Combined, i, lastOrient,
				true, orientFile); err != nil {
				return err
			}
		}
		if m.Others.Contains(i) {
			lastOrient = i
			misc.UnsetTrackers(i, &lastPos, &lastOrient, &lastState)
			hadFrame = true
		}

		// states
		for _, obj := range c.MolNames {
			stateFile := filepath.Join(iterPath, obj+".states.dat")
			if err := misc.CheckDREAMMV3IterItems(m.States, m.Combined, i, lastState,
				true, stateFile); err != nil {
				return err
			}
		}
		if m.States.Contains(i) {
			lastState = i
			misc.UnsetTrackers(i, &lastPos, &lastOrient, &lastState)
			hadFrame = true
		}

		volTemplate := filepath.Join(iterPath, "volume_molecules.dx")
		if err := misc.CheckDREAMMV3DXItems(i, lastPos, lastOrient, lastState,
			hadFrame, volTemplate); err != nil {
			return err
		}

		surfTemplate := filepath.Join(iterPath, "surface_molecules.dx")
		if err := misc.CheckDREAMMV3DXItems(i, lastPos, lastOrient, lastState,
			hadFrame, surfTemplate); err != nil {
			return err
		}
	}

	return nil
}

// checkDREAMMV3MeshBin checks the layout for mesh related data within the
// DREAMM v3 viz format
func checkDREAMMV3MeshBin(testDir, dataDir string, allIters, posIters,
	regionIters, stateIters jsonParser.IntList, meshEmpty bool) error {

	m, err := misc.CreateMolMeshIters(allIters, posIters, regionIters, stateIters)
	if err != nil {
		return err
	}

	dataPath := filepath.Join(file.GetOutputDir(testDir), dataDir)
	lastPos := -1
	lastRegion := -1
	lastState := -1

	for _, i := range m.All {
		iterPath := filepath.Join(dataPath, "frame_data", "iteration_%d")
		hadFrame := false

		// positions
		posFile := filepath.Join(iterPath, "mesh_positions.bin")
		if err := misc.CheckDREAMMV3IterItems(m.Pos, m.Combined, i, lastPos,
			meshEmpty, posFile); err != nil {
			return err
		}
		if m.Pos.Contains(i) {
			lastPos = i
			misc.UnsetTrackers(i, &lastPos, &lastRegion, &lastState)
			hadFrame = true
		}

		// regions
		regionFile := filepath.Join(iterPath, "region_indices.bin")
		if err := misc.CheckDREAMMV3IterItems(m.Others, m.States, i, lastRegion,
			meshEmpty, regionFile); err != nil {
			return err
		}
		if m.Others.Contains(i) {
			lastRegion = i
			misc.UnsetTrackers(i, &lastPos, &lastRegion, &lastState)
			hadFrame = true
		}

		// states
		statesFile := filepath.Join(iterPath, "mesh_states.bin")
		emptySet := set.NewIntSet()
		if err := misc.CheckDREAMMV3IterItems(m.States, emptySet, i, lastState,
			meshEmpty, statesFile); err != nil {
			return err
		}
		if m.States.Contains(i) {
			lastState = i
			misc.UnsetTrackers(i, &lastPos, &lastRegion, &lastState)
			hadFrame = true
		}

		template := filepath.Join(iterPath, "meshes.dx")
		if err := misc.CheckDREAMMV3DXItems(i, lastPos, lastRegion, lastState,
			hadFrame, template); err != nil {
			return err
		}
	}
	return nil
}

// checkDREAMMV3MeshASCII checks the layout for mesh related data within the
// DREAMM v3 viz format
func checkDREAMMV3MeshASCII(testDir, dataDir string, allIters, posIters,
	regionIters, stateIters jsonParser.IntList, meshEmpty bool, objects,
	objectRegions []string) error {

	m, err := misc.CreateMolMeshIters(allIters, posIters, regionIters, stateIters)
	if err != nil {
		return err
	}

	if len(objectRegions) == 0 {
		objectRegions = objects
	}

	dataPath := filepath.Join(file.GetOutputDir(testDir), dataDir)
	lastPos := -1
	lastRegion := -1
	lastState := -1

	for _, i := range m.All {
		iterPath := filepath.Join(dataPath, "frame_data", "iteration_%d")
		hadFrame := false

		// positions
		for _, obj := range objects {
			posFile := filepath.Join(iterPath, obj+".positions.dat")
			if err := misc.CheckDREAMMV3IterItems(m.Pos, m.Combined, i, lastPos,
				meshEmpty, posFile); err != nil {
				return err
			}
			conFile := filepath.Join(iterPath, obj+".connections.dat")
			if err := misc.CheckDREAMMV3IterItems(m.Pos, m.Combined, i, lastPos,
				meshEmpty, conFile); err != nil {
				return err
			}
		}
		if m.Pos.Contains(i) {
			lastPos = i
			misc.UnsetTrackers(i, &lastPos, &lastRegion, &lastState)
			hadFrame = true
		}

		// regions
		for _, obj := range objectRegions {
			regionFile := filepath.Join(iterPath, obj+".region_indices.dat")
			if err := misc.CheckDREAMMV3IterItems(m.Others, m.States, i, lastRegion,
				meshEmpty, regionFile); err != nil {
				return err
			}
		}
		if m.Others.Contains(i) {
			lastRegion = i
			misc.UnsetTrackers(i, &lastPos, &lastRegion, &lastState)
			hadFrame = true
		}

		// states
		for _, obj := range objects {
			statesFile := filepath.Join(iterPath, obj+".states.bin")
			emptySet := set.NewIntSet()
			if err := misc.CheckDREAMMV3IterItems(m.States, emptySet, i, lastState,
				meshEmpty, statesFile); err != nil {
				return err
			}
		}
		if m.States.Contains(i) {
			lastState = i
			misc.UnsetTrackers(i, &lastPos, &lastRegion, &lastState)
			hadFrame = true
		}

		template := filepath.Join(iterPath, "meshes.dx")
		if err := misc.CheckDREAMMV3DXItems(i, lastPos, lastRegion, lastState,
			hadFrame, template); err != nil {
			return err
		}
	}
	return nil
}

// checkDREAMMV3Grouped checks the layout for DREAMM V3 grouped format
func checkDREAMMV3Grouped(testDir, dataDir string, numIters, numTimes int,
	haveMeshPos, haveRgnIdx, haveMeshState, noMeshes, haveMolPos, haveMolOrient,
	haveMolState, noMols bool) error {

	dataPath := filepath.Join(file.GetOutputDir(testDir), dataDir)

	// meshes
	meshPath := dataPath + ".mesh_positions.1.bin"
	if err := misc.CheckDREAMMV3GroupedItem(meshPath, haveMeshPos, noMeshes); err != nil {
		return err
	}

	regionPath := dataPath + ".region_indices.1.bin"
	if err := misc.CheckDREAMMV3GroupedItem(regionPath, haveRgnIdx, noMeshes); err != nil {
		return err
	}

	meshStatesPath := dataPath + ".mesh_states.1.bin"
	if err := misc.CheckDREAMMV3GroupedItem(meshStatesPath, haveMeshState,
		noMeshes); err != nil {
		return err
	}

	// molecules
	molPath := dataPath + ".molecule_positions.1.bin"
	if err := misc.CheckDREAMMV3GroupedItem(molPath, haveMolPos, noMols); err != nil {
		return err
	}

	orientPath := dataPath + ".molecule_orientations.1.bin"
	if err := misc.CheckDREAMMV3GroupedItem(orientPath, haveMolOrient, noMols); err != nil {
		return err
	}

	molStatesPath := dataPath + ".molecule_states.1.bin"
	if err := misc.CheckDREAMMV3GroupedItem(molStatesPath, haveMolState, noMols); err != nil {
		return err
	}

	// iterations
	iterPath := dataPath + ".iteration_numbers.1.bin"
	if numIters != 0 {
		ok, err := file.HasSize(iterPath, int64(numIters*12))
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("file %s has incorrect file size", iterPath)
		}
	} else {
		ok, err := file.IsNonEmpty(iterPath)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("file %s is not non-empty", iterPath)
		}
	}

	// times
	timePath := dataPath + ".time_values.1.bin"
	if numTimes != 0 {
		ok, err := file.HasSize(timePath, int64(numTimes*8))
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("file %s has incorrect file size", timePath)
		}
	} else {
		ok, err := file.IsNonEmpty(timePath)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("file %s is not non-empty", timePath)
		}
	}

	return nil
}
