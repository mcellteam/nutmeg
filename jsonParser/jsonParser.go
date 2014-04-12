// Copyright 2014 Markus Dittrich. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// jsonParser parses a test description file for consumption by the
// test framework
package jsonParser

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
)

// TestDescription encapsulates all information needed to describe a unit
// or regression test of an MCell model
type TestDescription struct {
	description     string
	commandlineOpts string
	testCases       []TestCase
}

type TestCase struct {
	outputFile string
	testType   string
}

type parseStruct struct {
	Description string
	TestInfo    interface{}
}

// Parse takes the past to a test case and parses the test_description.json
// file contained therein into a TestDescription struct
func Parse(testPath string) (*TestDescription, error) {
	testDescriptionFile := filepath.Join(testPath, "test_description.json")
	content, err := ioutil.ReadFile(testDescriptionFile)
	if err != nil {
		return nil, err
	}

	var p parseStruct
	err = json.Unmarshal(content, &p)
	if err != nil {
		fmt.Println("Error parsing json:", err)
	}
	fmt.Println("**********", p)

	return &TestDescription{}, nil
}
