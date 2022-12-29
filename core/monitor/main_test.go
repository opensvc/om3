package monitor

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"opensvc.com/opensvc/core/mock_monitor"
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

var expected = "Threads                 \n" +
	" daemon    running      \n" +
	" dns                    \n" +
	" collector              \n" +
	" hb                    \n" +
	" listener          :0   \n" +
	" monitor                \n" +
	" scheduler              \n" +
	"                        \n" +
	"Nodes                   \n" +
	" score                  \n" +
	"  load15m               \n" +
	"  mem                   \n" +
	"  swap                  \n" +
	"  version  warn         \n" +
	"  compat   warn         \n" +
	" hb-q                  \n" +
	" state                  \n" +
	"                        \n" +
	"Objects                 \n"

var daemonResultString = "{\"monitor\": {\"nodes\": {}, \"services\": {}}}"

func TestMonitorOutputIsCorrect(t *testing.T) {
	m := New()
	m.SetColor("no")
	spy := recorder{}

	c := &mockDaemonStatus{daemonResultString}
	m.Do(c, &spy)

	assert.Equal(t, expected, string(spy.data), "they should be equal")
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

	assert.Equal(t, expected, string(spy.data), "they should be equal")
}
