// Copyright 2014 Markus Dittrich. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// io contains routines for reading of data files
package main

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

type Columns struct {
	times  []float64
	counts [][]int
}

// readCounts reads in the time values and counts from the provided
// reaction data file and returns them as a Column struct
func readCounts(fileName string, haveHeader bool) (*Columns, error) {

	file, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	// throw away header
	if haveHeader {
		scanner.Scan()
	}

	// read row by row
	var cols Columns
	for r := 0; scanner.Scan(); r++ {
		lineItems := strings.Fields(scanner.Text())

		if r == 0 {
			cols.times = make([]float64, 0)
			n := len(lineItems) - 1
			cols.counts = make([][]int, n)
			for i := 0; i < n; i++ {
				cols.counts[i] = make([]int, 0)
			}
		}

		t, err := strconv.ParseFloat(lineItems[0], 64)
		if err != nil {
			return nil, err
		}
		cols.times = append(cols.times, t)

		for i, cs := range lineItems[1:] {
			c, err := strconv.Atoi(cs)
			if err != nil {
				return nil, err
			}
			cols.counts[i] = append(cols.counts[i], c)
		}
	}
	return &cols, nil
}
