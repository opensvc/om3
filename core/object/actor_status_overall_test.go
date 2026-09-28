package object

import (
	"testing"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/status"
)

func TestOverallContribution(t *testing.T) {
	cases := []struct {
		name      string
		resources []struct {
			group driver.Group
			s     status.T
		}
		want status.T
	}{
		{
			name: "secondary receiving the replicas",
			resources: []struct {
				group driver.Group
				s     status.T
			}{{driver.GroupFS, status.Down}, {driver.GroupSync, status.Up}},
			want: status.Down,
		},
		{
			name: "primary replicating",
			resources: []struct {
				group driver.Group
				s     status.T
			}{{driver.GroupFS, status.Up}, {driver.GroupSync, status.Up}},
			want: status.Up,
		},
		{
			name: "primary with a sync down",
			resources: []struct {
				group driver.Group
				s     status.T
			}{{driver.GroupFS, status.Up}, {driver.GroupSync, status.Down}},
			want: status.Warn,
		},
		{
			name: "secondary with a sync down",
			resources: []struct {
				group driver.Group
				s     status.T
			}{{driver.GroupFS, status.Down}, {driver.GroupSync, status.Down}},
			want: status.Warn,
		},
		{
			name: "sync only, up",
			resources: []struct {
				group driver.Group
				s     status.T
			}{{driver.GroupSync, status.Up}},
			want: status.NotApplicable,
		},
		{
			name: "other groups are unchanged",
			resources: []struct {
				group driver.Group
				s     status.T
			}{{driver.GroupFS, status.Down}, {driver.GroupApp, status.Up}},
			want: status.Warn,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := status.Undef
			for _, r := range c.resources {
				got.Add(overallContribution(r.group, r.s))
			}
			if got == status.Undef {
				got = status.NotApplicable
			}
			if got != c.want {
				t.Errorf("got %s, want %s", got, c.want)
			}
		})
	}
}
