// Copyright 2014-2016 Markus Dittrich. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package misc provides helper function for the nutmeg unit testing framework
package misc

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"

	"github.com/mcellteam/nutmeg/src/tomlParser"
)

// CleanOutput removes all files leftover from a previous test run
func CleanOutput(tests []string) error {
	for _, path := range tests {
		outputPath := filepath.Join(path, "output")
		if err := os.RemoveAll(outputPath); err != nil {
			return err
		}
	}
	return nil
}

// WriteCmdLine writes the commandline with which MCell was called
// to the output directory
func WriteCmdLine(mcellPath string, outputDir string, argList []string) error {
	cmdlineArgs := append([]string{mcellPath}, argList...)
	cmdlineArgs = append(cmdlineArgs, "\n")
	cmdline := strings.Join(cmdlineArgs, " ")
	cmdlinePath := filepath.Join(outputDir, "commandline.txt")
	return ioutil.WriteFile(cmdlinePath, []byte(cmdline), 0644)
}

// DetermineExitCode tries to figure out the exit code of a failed command
// execution via exec.Command(...).Run().
// NOTE: This will not work on windows - here we need
//                 return int(s.ExitCode), nil
func DetermineExitCode(err error) (int, error) {
	if e, ok := err.(*exec.ExitError); ok {
		if s, ok := e.Sys().(syscall.WaitStatus); ok {
			return s.ExitStatus(), nil
		}
	}
	return 0, err
}

// ContainsString checks if a given string is part of the provided string slice
// and returns true if yes and false otherwise
func ContainsString(ss []string, item string) bool {
	for _, s := range ss {
		if s == item {
			return true
		}
	}
	return false
}

// GenerateFileList takes a filename which could be a format string and a
// range and creates the corresponding list of filenames.
func GenerateFileList(name string, IDStringRange tomlParser.IntList) ([]string, error) {

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
func convertIntList(list tomlParser.IntList) ([]int, error) {

	intMap := make(map[int]bool)
	for _, r := range list {
		if strings.Contains(r, ":") {
			list, err := ConvertRangeToList(r)
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

// ConvertRangeToList converts a single string containing a range statement
// of the form "start:end:step" into an explicit integer list describing the
// range [4, 5, 6, 7, 8, 9].
// NOTE: end is not part of the range.
func ConvertRangeToList(rangeStatement string) ([]int, error) {

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

// UnsetTrackers resets the trackers used to keep track of symlinks used in
// the binary viz data test routines
func UnsetTrackers(s int, xs ...*int) {
	for _, x := range xs {
		if *x != s {
			*x = -1
		}
	}
}

// below are some useful math functions

// Abs returns the absolute value of integer i
func Abs(i int) int {
	var isize uint = strconv.IntSize - 1
	t := i >> isize
	return t ^ (i + t)
}
