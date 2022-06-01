package core_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"opensvc.com/opensvc/core/instance"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/core/resource"
	"opensvc.com/opensvc/util/timestamp"
)

func TestInstanceStatus(t *testing.T) {
	t.Run("DeepCopy", func(t *testing.T) {
		a := instance.Status{
			Nodename: "a",
			Path:     path.T{Name: "a"},
			Monitor: instance.Monitor{
				GlobalExpect:  "globalExpectA",
				StatusUpdated: timestamp.Now(),
			},
			Updated: timestamp.Now(),
			Resources: map[string]resource.ExposedStatus{
				"fooA": resource.ExposedStatus{
					Label: "labelA",
				},
			},
			Parents: []path.Relation{
				"a1",
				"a2",
			},
		}
		b := *a.DeepCopy()

		b.Nodename = "b"
		require.Equal(t, "a", a.Nodename)
		require.Equal(t, "b", b.Nodename)

		b.Updated = timestamp.Now()
		require.True(t, b.Updated.Time().After(a.Updated.Time()))

		b.Path.Name = "b"
		require.Equal(t, "a", a.Path.Name)
		require.Equal(t, "b", b.Path.Name)

		b.Parents = []path.Relation{"b1"}
		require.Equal(t, []path.Relation{"a1", "a2"}, a.Parents)
		require.Equal(t, []path.Relation{"b1"}, b.Parents)

		if e, ok := b.Resources["fooA"]; ok {
			e.Label = "labelB"
			b.Resources["fooA"] = e
		}
		require.Equal(t, "labelA", a.Resources["fooA"].Label)
		require.Equal(t, "labelB", b.Resources["fooA"].Label)
	})
}
