package check_test

import (
	"opensvc.com/opensvc/core/check"

	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/assert"
	"os"
	"os/exec"
	"strings"
	"testing"
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
	case cmd == "exitCode3":
		exitCode = 3
	case strings.Contains(cmd, "succeedWithOut"):
		data := check.ResultSet{
			Data: []check.Result{
				{"group1", cmd, "path/" + cmd, "1", "count", 2},
			},
		}
		outB, err := json.Marshal(data)
		if err != nil {
			t.Fatal(err)
		}
		out = string(outB)
	case cmd == "failWithCorrectOut":
		data := check.ResultSet{Data: []check.Result{
			{"group1", cmd, "path/" + cmd, "1", "count", 2}},
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
		_, _ = fmt.Fprintf(os.Stdout, out)
	}
	if err != "" {
		_, _ = fmt.Fprintf(os.Stderr, err)
	}
	return
}

func TestRunnerDo(t *testing.T) {
	check.ExecCommand = fakeExecCommand
	defer func() { check.ExecCommand = exec.Command }()
	cases := []struct {
		Name             string
		CustomCheckPaths []string
		ExpectedResults  []check.Result
	}{
		{
			"withoutCustomCheckers",
			[]string{},
			[]check.Result{},
		},
		{
			"withOneFailedChecker",
			[]string{"exitCode3"},
			[]check.Result{},
		},
		{
			"withOneSucceedCustomCheckers",
			[]string{"succeedWithOut"},
			[]check.Result{
				{
					"group1",
					"succeedWithOut",
					"path/succeedWithOut",
					"1",
					"count",
					int64(2),
				},
			},
		},
		{
			"withSomeSucceedCustomCheckers",
			[]string{"succeedWithOut1", "succeedWithOut2"},
			[]check.Result{
				{
					"group1",
					"succeedWithOut1",
					"path/succeedWithOut1",
					"1",
					"count",
					int64(2),
				},
				{
					"group1",
					"succeedWithOut2",
					"path/succeedWithOut2",
					"1",
					"count",
					int64(2),
				},
			},
		},
		{
			"withSomeFailedCustomCheckers",
			[]string{"succeedWithOut", "exitCode3"},
			[]check.Result{
				{
					"group1",
					"succeedWithOut",
					"path/succeedWithOut",
					"1",
					"count",
					int64(2),
				},
			},
		},
		{
			"withWithCorrectOutputButBadExitCode",
			[]string{"failWithCorrectOut"},
			[]check.Result{},
		},
		{
			"withFailedCustomCheckers",
			[]string{"failWithOutAndErr"},
			[]check.Result{},
		},
	}
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			resultSet := check.NewRunner(tc.CustomCheckPaths).Do()
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
