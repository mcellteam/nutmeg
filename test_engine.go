// Copyright 2014 Markus Dittrich. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// test_engine contains the actual test functions analysing
// the output of the run MCell simulations
package main

// testRunner analyses the TestDescriptions coming from an MCell run on a
// test and analyses them as requested per the TestDescription.
func testRunner(test *TestDescription, result chan *TestResult) {
	if !test.simStatus.success {
		result <- &TestResult{test.Path, false, test.simStatus.message}
		return
	}

	result <- &TestResult{test.Path, true, ""}
}
