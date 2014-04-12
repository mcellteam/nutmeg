// Copyright 2014 Markus Dittrich. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// jsonParser parses a test description file for consumption by the
// test framework
package jsonParser

// TestDescription encapsulates all information needed to describe a unit
// or regression test of an MCell model
type TestDescription struct {
	description     string
	commandlineOpts string
	testCases       []testCase
}

type TestCase struct {
	outputFile string
	testType   string
}
