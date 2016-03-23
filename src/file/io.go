// Copyright 2014-2016 Markus Dittrich. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package file contains routines for reading of data files
package file

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// name of output directory
const outputDirName = "output"

// Columns describes the content of a reaction data output file including a
// column of time values and an arbitrary number of integer data columns
type Columns struct {
	Times  []float64
	Counts [][]int
}

// StringColumns describes the content of a trigger data output file including a
// column of time values and an arbitrary number of string valued data columns
// NOTE: The data is kept as string values since they can contain either
// integer or float values and we have to wait with coercing them until run time
type StringColumns struct {
	Times  []float64
	Values [][]string
}

// LoadData reads all the reaction count data in the file paths provided by dataPaths
// and either returns the individually as a list or averages them
func LoadData(dataPaths []string, haveHeader, averageData bool) ([]*Columns, error) {

	var data []*Columns
	if averageData {
		cols, err := readAverageCounts(dataPaths, haveHeader)
		if err != nil {
			return nil, err
		}
		data = append(data, cols)
	} else {
		for _, dataPath := range dataPaths {
			cols, err := ReadCounts(dataPath, haveHeader)
			if err != nil {
				return nil, err
			}
			data = append(data, cols)
		}
	}
	return data, nil
}

// readAverageCounts parses all data in in the list of reaction data
// filenames and computes and returns the average.
//
// NOTE: this function assumes that the data files all have the same
// shape, i.e. the same number of rows and columns
//
// NOTE: the average computation is done with integer arithmetic
func readAverageCounts(fileNames []string, haveHeader bool) (*Columns, error) {

	var averageCols *Columns
	for i, fileName := range fileNames {
		col, err := ReadCounts(fileName, haveHeader)
		if err != nil {
			return nil, err
		}

		if i != 0 {
			for r := 0; r < len(averageCols.Times); r++ {
				for c := 0; c < len(averageCols.Counts); c++ {
					averageCols.Counts[c][r] += col.Counts[c][r]
				}
			}
		} else { // set the average to the first data set
			averageCols = col
		}
	}

	numDataSets := len(fileNames)
	for r := 0; r < len(averageCols.Times); r++ {
		for c := 0; c < len(averageCols.Counts); c++ {
			averageCols.Counts[c][r] = averageCols.Counts[c][r] / numDataSets
		}
	}
	return averageCols, nil
}

// ReadCounts reads in the time values and counts from the provided
// reaction data file and returns them as a Column struct
func ReadCounts(fileName string, haveHeader bool) (*Columns, error) {

	file, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	// throw away header
	if haveHeader {
		scanner.Scan()
	}

	// read row by row
	var cols Columns
	for r := 0; scanner.Scan(); r++ {
		lineItems := strings.Fields(scanner.Text())

		if r == 0 {
			cols.Times = make([]float64, 0)
			n := len(lineItems) - 1
			cols.Counts = make([][]int, n)
			for i := 0; i < n; i++ {
				cols.Counts[i] = make([]int, 0)
			}
		}

		t, err := strconv.ParseFloat(lineItems[0], 64)
		if err != nil {
			return nil, err
		}
		cols.Times = append(cols.Times, t)

		for i, cs := range lineItems[1:] {
			c, err := strconv.Atoi(cs)
			if err != nil {
				return nil, err
			}
			cols.Counts[i] = append(cols.Counts[i], c)
		}
	}

	// sanity check - we expect at least one row of data
	if len(cols.Times) == 0 {
		return nil, fmt.Errorf("%s: contains no data", fileName)
	}

	return &cols, nil
}

// LoadStringData reads all the reaction data columns as strings. The
// string data loader is used for analyzing trigger data since this
// typically contains a mix of integer and float data
func LoadStringData(dataPaths []string, haveHeader bool) ([]*StringColumns, error) {

	var data []*StringColumns
	for _, dataPath := range dataPaths {
		cols, err := readStringCounts(dataPath, haveHeader)
		if err != nil {
			return nil, err
		}
		data = append(data, cols)
	}
	return data, nil
}

