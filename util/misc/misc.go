// Copyright 2014 Markus Dittrich. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// misc provides helper function for the nutmeg unit testing framework
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

	"github.com/haskelladdict/datastruct/set/intset"
	"github.com/haskelladdict/nutmeg/util/file"
	"github.com/haskelladdict/nutmeg/util/jsonParser"
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

// containsString checks if a given string is part of the provided string slice
// and returns true if yes and false otherwise
func ContainsString(ss []string, item string) bool {
	for _, s := range ss {
		if s == item {
			return true
		}
	}
	return false
}

// generateFileList takes a filename which could be a format string and a
// range and creates the corresponding list of filenames.
func GenerateFileList(name string, IDStringRange jsonParser.IntList) ([]string, error) {

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
func convertIntList(list jsonParser.IntList) ([]int, error) {

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

// CheckDREAMMV2IterItems examines the viz iterations directory for
// a specific items (surface positions, orientations, etc.)
func CheckDREAMMV3IterItems(molSet, molIters *set.IntSet, iter, lastPos int,
	isEmpty bool, fileTemplate string) error {

	fileName := fmt.Sprintf(fileTemplate, iter)
	if molSet.Contains(iter) {
		if isEmpty {
			ok, err := file.Exists(fileName)
			if err != nil {
				return err
			} else if !ok {
				return fmt.Errorf("file %s does not exists", fileName)
			}
		} else {
			ok, err := file.IsNonEmpty(fileName)
			if err != nil {
				return err
			} else if !ok {
				return fmt.Errorf("file %s is not non-empty as expected", fileName)
			}
		}
	} else if lastPos >= 0 && !molIters.Contains(iter) {
		fileTemplate := filepath.Join("../iteration_%d", filepath.Base(fileName))
		linkName := fmt.Sprintf(fileTemplate, lastPos)
		ok, err := file.IsSymLink(linkName, fileName)
		if err != nil {
			return err
		} else if !ok {
			return fmt.Errorf("file %s is not properly symlinked to %s", fileName, linkName)
		}
	} else {
		ok, err := file.NoFile(fileName)
		if err != nil {
			return err
		} else if !ok {
			return fmt.Errorf("file %s exists but shouldn't", fileName)
		}
	}
	return nil
}

// CheckDREAMMV3DXitems checks the presence of the correct dx files/symlinks
// in a given viz iteration directory.
// NOTE: lastProperty could refer to molecule orientations or regions for meshes
func CheckDREAMMV3DXItems(iter, lastPos, lastProperty, lastState int,
	hadFrame bool, fileTemplate string) error {

	pos := -1
	if lastPos >= 0 {
		pos = lastPos
	} else if lastProperty >= 0 {
		pos = lastProperty
	} else if lastState >= 0 {
		pos = lastState
	}

	fileName := fmt.Sprintf(fileTemplate, iter)
	if hadFrame {
		ok, err := file.IsNonEmpty(fileName)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("file %s is non non-empty as expected", fileName)
		}
	} else if pos >= 0 {
		fileTemplate := filepath.Join("../iteration_%d", filepath.Base(fileName))
		linkName := fmt.Sprintf(fileTemplate, pos)
		ok, err := file.IsSymLink(linkName, fileName)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("file %s is not properly symlinked to %s as expected",
				fileName, linkName)
		}
	} else {
		ok, err := file.NoFile(fileName)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("file %s exists but was expected to not be present",
				fileName)
		}
	}
	return nil
}

// meshIters is a small helper struct to bundle all frame data related to
// DREAMM V3 binary mesh data
type molMeshIters struct {
	All                   []int
	Pos, Others, States   *set.IntSet
	Combined, AllCombined *set.IntSet
}

// CreateMeshIters is a helper function for converting the list of
// input specified iterations at which mesh positions, regions and
// states were output into corresponding lists of integer values.
func CreateMolMeshIters(allIters, posIters, otherIters,
	stateIters jsonParser.IntList) (*molMeshIters, error) {

	var m molMeshIters
	var err error
	if m.All, err = convertIntList(allIters); err != nil {
		return nil, err
	}

	pos, err := convertIntList(posIters)
	if err != nil {
		return nil, err
	}
	if len(pos) == 0 {
		pos = m.All
	}
	m.Pos = set.NewIntSet(pos...)

	others, err := convertIntList(otherIters)
	if err != nil {
		return nil, err
	}
	if len(others) == 0 {
		others = m.All
	}
	m.Others = set.NewIntSet(others...)

	states, err := convertIntList(stateIters)
	if err != nil {
		return nil, err
	}
	m.States = set.NewIntSet(states...)

	m.Combined = m.Others.Clone().Union(m.States)
	m.AllCombined = m.Combined.Clone().Union(m.Pos)

	return &m, nil
}

// unsetTrackers resets the trackers used to keep track of symlinks used in
// the binary viz data test routines
func UnsetTrackers(s int, xs ...*int) {
	for _, x := range xs {
		if *x != s {
			*x = -1
		}
	}
}

// CheckDREAMMV3GroupedItem tests the viz data directory for the
// presence/absence of the given file as part of grouped DREAMM V3 format
func CheckDREAMMV3GroupedItem(filePath string, haveItemProperty, noItem bool) error {
	if haveItemProperty && noItem {
		ok, err := file.IsEmpty(filePath)
		if err != nil {
			return err
		} else if !ok {
			return fmt.Errorf("file %s does not exist or is not empty", filePath)
		}
	} else if haveItemProperty && !noItem {
		ok, err := file.IsNonEmpty(filePath)
		if err != nil {
			return err
		} else if !ok {
			return fmt.Errorf("file %s is not non-empty", filePath)
		}
	}
	return nil
}
