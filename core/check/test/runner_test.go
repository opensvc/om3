package check_test

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/core/check"
)

func fakeExecCommand(command string, args ...string) *exec.Cmd {
	cs := []string{"-test.run=TestHelperProcess", "--", command}
	cs = append(cs, args...)
	cmd := exec.Command(os.Args[0], cs...)
	cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
	return cmd
}

func TestHelperProcess(t *testing.T) {
	t.Helper()
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	args := os.Args
	for i, arg := range args {
		if arg == "--" {
			args = args[i+1:]
			break
		}
	}

	var exitCode int
	var out, err string
	defer func() { os.Exit(exitCode) }()
	cmd := args[0]
	switch {
	case cmd == "succeed":
	case cmd == "succeedWithInvalidOut":
		out = "{}["
	case cmd == "exitCode3":
		exitCode = 3
	case strings.Contains(cmd, "succeedWithOut"):
		data := check.ResultSet{
			Data: []check.Result{
				{
					DriverGroup: "group1",
					DriverName:  cmd,
					Path:        "path/" + cmd,
					Instance:    "1",
					Unit:        "count",
					Value:       2,
				},
			},
		}
		outB, err := json.Marshal(data)
		if err != nil {
			t.Fatal(err)
		}
		out = string(outB)
	case cmd == "failWithCorrectOut":
		data := check.ResultSet{
			Data: []check.Result{
				{
					DriverGroup: "group1",
					DriverName:  cmd,
					Path:        "path/" + cmd,
					Instance:    "1",
					Unit:        "count",
					Value:       2,
				},
			},
		}
		outB, err := json.Marshal(data)
		if err != nil {
			t.Fatal(err)
		}
		out = string(outB)
		exitCode = 1
	case cmd == "failWithOutAndErr":
		out = "some output"
		err = "some error"
		exitCode = 1
	}

	if out != "" {
		_, _ = fmt.Fprint(os.Stdout, out)
	}
	if err != "" {
		_, _ = fmt.Fprint(os.Stderr, err)
	}
	return
}

type fakeChecker struct {
	DriverGroup string
	DriverName  string
	IDs         []string
	Unit        string
}

func (t fakeChecker) Check(objs []interface{}) (*check.ResultSet, error) {
	var results []check.Result
	if strings.Contains(t.DriverGroup, "error") {
		return &check.ResultSet{}, fmt.Errorf("something wrong happen")
	}
	for _, id := range t.IDs {
		results = append(results, check.Result{
			DriverGroup: t.DriverGroup,
			DriverName:  t.DriverName,
			Path:        "path-" + id,
			Instance:    "instance-" + id,
			Unit:        "",
			Value:       0,
		})
	}
	return &check.ResultSet{Data: results}, nil
}

