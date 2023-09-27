package instance

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/core/path"
	"github.com/opensvc/om3/core/status"
)

func Test_Mapper(t *testing.T) {
	mapper := NewData[Status]()
	p, _ := path.Parse("foo")
	p2, _ := path.Parse("bar")
	mapper.Set(p, "node1", &Status{Avail: status.Up})
	mapper.Set(p, "node2", &Status{Avail: status.Warn})
	mapper.Set(p2, "node2", &Status{Avail: status.Down})

	require.Equal(t, status.Up, mapper.Get(p, "node1").Avail)
	require.Equal(t, status.Warn, mapper.Get(p, "node2").Avail)
	require.Equal(t, status.Down, mapper.Get(p2, "node2").Avail)
	require.Nil(t, mapper.Get(p, "node3"))
	require.Len(t, mapper.GetAll(), 3)
	require.Len(t, mapper.GetByPath(p), 2)
	require.Len(t, mapper.GetByNode("node1"), 1)
	require.Len(t, mapper.GetByNode("node2"), 2)

	mapper.Unset(p, "node3")
	mapper.Unset(p, "node2")

	require.Equal(t, status.Up, mapper.Get(p, "node1").Avail)
	require.Nil(t, mapper.Get(p, "node2"))
	require.Nil(t, mapper.Get(p, "node3"))
	require.Len(t, mapper.GetAll(), 2)
	require.Len(t, mapper.GetByPath(p), 1)
	require.Len(t, mapper.GetByNode("node1"), 1)
	require.Len(t, mapper.GetByNode("node2"), 1)
}
