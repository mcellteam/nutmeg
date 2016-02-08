// Copyright 2014-2016 Markus Dittrich. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package jsonParser parses a test description file for consumption by the
// test framework
package jsonParser

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
)

// Config keeps track of package Configuration settings
type Config struct {
	McellPath  string // path to mcell executable
	TestDir    string // path to directory with nutmeg tests
	IncludeDir string // path to directory with nutmeg test include file
}

// TestDescription encapsulates all information needed to describe a unit
// or regression test of an MCell model
// NOTE: the JSON test includes are assumed to be in a directory json_includes
// in the top level nutmeg directory
type TestDescription struct {
	Description string
	Path        string
	KeyWords    []string
	Includes    []string // names of JSON test description files to be included
	Run         RunSpec  // simulation runs to conduct as part of this test
	Checks      []*TestCase
	//	SimStatus   []RunStatus // status of all simulation runs
}

// RunSpec describes an individual run to be conducted as part of a single
// mcell test.
type RunSpec struct {
	MdlFiles        []string // name of mdl file to run
	NumSeeds        int      // number of seeds to run
	CommandlineOpts []string // commandline options for this run
	Seed            int      // seed value for this particular run
	RunID           int      // unique ID for this run needed to collect results for multi seed runs
}

// TestCase describes an individual test case of an overall test
type TestCase struct {
	testCommon
	testCompareCounts
	testConstraints
	testExitCode
	testMeans
	testMinMax
	testPatternMatch
	testRates
	testTrigger
	testFileSizes
	testDiffFileContent
	testLegacyVolOutput
	testASCIIVizOutput
	testCheckPoint
	testDREAMMV3Common
	testDREAMMV3MolBinOutput
	testDREAMMV3MolASCIIOutput
	testDREAMMV3MeshASCIIOutput
	testDREAMMV3GroupedOutput
}

// testCommon includes common options that are used by two or more tests
type testCommon struct {
	TestType    string  // test type - used to dispatch appropriate testing function
	Description string  // textual description of test case
	HaveHeader  bool    // indicates if DataFile contains a header
	AverageData bool    // test averaged data (only useful for multiple seeds)
	DataFile    string  // name of (output) file to test
	MinTime     float64 // ignore all data items before MinTime for testing
	MaxTime     float64 // ignore all data items after MaxTime for testing
}

// testRates pertains to testing average reaction rates
type testRates struct {
	BaseTime float64 // base time used for computing reaction rates from counts
}

// testExitCode pertains to testing the exit code of simulations
type testExitCode struct {
	ExitCode int // expected exit code of MCell run
}

// testMinMax pertains to checks testing that data is within certain ranges
type testMinMax struct {
	CountMaximum []int // test if counts are larger than provided minmum
	CountMinimum []int // test if counts are smaller than provided maximum
}

// testConstraints pertains to checks testing that the data count columns
// satisfy simple arithmetic constraints
type testConstraints struct {
	CountConstraints []*ConstraintSpec // test if counts fullfill the provided constraints
}

// testPatternMatch partains to checks that test if certains string patterns
// are present in output files
type testPatternMatch struct {
	MatchPattern string // test pattern to match file against
	NumMatches   int    // number of expected pattern matches
}

// testCompareCounts pertains to checks comparing data against reference
// counts. If absDeviation or relDeviation are provided the actual data
// is compared to the reference data taking into account the relative or
// absolute deviation. If absDeviation or relDeviation are not provided
// they are assumed to be 0. Both absDeviation and relDeviation are arrays with
// one value per data column. Any non-specified columns are assumed to be zero,
// any additional values are ignored.
type testCompareCounts struct {
	ReferenceFile string    // name of file with reference counts to compare against
	AbsDeviation  []int     // allowed absolute deviation from reference, one per column
	RelDeviation  []float64 // allowed relative deviation from reference, one per column
}

// testMeans pertaints to checks testing that data values have a certain mean
// and fluctuation within the given tolerances
type testMeans struct {
	Means      []float64 // target column means for count equilibrium tests
	Tolerances []float64 // tolerances by which actual colummn means may deviate from target
}

// testTriggers pertains to checks testing the integrity of trigger data
type testTrigger struct {
	TriggerType   string    // what trigger is this "reactions", "hits", "molCounts"
	HaveExactTime bool      // is the exact event time part of the trigger data
	OutputTime    float64   // output time step
	Xrange        []float64 // tuple of valid x ranges for triggered events
	Yrange        []float64 // tuple of valid y ranges for triggered events
	Zrange        []float64 // typle of valid z ranges for triggered events
}

// testFileSizes pertains to checks testing that the given list of files
// exists and each file is either emtpy or non-empty with a given size.
// FileNames can contain format strings containing integer (%d) specifiers. In
// this case IDRange needs to be a list of strings describing a range. Each item
// can either correspond to an integer or a range of the form start:end:step.
// FileSize is the optional size of the file (for each of the files in the
// interpolated list of files).
type testFileSizes struct {
	FileNames []string // the filenames (can each be format string)
	IDRange   IntList  // list of strings describing a numeric range, e.g. [1, 2, 3:100:5]
	FileSize  int64    // file size in bytes
}