func TestRunnerDo(t *testing.T) {
	check.ExecCommand = fakeExecCommand
	defer func() { check.ExecCommand = exec.Command }()
	var checker1, checker2, errorChecker check.Checker
	checker1 = &fakeChecker{
		DriverGroup: "checker1Grp",
		DriverName:  "checker1Drv",
		IDs:         []string{"a", "b"},
		Unit:        "",
	}
	checker2 = &fakeChecker{
		DriverGroup: "checker2Grp",
		DriverName:  "checker2Drv",
		IDs:         []string{"a"},
		Unit:        "",
	}
	errorChecker = &fakeChecker{
		DriverGroup: "error",
		DriverName:  "checker",
		IDs:         []string{"a"},
		Unit:        "",
	}
	cases := []struct {
		Name               string
		CustomCheckPaths   []string
		RegisteredCheckers []check.Checker
		ExpectedResults    []check.Result
	}{
		{
			Name:             "succeedWithInvalidOut",
			CustomCheckPaths: []string{"succeedWithInvalidOut"},
			ExpectedResults:  []check.Result{},
		},
		{
			Name:             "withoutCustomCheckers",
			CustomCheckPaths: []string{},
			ExpectedResults:  []check.Result{},
		},
		{
			Name:             "withOneFailedChecker",
			CustomCheckPaths: []string{"exitCode3"},
			ExpectedResults:  nil,
		},
		{
			Name:             "withOneSucceedCustomCheckers",
			CustomCheckPaths: []string{"succeedWithOut"},
			ExpectedResults: []check.Result{
				{
					DriverGroup: "group1",
					DriverName:  "succeedWithOut",
					Path:        "path/succeedWithOut",
					Instance:    "1",
					Unit:        "count",
					Value:       int64(2),
				},
			},
		},
		{
			Name:               "withOneSucceedCustomAndRegisteredCheckers",
			CustomCheckPaths:   []string{"succeedWithOut"},
			RegisteredCheckers: []check.Checker{checker1},
			ExpectedResults: []check.Result{
				{
					DriverGroup: "group1",
					DriverName:  "succeedWithOut",
					Path:        "path/succeedWithOut",
					Instance:    "1",
					Unit:        "count",
					Value:       int64(2),
				},
				{
					DriverGroup: "checker1Grp",
					DriverName:  "checker1Drv",
					Path:        "path-a",
					Instance:    "instance-a",
					Unit:        "",
					Value:       int64(0),
				},
				{
					DriverGroup: "checker1Grp",
					DriverName:  "checker1Drv",
					Path:        "path-b",
					Instance:    "instance-b",
					Unit:        "",
					Value:       int64(0),
				},
			},
		},
		{
			Name:               "withOneSucceedCustomAndTwoRegisteredCheckers",
			CustomCheckPaths:   []string{"succeedWithOut"},
			RegisteredCheckers: []check.Checker{checker1, checker2},
			ExpectedResults: []check.Result{
				{
					DriverGroup: "group1",
					DriverName:  "succeedWithOut",
					Path:        "path/succeedWithOut",
					Instance:    "1",
					Unit:        "count",
					Value:       int64(2),
				},
				{
					DriverGroup: "checker1Grp",
					DriverName:  "checker1Drv",
					Path:        "path-a",
					Instance:    "instance-a",
					Unit:        "",
					Value:       int64(0),
				},
				{
					DriverGroup: "checker1Grp",
					DriverName:  "checker1Drv",
					Path:        "path-b",
					Instance:    "instance-b",
					Unit:        "",
					Value:       int64(0),
				},
				{
					DriverGroup: "checker2Grp",
					DriverName:  "checker2Drv",
					Path:        "path-a",
					Instance:    "instance-a",
					Unit:        "",
					Value:       int64(0),
				},
			},
		},
		{
			Name:               "succeedCustomChecker, error and succeed registered checkers",
			CustomCheckPaths:   []string{"succeedWithOut"},
			RegisteredCheckers: []check.Checker{errorChecker, checker2},
			ExpectedResults: []check.Result{
				{
					DriverGroup: "group1",
					DriverName:  "succeedWithOut",
					Path:        "path/succeedWithOut",
					Instance:    "1",
					Unit:        "count",
					Value:       int64(2),
				},
				{
					DriverGroup: "checker2Grp",
					DriverName:  "checker2Drv",
					Path:        "path-a",
					Instance:    "instance-a",
					Unit:        "",
					Value:       int64(0),
				},
			},
		},
		{
			Name:             "withSomeSucceedCustomCheckers",
			CustomCheckPaths: []string{"succeedWithOut1", "succeedWithOut2"},
			ExpectedResults: []check.Result{
				{
					DriverGroup: "group1",
					DriverName:  "succeedWithOut1",
					Path:        "path/succeedWithOut1",
					Instance:    "1",
					Unit:        "count",
					Value:       int64(2),
				},
				{
					DriverGroup: "group1",
					DriverName:  "succeedWithOut2",
					Path:        "path/succeedWithOut2",
					Instance:    "1",
					Unit:        "count",
					Value:       int64(2),
				},
			},
		},
		{
			Name:             "withSomeFailedCustomCheckers",
			CustomCheckPaths: []string{"succeedWithOut", "exitCode3"},
			ExpectedResults: []check.Result{
				{
					DriverGroup: "group1",
					DriverName:  "succeedWithOut",
					Path:        "path/succeedWithOut",
					Instance:    "1",
					Unit:        "count",
					Value:       int64(2),
				},
			},
		},
		{
			Name:             "withWithCorrectOutputButBadExitCode",
			CustomCheckPaths: []string{"failWithCorrectOut"},
			ExpectedResults:  []check.Result{},
		},
		{
			Name:             "withFailedCustomCheckers",
			CustomCheckPaths: []string{"failWithOutAndErr"},
			ExpectedResults:  []check.Result{},
		},
	}
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			check.UnRegisterAll()
			for _, checker := range tc.RegisteredCheckers {
				check.Register(checker)
			}
			resultSet := check.NewRunner(
				check.RunnerWithCustomCheckPaths(tc.CustomCheckPaths...),
			).Do()
			for _, expectedResult := range tc.ExpectedResults {
				assert.Containsf(t, resultSet.Data, expectedResult,
					"result: %+v not found in resultSet %+v\n", expectedResult, resultSet.Data)
			}
			assert.ElementsMatchf(t, resultSet.Data, tc.ExpectedResults,
				"ResultSets Data: %+v instead of expected: %+v",
				resultSet.Data, tc.ExpectedResults)
			assert.Equal(t, len(resultSet.Data), len(tc.ExpectedResults))
		})
	}
}
