//go:build linux

package network

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/util/plog"
)

type testLogger struct{}

func (testLogger) Log() *plog.Logger { return plog.NewDefaultLogger() }

// fakeIPTables keeps the chains of the filter table of one family, as the
// iptables commands would.
type fakeIPTables struct {
	chains map[string][]string
	calls  []string
}

func (f *fakeIPTables) run(name, stdin string, args ...string) ([]byte, error) {
	f.calls = append(f.calls, name+" "+strings.Join(args, " "))
	if strings.HasSuffix(name, "-restore") {
		for _, line := range strings.Split(stdin, "\n") {
			switch {
			case strings.HasPrefix(line, ":"):
				f.chains[strings.Fields(line[1:])[0]] = []string{}
			case strings.HasPrefix(line, "-A "):
				fields := strings.Fields(line)
				f.chains[fields[1]] = append(f.chains[fields[1]], strings.Join(fields[2:], " "))
			}
		}
		return nil, nil
	}
	args = args[1:] // -w
	chain := args[1]
	rules, ok := f.chains[chain]
	if !ok {
		return nil, errors.New("no chain")
	}
	rule := strings.Join(args[2:], " ")
	switch args[0] {
	case "-S":
	case "-C":
		for _, r := range rules {
			if r == rule {
				return nil, nil
			}
		}
		return nil, errors.New("no rule")
	case "-I":
		f.chains[chain] = append([]string{strings.Join(args[3:], " ")}, rules...)
	case "-D":
		for i, r := range rules {
			if r == rule {
				f.chains[chain] = append(rules[:i:i], rules[i+1:]...)
				return nil, nil
			}
		}
		return nil, errors.New("no rule")
	case "-F":
		f.chains[chain] = []string{}
	case "-X":
		delete(f.chains, chain)
	}
	return nil, nil
}

func withFakeIPTables(t *testing.T, f *fakeIPTables) {
	run, has := runIPTables, hasIPTables
	runIPTables = f.run
	hasIPTables = func(name string) bool { return name == "ip6tables" }
	t.Cleanup(func() { runIPTables, hasIPTables = run, has })
}

var dockerTestNetworks = []fwNetwork{
	{CIDR: "fd01::/64", Dev: "obr_backend5"},
	{CIDR: "10.100.0.0/22", Dev: "obr_backend3"},
	{CIDR: "fd02::/64"},
}

// On a node running docker, DOCKER-USER jumps to a chain accepting the
// traffic of the devices of the om networks of the family, once whatever the
// number of setups, before the return docker leaves there.
func TestSetupDockerFW(t *testing.T) {
	f := &fakeIPTables{chains: map[string][]string{fwDockerUserChain: {"-j RETURN"}}}
	withFakeIPTables(t, f)
	for i := 0; i < 2; i++ {
		require.NoError(t, setupDockerFW(testLogger{}, dockerTestNetworks))
	}
	assert.Equal(t, []string{"-j " + fwDockerChain, "-j RETURN"}, f.chains[fwDockerUserChain])
	assert.Equal(t, []string{"-i obr_backend5 -j ACCEPT", "-o obr_backend5 -j ACCEPT"}, f.chains[fwDockerChain])

	// No network of the family left: what om added is removed.
	require.NoError(t, setupDockerFW(testLogger{}, dockerTestNetworks[1:]))
	assert.Equal(t, []string{"-j RETURN"}, f.chains[fwDockerUserChain])
	assert.NotContains(t, f.chains, fwDockerChain)
}

// A node not running docker is left alone, and the chain om added while it
// ran it is removed.
func TestSetupDockerFWWithoutDocker(t *testing.T) {
	f := &fakeIPTables{chains: map[string][]string{}}
	withFakeIPTables(t, f)
	require.NoError(t, setupDockerFW(testLogger{}, dockerTestNetworks))
	assert.Empty(t, f.chains)
	for _, call := range f.calls {
		assert.Contains(t, call, " -S ", "only reads")
	}

	f.chains[fwDockerChain] = []string{"-i obr_backend5 -j ACCEPT"}
	require.NoError(t, setupDockerFW(testLogger{}, dockerTestNetworks))
	assert.Empty(t, f.chains)
}

func TestFWDockerRestore(t *testing.T) {
	assert.Equal(t, "*filter\n:OSVC-FORWARD - [0:0]\n-A OSVC-FORWARD -i br0 -j ACCEPT\n-A OSVC-FORWARD -o br0 -j ACCEPT\nCOMMIT\n", fwDockerRestore([]string{"br0"}))
}
