package resource

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// The holds are published as their values by class code, and read back as
// the same holds, sorted by class and value.
func TestHoldsJSON(t *testing.T) {
	holds := Holds{
		{Class: HoldMountPoint, Value: "/srv/app"},
		{Class: HoldBlockDevice, Value: "/dev/sdc"},
		{Class: HoldBlockDevice, Value: "/dev/sdb"},
		{Class: "future class", Value: "x"},
	}
	b, err := json.Marshal(holds)
	require.NoError(t, err)
	require.JSONEq(t, `{"bdev":["/dev/sdc","/dev/sdb"],"future class":["x"],"mnt":["/srv/app"]}`, string(b))

	var got Holds
	require.NoError(t, json.Unmarshal(b, &got))
	require.Equal(t, Holds{
		{Class: HoldBlockDevice, Value: "/dev/sdb"},
		{Class: HoldBlockDevice, Value: "/dev/sdc"},
		{Class: "future class", Value: "x"},
		{Class: HoldMountPoint, Value: "/srv/app"},
	}, got)

	// A resource holding nothing publishes no holds.
	b, err = json.Marshal(struct {
		Holds Holds `json:"holds,omitempty"`
	}{})
	require.NoError(t, err)
	require.Equal(t, `{}`, string(b))
}

// Every class has a code, and no two classes share one.
func TestHoldClassCodes(t *testing.T) {
	require.Len(t, holdClassesByCode, len(holdClassCodes))
}
