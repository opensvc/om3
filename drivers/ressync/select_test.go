package ressync

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSelectPeernames(t *testing.T) {
	expandNodeSelector = func(s string) ([]string, error) {
		switch s {
		case "n*":
			return []string{"n1", "n2", "n3"}, nil
		case "site=b":
			return []string{"d1", "x9"}, nil
		default:
			return nil, nil
		}
	}
	var d T
	nodes := []string{"n1", "n2"}
	drpNodes := []string{"d1"}
	configured := []string{"nodes", "drpnodes"}

	l, err := d.SelectPeernames(nil, configured, nodes, drpNodes)
	require.NoError(t, err)
	require.Equal(t, []string{"n1", "n2", "d1"}, l, "all the configured peers")

	l, err = d.SelectPeernames([]string{"drpnodes"}, configured, nodes, drpNodes)
	require.NoError(t, err)
	require.Equal(t, []string{"d1"}, l)

	l, err = d.SelectPeernames([]string{"n2"}, configured, nodes, drpNodes)
	require.NoError(t, err)
	require.Equal(t, []string{"n2"}, l, "a peer named")

	l, err = d.SelectPeernames([]string{"n2", "nodes"}, configured, nodes, drpNodes)
	require.NoError(t, err)
	require.Equal(t, []string{"n2", "n1"}, l, "named twice, selected once")

	l, err = d.SelectPeernames([]string{"drpnodes"}, []string{"nodes"}, nodes, drpNodes)
	require.NoError(t, err)
	require.Empty(t, l, "not a target of the configuration")

	l, err = d.SelectPeernames([]string{"n*"}, configured, nodes, drpNodes)
	require.NoError(t, err)
	require.Equal(t, []string{"n1", "n2"}, l, "a glob, n3 being no peer")

	l, err = d.SelectPeernames([]string{"site=b"}, configured, nodes, drpNodes)
	require.NoError(t, err)
	require.Equal(t, []string{"d1"}, l, "a label")

	_, err = d.SelectPeernames([]string{"d1"}, []string{"nodes"}, nodes, drpNodes)
	require.ErrorContains(t, err, "d1 selects no peer")

	_, err = d.SelectPeernames([]string{"x9"}, configured, nodes, drpNodes)
	require.ErrorContains(t, err, "x9 selects no peer")
}
