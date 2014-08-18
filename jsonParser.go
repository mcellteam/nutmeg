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
	"os"
	"path/filepath"
)

// ParseJSON takes the past to a test case and parses the test_description.json
// file contained therein into a TestDescription struct
func ParseJSON(testPath, includePath string) (*TestDescription, error) {
	content, err := ioutil.ReadFile(testPath)
	if err != nil {
		return nil, err
	}

	var test TestDescription
	err = json.Unmarshal(content, &test)
	if err != nil {
		return &test, err
	}
	for _, inc := range test.Includes {
		incFile := filepath.Join(includePath, inc+".json")
		t, err := ParseJSON(incFile, includePath)
		if err != nil {
			return nil, err
		}
		test.Checks = append(test.Checks, t.Checks...)
	}
	return &test, nil
}

// readConfig reads the configuration file
// NOTE: For now the name of the config file is assumed to be nutmeg.conf
// and is expected to be located in the same directory where the nutmeg
// executable is located
func readConfig() (*config, error) {
	currentDir, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	configPath := filepath.Join(currentDir, "nutmeg.conf")
	content, err := ioutil.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var myConf config
	err = json.Unmarshal(content, &myConf)
	if err != nil {
		return nil, err
	}
	return &myConf, nil
}
