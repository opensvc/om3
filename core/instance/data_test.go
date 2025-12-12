package instance

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/status"
)

func Test_Mapper(t *testing.T) {
	mapper := NewData[Status]()
	p, _ := naming.ParsePath("foo")
	p2, _ := naming.ParsePath("bar")
	mapper.Set(p, "node1", &Status{Avail: status.Up})
	mapper.Set(p, "node2", &Status{Avail: status.Warn})
	mapper.Set(p2, "node2", &Status{Avail: status.Down})

	require.Equal(t, status.Up, mapper.GetByPathAndNode(p, "node1").Avail)
	require.Equal(t, status.Warn, mapper.GetByPathAndNode(p, "node2").Avail)
	require.Equal(t, status.Down, mapper.GetByPathAndNode(p2, "node2").Avail)
	require.Nil(t, mapper.GetByPathAndNode(p, "node3"))
	require.Len(t, mapper.GetAll(), 3)
	require.Len(t, mapper.GetByPath(p), 2)
	require.Len(t, mapper.GetByNode("node1"), 1)
	require.Len(t, mapper.GetByNode("node2"), 2)

	mapper.Unset(p, "node3")
	mapper.Unset(p, "node2")

	require.Equal(t, status.Up, mapper.GetByPathAndNode(p, "node1").Avail)
	require.Nil(t, mapper.GetByPathAndNode(p, "node2"))
	require.Nil(t, mapper.GetByPathAndNode(p, "node3"))
	require.Len(t, mapper.GetAll(), 2)
	require.Len(t, mapper.GetByPath(p), 1)
	require.Len(t, mapper.GetByNode("node1"), 1)
	require.Len(t, mapper.GetByNode("node2"), 1)
}
