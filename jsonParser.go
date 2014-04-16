// Copyright 2014 Markus Dittrich. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// jsonParser parses a test description file for consumption by the
// test framework
package main

import (
	"encoding/json"
	"io/ioutil"
	"path/filepath"
)

// Parse takes the past to a test case and parses the test_description.json
// file contained therein into a TestDescription struct
func ParseJSON(testPath string) (*TestDescription, error) {
	testDescriptionFile := filepath.Join(testPath, "test_description.json")
	content, err := ioutil.ReadFile(testDescriptionFile)
	if err != nil {
		return nil, err
	}

	var test TestDescription
	err = json.Unmarshal(content, &test)
	if err != nil {
		return &test, err
	}

	return &test, nil
}