// testDiffFileContent pertains that check the content of a file against a
// template file. The template file can be a format string whose format
// interpolation works similar to go strings. The templateParameters member
// describes the kind of interpolation to be performed
type testDiffFileContent struct {
	TemplateFile       string   // name of template file
	TemplateParameters []string // list of parameters to interpolate into template file
}

// testVolOutput check if the legacy volume output has the proper format
// and correct number of data items
type testLegacyVolOutput struct {
	Xdim int // voxel count in x dimension
	Ydim int // voxel count in y dimension
	Zdim int // voxel count in z dimension
}

// testASCIIVizOutput does some basic check on the consistency of the legacy
// MCell2 VIZ_DATA_OUTPUT.
type testASCIIVizOutput struct {
	SurfaceStates []int
	VolumeStates  []int
}

// testCheckPoint does basic timing tests involving checkpoints
type testCheckPoint struct {
	BaseName string
	Delay    float64 // delay in seconds at which checkpoint should happen
	Margin   float64 // acceptable margin for checkpoint delay in seconds
}

// testDREAMMV3MeshCommon contains common items used in DREAMM V3 molecule
// and mesh tests
type testDREAMMV3Common struct {
	AllIters    IntList // list of all frames *required*
	PosIters    IntList // iterations with molecule/mesh positions
	OrientIters IntList // iteractions with molecule orientations
	RegionIters IntList // iterations with region information
	StateIters  IntList // iterations with molecule/mesh state information
	VizPath     string  // path to viz data output directory
	MeshEmpty   bool    // true if no mesh info is present
}

// testDREAMMV3MeshASCIIOutput encapsulates items specific to the ASCII format
// of the DREAMM_V3 mesh viz data output
type testDREAMMV3MeshASCIIOutput struct {
	Objects       []string // names of mesh objects which should be present
	ObjectRegions []string // names of objects with regions
}

// testDREAMMV3MolBinOutput test the DREAMM_V3 molecule viz data output.
// NOTE: It is a bit tricky to split this test into more elementary tests since
// the framework also checks the existence of the proper soft links which
// depend on the overall iteration structure (e.g., an iteration directory
// without a requested molecule output receives links to the most recently
// added data in a previous iteration). Thus, it seemed better to hardcode
// everything into a more complex single test.
type testDREAMMV3MolBinOutput struct {
	SurfPosIters    IntList // iterations with surface mol. positions
	SurfOrientIters IntList // iterations with surface mol. orientations
	SurfStateIters  IntList // iterations with surface mol. states
	SurfEmpty       bool    // true if no surface molecules are present
	VolPosIters     IntList // iterations with volume mol. positions
	VolOrientIters  IntList // iterations with surface mol. orientation iterations
	VolStateIters   IntList // iterations with surface mol. states
	VolEmpty        bool    // true if no volume molecules are present
}

// testDREAMMV3MolASCIIOutput encapsulates specs related to tests for the
// ASCII viz format of the DREAMM V3 molecule data
type testDREAMMV3MolASCIIOutput struct {
	MolNames []string // names of molecules to look for
}

// testDREAMMV3Grouped encapsulates specs related to tests for DREAMM V3
// grouped format.
type testDREAMMV3GroupedOutput struct {
	NumIters      int  // number of iterations with output
	NumTimes      int  // number of times with output
	HaveMeshPos   bool // true if a mesh positions file is expected
	HaveRgnIdx    bool // true if a region index file is expected
	HaveMeshState bool // true if a mesh state file is expected
	NoMeshes      bool // true if no mesh is present
	HaveMolPos    bool // true if a molecule positions file is expected
	HaveMolOrient bool // true if a molecule orientations file is expected
	HaveMolState  bool // true if a molecule states file is expected
	NoMols        bool // true if no molecules are present
}

// ConstraintSpec encapsulates a single constraint specification.
type ConstraintSpec struct {
	Target int
	Query  []int
}

// IntList is a parse time list of strings which will be converted into an
// integer range. Each item is either a string representation of an integer or
// an integer range of the form start:end:step.
// Exampe: [1, 2, 3:100:5]
type IntList []string

// Copy member function for a TestDescription
func (t *TestDescription) Copy() *TestDescription {
	newT := TestDescription{t.Description, t.Path, t.KeyWords, t.Includes,
		t.Run, t.Checks}
	return &newT
}

// Parse takes the past to a test case and parses the test_description.json
// file contained therein into a TestDescription struct
func Parse(testPath, includePath string) (*TestDescription, error) {
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
		t, err := Parse(incFile, includePath)
		if err != nil {
			return nil, err
		}
		test.Checks = append(test.Checks, t.Checks...)
	}
	return &test, nil
}

// ReadConfig reads the Configuration file
// NOTE: For now the name of the Config file is assumed to be nutmeg.conf
// and is expected to be located in the same directory where the nutmeg
// executable is located
func ReadConfig() (*Config, error) {
	currentDir, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	ConfigPath := filepath.Join(currentDir, "nutmeg.conf")
	content, err := ioutil.ReadFile(ConfigPath)
	if err != nil {
		return nil, err
	}

	var myConf Config
	err = json.Unmarshal(content, &myConf)
	if err != nil {
		return nil, err
	}
	return &myConf, nil
}