// readStringCounts reads in the time values and the rest of the column
// data (could be ints, floats) and returns them as a StringColumns struct
func readStringCounts(fileName string, haveHeader bool) (*StringColumns, error) {

	file, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	// throw away header
	if haveHeader {
		scanner.Scan()
	}

	// read row by row
	var cols StringColumns
	for r := 0; scanner.Scan(); r++ {
		lineItems := strings.Fields(scanner.Text())

		if r == 0 {
			cols.Times = make([]float64, 0)
			n := len(lineItems) - 1
			cols.Values = make([][]string, n)
			for i := 0; i < n; i++ {
				cols.Values[i] = make([]string, 0)
			}
		}

		t, err := strconv.ParseFloat(lineItems[0], 64)
		if err != nil {
			return nil, err
		}
		cols.Times = append(cols.Times, t)

		for i, c := range lineItems[1:] {
			cols.Values[i] = append(cols.Values[i], c)
		}
	}

	// sanity check - we expect at least one row of data
	if len(cols.Times) == 0 {
		return nil, fmt.Errorf("%s: contains no data", fileName)
	}

	return &cols, nil
}

// GetDataPaths returns a list of all reaction data files names that were
// generated as part of this run (at least one but could be many for multi
// seed runs)
func GetDataPaths(path, dataFile string, seed, numSeeds int) ([]string, error) {

	var dataPaths []string
	dataDir := GetOutputDir(path)

	// check if data file has a single format specifier
	count := strings.Count(dataFile, "%")

	switch count {
	case 0:
		filePath := filepath.Join(dataDir, dataFile)
		dataPaths = append(dataPaths, filePath)
	case 1:
		if numSeeds == 1 {
			fileName := fmt.Sprintf(dataFile, seed)
			filePath := filepath.Join(dataDir, fileName)
			dataPaths = append(dataPaths, filePath)
		} else {
			for i := 1; i < numSeeds+1; i++ {
				fileName := fmt.Sprintf(dataFile, i)
				filePath := filepath.Join(dataDir, fileName)
				dataPaths = append(dataPaths, filePath)
			}
		}
	default:
		return nil, fmt.Errorf("datafile has too many format specifiers")
	}

	// expand all "*" glob patterns if present
	var outDataPaths []string
	for _, p := range dataPaths {
		if strings.ContainsAny(p, "*") {
			paths, err := filepath.Glob(p)
			if err != nil {
				return nil, fmt.Errorf("Failed to expand glob patterns in %s", p)
			}
			outDataPaths = append(outDataPaths, paths...)
		} else {
			outDataPaths = append(outDataPaths, p)
		}
	}

	// make sure we have at least one valid output file, i.e., even when
	// globbing we expect at least on match
	if len(outDataPaths) == 0 {
		return nil, fmt.Errorf("Could not construct any valid output files " +
			"perhaps due to failed globbing.")
	}

	return outDataPaths, nil
}

// GetOutputDir returns the path in which the output for the testcase at path
// is located
func GetOutputDir(testPath string) string {
	return filepath.Join(testPath, outputDirName)
}

// IsEmpty checks that the given file exists and is empty
func IsEmpty(filePath string) (bool, error) {
	fi, err := os.Stat(filePath)
	if err != nil {
		return false, err
	}

	if fi.Size() != 0 {
		return false, nil
	}
	return true, nil
}

// IsNonEmpty check that the given file exists and is non-empty.
func IsNonEmpty(testPath string) (bool, error) {
	fi, err := os.Stat(testPath)
	if err != nil {
		return false, err
	}

	if fi.Size() == 0 {
		return false, nil
	}
	return true, nil
}

// Exists checks that the given file exists
func Exists(testPath string) (bool, error) {
	_, err := os.Stat(testPath)
	if err != nil {
		return false, nil
	}
	return true, nil
}

// HasSize that the given file exists and has the requested
// file size
func HasSize(testPath string, size int64) (bool, error) {
	fi, err := os.Stat(testPath)
	if err != nil {
		return false, err
	}

	if fi.Size() != size {
		return false, nil
	}
	return true, nil
}

// NoFile checks that there is no file at the given path
func NoFile(testPath string) (bool, error) {
	if _, err := os.Stat(testPath); os.IsNotExist(err) {
		return true, nil
	}
	return false, nil
}

// IsSymLink checkes that the given path exists, is a symlink and points to
// the provided file.
func IsSymLink(destFilePath, filePath string) (bool, error) {
	targetPath, err := os.Readlink(filePath)
	if err != nil {
		return false, err
	}
	if targetPath != destFilePath {
		return false, nil
	}
	return true, nil
}
