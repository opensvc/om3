package resapp

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/core/resource"
	"github.com/opensvc/om3/testhelper"
	"github.com/opensvc/om3/util/executable"
	"github.com/opensvc/om3/util/pg"
)

func TestT_Info(t *testing.T) {
	testhelper.SetExecutable(t, "../..")
	defer executable.Unset()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	t.Run("from custom app", func(t *testing.T) {
		duration := 1 * time.Hour
		startDuration := 3*time.Second + 3*time.Millisecond
		stopDuration := 6 * time.Second
		checkDuration := 2*time.Second + 3*time.Millisecond
		infoDuration := 2*time.Second + 1*time.Millisecond
		app := T{
			StartCmd:     "startCmd",
			StopCmd:      "stopCmd",
			CheckCmd:     "checkCmd",
			InfoCmd:      "echo foo:Foo && echo notAnInfo && echo fooBar:FOOBAR",
			CheckTimeout: &checkDuration,
			InfoTimeout:  &infoDuration,
			ScriptPath:   "scriptPath",
		}
		app.SetRID("app#1")
		app.SetPG(&pg.Config{})
		app.Timeout = &duration
		app.StartTimeout = &startDuration
		app.StopTimeout = &stopDuration

		info, err := app.Info(ctx)
		assert.Nil(t, err)
		expected := []resource.InfoKey{
			{Key: "script", Value: "scriptPath"},
			{Key: "start", Value: "startCmd"},
			{Key: "stop", Value: "stopCmd"},
			{Key: "check", Value: "checkCmd"},
			{Key: "info", Value: "echo foo:Foo && echo notAnInfo && echo fooBar:FOOBAR"},
			{Key: "timeout", Value: "1h0m0s"},
			{Key: "start_timeout", Value: "3.003s"},
			{Key: "stop_timeout", Value: "6s"},
			{Key: "check_timeout", Value: "2.003s"},
			{Key: "info_timeout", Value: "2.001s"},
		}
		for _, entry := range expected {
			t.Run(entry.Key+" "+entry.String(), func(t *testing.T) {
				assert.Contains(t, info, entry)
			})
		}
		t.Run("has info from info output", func(t *testing.T) {
			assert.Contains(t, info, resource.InfoKey{Key: "foo", Value: "Foo"})
			assert.Contains(t, info, resource.InfoKey{Key: "fooBar", Value: "FOOBAR"})
		})
	})

	t.Run("from zero app", func(t *testing.T) {
		app := T{}
		app.SetRID("app#1")
		app.SetPG(&pg.Config{})
		info, err := app.Info(ctx)
		assert.Nil(t, err)
		expected := []resource.InfoKey{
			{Key: "script", Value: ""},
			{Key: "start", Value: ""},
			{Key: "stop", Value: ""},
			{Key: "check", Value: ""},
			{Key: "info", Value: ""},
			{Key: "timeout", Value: ""},
			{Key: "start_timeout", Value: ""},
			{Key: "stop_timeout", Value: ""},
			{Key: "check_timeout", Value: ""},
			{Key: "info_timeout", Value: ""},
		}
		for _, entry := range expected {
			t.Run("default value empty "+entry.Key, func(t *testing.T) {
				assert.Contains(t, info, entry)
			})
		}
	})
}
