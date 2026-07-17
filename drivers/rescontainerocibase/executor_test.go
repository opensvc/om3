package rescontainerocibase

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExecutorDoExecRunLogsStreamsStdoutAndStderr(t *testing.T) {
	t.Setenv("OSVC_CONTAINER_LOGS_HELPER", "1")

	executor := &Executor{bin: os.Args[0]}
	logChan, err := executor.doExecRunLogs(
		context.Background(),
		"-test.run=TestExecutorDoExecRunLogsHelperProcess$",
	)
	require.NoError(t, err)

	var output strings.Builder
	for data := range logChan {
		output.Write(data)
	}

	require.Contains(t, output.String(), "log from stdout")
	require.Contains(t, output.String(), "log from stderr")
}

func TestExecutorDoExecRunLogsHelperProcess(t *testing.T) {
	if os.Getenv("OSVC_CONTAINER_LOGS_HELPER") != "1" {
		return
	}

	_, _ = fmt.Fprintln(os.Stdout, "log from stdout")
	_, _ = fmt.Fprintln(os.Stderr, "log from stderr")
	os.Exit(0)
}
