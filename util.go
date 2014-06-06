// Copyright 2014 Markus Dittrich. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// util provides helper function for the nutmeg unit testing framework
package main

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
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
