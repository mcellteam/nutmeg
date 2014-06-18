// Copyright 2014 Markus Dittrich. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// util provides helper function for the nutmeg unit testing framework
package main

import (
	"fmt"
	"github.com/haskelladdict/datastruct/set/intset"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
)

// clean_output removes all files leftover from a previous test run
func cleanOutput(tests []string) error {
	for _, path := range tests {
		outputPath := filepath.Join(path, "output")
		if err := os.RemoveAll(outputPath); err != nil {
			return err
		}
	}
	return nil
}

// writeCmdLine writes the commandline with which MCell was called
// to the output directory
func writeCmdLine(mcellPath string, outputDir string, argList []string) error {
	cmdlineArgs := append([]string{mcellPath}, argList...)
	cmdlineArgs = append(cmdlineArgs, "\n")
	cmdline := strings.Join(cmdlineArgs, " ")
	cmdlinePath := filepath.Join(outputDir, "commandline.txt")
	return ioutil.WriteFile(cmdlinePath, []byte(cmdline), 0644)
}

// determineExitCode tries to figure out the exit code of a failed command
// execution via exec.Command(...).Run().
// NOTE: This will not work on windows - here we need
//                 return int(s.ExitCode), nil
func determineExitCode(err error) (int, error) {
	if e, ok := err.(*exec.ExitError); ok {
		if s, ok := e.Sys().(syscall.WaitStatus); ok {
			return s.ExitStatus(), nil
		}
	}
	return 0, err
}

// containsString checks if a given string is part of the provided string slice
// and returns true if yes and false otherwise
func containsString(ss []string, item string) bool {
	for _, s := range ss {
		if s == item {
			return true
		}
	}
	return false
}

// generateFileList takes a filename which could be a format string and a
// range and creates the corresponding list of filenames.
func generateFileList(name string, IDStringRange intList) ([]string, error) {

	// if no range is given we pass the name through as is
	if len(IDStringRange) == 0 {
		return []string{name}, nil
	}

	list, err := convertIntList(IDStringRange)
	if err != nil {
		return nil, err
	}

	var names []string
	for _, k := range list {
		names = append(names, fmt.Sprintf(name, k))
	}

	return names, nil
}

// convertIntList converts an intList expression into a sorted list of unique
// integers
func convertIntList(list intList) ([]int, error) {

	intMap := make(map[int]bool)
	for _, r := range list {
		if strings.Contains(r, ":") {
			list, err := convertRangeToList(r)
			if err != nil {
				return nil, err
			}
			for _, i := range list {
				intMap[i] = true
			}
		} else {
			i, err := strconv.Atoi(r)
			if err != nil {
				return nil, fmt.Errorf("cannot convert range value %s into integer", r)
			}
			intMap[i] = true
		}
	}

	var outList sort.IntSlice //[]int
	for k := range intMap {
		outList = append(outList, k)
	}
	outList.Sort()

	return outList, nil
}

// convertRangeToList converts a single string containing a range statement
// of the form "start:end:step" into an explicit integer list describing the
// range [4, 5, 6, 7, 8, 9].
// NOTE: end is not part of the range.
func convertRangeToList(rangeStatement string) ([]int, error) {

	rangeEndpoints := strings.Split(rangeStatement, ":")
	if len(rangeEndpoints) < 2 || len(rangeEndpoints) > 3 {
		return nil, fmt.Errorf("range selection %s not valid", rangeStatement)
	}

	var rangeBegin int
	var err error
	if rangeBegin, err = strconv.Atoi(rangeEndpoints[0]); err != nil {
		return nil, fmt.Errorf("invalid range start character %s", rangeEndpoints[0])
	}

	var rangeEnd int
	if rangeEnd, err = strconv.Atoi(rangeEndpoints[1]); err != nil {
		return nil, fmt.Errorf("invalid range end character %s", rangeEndpoints[1])
	}

	rangeStep := 1
	if len(rangeEndpoints) == 3 {
		if rangeStep, err = strconv.Atoi(rangeEndpoints[2]); err != nil {
			return nil, fmt.Errorf("invalid range step character %s", rangeEndpoints[1])
		}
	}

	var newRange []int
	for i := rangeBegin; i < rangeEnd; i += rangeStep {
		newRange = append(newRange, i)
	}

	return newRange, nil
}

// molIters is a small helper struct to bundle all frame data related to
// DREAMM V3 binary molecule data
type molIters struct {
	all                            []int
	surfPos, surfOrient, surfState *set.IntSet
	volPos, volOrient, volState    *set.IntSet
	molIters, surfIters, volIters  *set.IntSet
}

// createMolIters is a helper function for converting the list of
// input specified iterations at which molecule positions, orientations and
// states were output into corresponding lists of integer values. Unspecified
// items default to all iterations
func createMolList(allIters, surfPosIters, surfOrientIters, surfStateIters,
	volPosIters, volOrientIters, volStateIters intList) (*molIters, error) {

	var m molIters
	var err error
	if m.all, err = convertIntList(allIters); err != nil {
		return nil, err
	}

	surfPos, err := convertIntList(surfPosIters)
	if err != nil {
		return nil, err
	}
	if len(surfPos) == 0 {
		surfPos = m.all
	}
	m.surfPos = set.NewIntSet(surfPos...)

	surfOrient, err := convertIntList(surfOrientIters)
	if err != nil {
		return nil, err
	}
	if len(surfOrient) == 0 {
		surfOrient = m.all
	}
	m.surfOrient = set.NewIntSet(surfOrient...)

	surfState, err := convertIntList(surfStateIters)
	if err != nil {
		return nil, err
	}
	m.surfState = set.NewIntSet(surfState...)

	volPos, err := convertIntList(volPosIters)
	if err != nil {
		return nil, err
	}
	if len(volPos) == 0 {
		volPos = m.all
	}
	m.volPos = set.NewIntSet(volPos...)

	volOrient, err := convertIntList(volOrientIters)
	if err != nil {
		return nil, err
	}
	if len(volOrient) == 0 {
		volOrient = m.all
	}
	m.volOrient = set.NewIntSet(volOrient...)

	volState, err := convertIntList(volStateIters)
	if err != nil {
		return nil, err
	}
	m.volState = set.NewIntSet(volState...)

	// unions of all surface, volume, and molecules
	m.surfIters = m.surfPos.Clone().Union(m.surfOrient).Union(m.surfState)
	m.volIters = m.volPos.Clone().Union(m.volOrient).Union(m.volState)
	m.molIters = m.surfIters.Clone().Union(m.volIters)

	return &m, nil
}
