// Copyright 2014 Markus Dittrich. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// test_engine contains the actual test functions analysing
// the output of the run MCell simulations
package main

import (
	"fmt"
	"path/filepath"
)

// TestDescription encapsulates all information needed to describe a unit
// or regression test of an MCell model
type TestDescription struct {
	Description     string
	CommandlineOpts []string
	Path            string
	Checks          []*TestCase
	simStatus       runStatus
}

type TestCase struct {
	TestType         string
	Description      string
	HaveHeader       bool
	DataFile         string
	CountConstraints []*Constraint
}

type Constraint struct {
	Target int
	Query  []int
}

// testRunner analyses the TestDescriptions coming from an MCell run on a
// test and analyses them as requested per the TestDescription.
func testRunner(test *TestDescription, result chan *TestResult) {
	for _, c := range test.Checks {
		switch c.TestType {
		case "CHECK_SUCCESS":
			if !test.simStatus.success {
				result <- &TestResult{test.Path, false, "CHECK_SUCCESS", test.simStatus.message}
				return // Special case - if simulation fails we won't continue testing
			} else {
				result <- &TestResult{test.Path, true, "CHECK_SUCCESS", ""}
			}

		case "COUNT_CONSTRAINTS":
			dataPath := filepath.Join(test.Path, "output", c.DataFile)
			success, err := checkCountConstraints(dataPath, c.HaveHeader, c.CountConstraints)
			if !success || err != nil {
				result <- &TestResult{test.Path, false, "COUNT_CONSTRAINTS", ""}
			} else {
				result <- &TestResult{test.Path, true, "COUNT_CONSTRAINTS", ""}
			}
		}
	}
}

// checkCountConstraints test the provided array of constraints
// on the simulation output data contained in the file filePath
// NOTE: Currenty this function assumes that the column counts
// in the Constrains match what's contained in the file.
func checkCountConstraints(filePath string, haveHeader bool,
	constraints []*Constraint) (bool, error) {

	// read data
	rows, err := readCounts(filePath, haveHeader)
	if err != nil {
		fmt.Println(filePath)
		return false, err
	}

	// check constraints for each data row
	for r := 0; r < len(rows.times); r++ {
		for _, con := range constraints {
			result := 0
			for c, q := range con.Query {
				result += (q * rows.counts[c][r])
			}

			if result != con.Target {
				return false, nil
			}
		}
	}

	return true, nil
}
