package resapp

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"opensvc.com/opensvc/util/pg"
)

func TestT_Info(t *testing.T) {
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
		expected := []infoEntry{
			{"script", "scriptPath"},
			{"start", "startCmd"},
			{"stop", "stopCmd"},
			{"check", "checkCmd"},
			{"info", "echo foo:Foo && echo notAnInfo && echo fooBar:FOOBAR"},
			{"timeout", "1h0m0s"},
			{"start_timeout", "3.003s"},
			{"stop_timeout", "6s"},
			{"check_timeout", "2.003s"},
			{"info_timeout", "2.001s"},
		}
		for _, entry := range expected {
			t.Run(entry[0]+" "+entry[1], func(t *testing.T) {
				assert.Contains(t, info, entry)
			})
		}
		t.Run("has info from info output", func(t *testing.T) {
			assert.Contains(t, info, infoEntry{"foo", "Foo"})
			assert.Contains(t, info, infoEntry{"fooBar", "FOOBAR"})
		})
	})

	t.Run("from zero app", func(t *testing.T) {
		app := T{}
		app.SetRID("app#1")
		app.SetPG(&pg.Config{})
		info, err := app.Info(ctx)
		assert.Nil(t, err)
		expected := []infoEntry{
			{"script", ""},
			{"start", ""},
			{"stop", ""},
			{"check", ""},
			{"info", ""},
			{"timeout", ""},
			{"start_timeout", ""},
			{"stop_timeout", ""},
			{"check_timeout", ""},
			{"info_timeout", ""},
		}
		for _, entry := range expected {
			t.Run("default value empty "+entry[0], func(t *testing.T) {
				assert.Contains(t, info, entry)
			})
		}
	})
}
