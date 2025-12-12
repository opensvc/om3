package monitor

import (
	"os"
	"path"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/mock_monitor"
	"github.com/opensvc/om3/v3/util/hostname"
)

type recorder struct {
	data []byte
}

func (r *recorder) Write(p []byte) (n int, err error) {
	r.data = append(r.data, p...)
	return len(p), nil
}

type mockDaemonStatus struct {
	value string
}

func (c *mockDaemonStatus) Get() ([]byte, error) {
	return []byte(c.value), nil
}

var daemonResultString = `{"monitor": {"nodes": {}, "objects": {}}}`

func TestMonitorOutputIsCorrect(t *testing.T) {
	m := New()
	m.SetColor("no")
	spy := recorder{}

	c := &mockDaemonStatus{daemonResultString}
	m.Do(c, &spy)

	expected, err := os.ReadFile(path.Join("testdata", "empty-om-mon.fixture"))
	require.Nil(t, err)

	assert.Equal(t, string(expected), string(spy.data), "they should be equal")
}

func TestMonitorOutputIsCorrectWithGoMock(t *testing.T) {
	ctrl := gomock.NewController(t)
	daemonStatusGetter := mock_monitor.NewMockGetter(ctrl)

	daemonStatusGetter.EXPECT().
		Get().
		Return([]byte(daemonResultString), nil)

	m := New()
	m.SetColor("no")
	spy := recorder{}

	m.Do(daemonStatusGetter, &spy)

	expected, err := os.ReadFile(path.Join("testdata", "empty-om-mon.fixture"))
	require.Nil(t, err)

	assert.Equal(t, string(expected), string(spy.data), "they should be equal")
}

func TestMonitorOutput(t *testing.T) {
	for _, s := range []string{
		"single-node",
		"multi-node",
	} {
		t.Run(s, func(t *testing.T) {
			hostname.SetHostnameForGoTest("node3")
			defer hostname.SetHostnameForGoTest("")
			b, err := os.ReadFile(path.Join("testdata", s+"-daemon-status.json"))
			require.Nil(t, err)
			expected, err := os.ReadFile(path.Join("testdata", s+"-om-mon.fixture"))
			require.Nil(t, err)
			ctrl := gomock.NewController(t)

			now = func() time.Time {
				return time.Date(2025, 11, 21, 15, 0, 0, 0, time.UTC)
			}

			daemonStatusGetter := mock_monitor.NewMockGetter(ctrl)

			daemonStatusGetter.EXPECT().
				Get().
				Return(b, nil)

			m := New()
			m.SetColor("no")
			spy := recorder{}

			m.Do(daemonStatusGetter, &spy)

			assert.Equalf(t, string(expected), string(spy.data), "they should be equal:\nexpected:\n%s\nfound:\n%s", expected, spy.data)

		})
	}
}
