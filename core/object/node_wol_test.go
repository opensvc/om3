package object

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/testhelper"
)

func TestNodeWol(t *testing.T) {
	testhelper.Setup(t)

	var sent []string
	saved := wolSend
	wolSend = func(mac, broadcast string, port int) error {
		sent = append(sent, fmt.Sprintf("%s %s %d", mac, broadcast, port))
		if mac == "52:54:00:00:00:02" && port == 7 {
			return errors.New("send failed")
		}
		return nil
	}
	t.Cleanup(func() { wolSend = saved })

	n, err := NewNode()
	require.NoError(t, err)

	t.Run("sends broadcast x mac x port, default port 9", func(t *testing.T) {
		sent = nil
		require.NoError(t, n.Wol([]string{"52:54:00:00:00:01"}, []string{"10.0.0.255", "10.0.1.255"}, nil))
		assert.Equal(t, []string{
			"52:54:00:00:00:01 10.0.0.255 9",
			"52:54:00:00:00:01 10.0.1.255 9",
		}, sent)
	})

	t.Run("a send error does not stop the other sends", func(t *testing.T) {
		sent = nil
		err := n.Wol([]string{"52:54:00:00:00:01", "52:54:00:00:00:02"}, []string{"10.0.0.255"}, []int{7, 9})
		assert.ErrorContains(t, err, "send failed")
		assert.Len(t, sent, 4)
	})

	t.Run("invalid arguments abort before any send", func(t *testing.T) {
		cases := map[string]struct {
			macs       []string
			broadcasts []string
			ports      []int
		}{
			"no mac":            {nil, []string{"10.0.0.255"}, nil},
			"no broadcast":      {[]string{"52:54:00:00:00:01"}, nil, nil},
			"invalid mac":       {[]string{"52:54:00:00:00:01", "bad"}, []string{"10.0.0.255"}, nil},
			"invalid broadcast": {[]string{"52:54:00:00:00:01"}, []string{"10.0.0.256"}, nil},
			"invalid port":      {[]string{"52:54:00:00:00:01"}, []string{"10.0.0.255"}, []int{9, 70000}},
		}
		for name, c := range cases {
			sent = nil
			assert.Error(t, n.Wol(c.macs, c.broadcasts, c.ports), name)
			assert.Empty(t, sent, name)
		}
	})
}
