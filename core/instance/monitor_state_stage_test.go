package instance

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMonitorStateResizeStage(t *testing.T) {
	for stage, expected := range map[int]string{
		0: "resized:0",
		1: "resized:1",
		2: "resized:2",
	} {
		v, ok := NewMonitorStateResizeStage(stage)
		require.True(t, ok)
		assert.Equal(t, expected, v.String())

		got, ok := v.ResizeStage()
		require.True(t, ok)
		assert.Equal(t, stage, got)

		b, err := json.Marshal(v)
		require.NoError(t, err)
		assert.JSONEq(t, `"`+expected+`"`, string(b))

		var back MonitorState
		require.NoError(t, json.Unmarshal(b, &back))
		assert.Equal(t, v, back)
	}
}

func TestMonitorStateResizeStageStopsAtTheMax(t *testing.T) {
	_, ok := NewMonitorStateResizeStage(MaxResizeStages)
	assert.False(t, ok)
	_, ok = NewMonitorStateResizeStage(-1)
	assert.False(t, ok)
}

func TestMonitorStateNamedAreNotStages(t *testing.T) {
	for _, v := range []MonitorState{MonitorStateIdle, MonitorStateResizeSuccess, MonitorStateResizeFailure, MonitorStateWaitNonLeader} {
		_, ok := v.ResizeStage()
		assert.False(t, ok, v.String())
	}
}
