package sdunit

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParse(t *testing.T) {
	l, err := Parse(`ProtectSystem=strict
		ReadWritePaths=/srv/data /var/log/app
		ProtectHome=yes PrivateTmp=yes
		SystemCallFilter="@system-service ~@privileged"`)
	require.NoError(t, err)
	assert.Equal(t, []Property{
		{"ProtectSystem", "strict"},
		{"ReadWritePaths", "/srv/data /var/log/app"},
		{"ProtectHome", "yes"},
		{"PrivateTmp", "yes"},
		{"SystemCallFilter", "@system-service ~@privileged"},
	}, l)

	l, err = Parse("")
	require.NoError(t, err)
	assert.Empty(t, l)

	_, err = Parse("/srv/data ProtectSystem=strict")
	assert.ErrorContains(t, err, `"/srv/data" is no property`)

	_, err = Parse(`ProtectSystem="strict`)
	assert.ErrorContains(t, err, "unterminated")

	// A lowercase word with = is a value, not a property.
	l, err = Parse("Environment=a=b")
	require.NoError(t, err)
	assert.Equal(t, []Property{{"Environment", "a=b"}}, l)
}

func TestValidate(t *testing.T) {
	assert.NoError(t, Validate("ProtectSystem=strict ReadWritePaths=/srv PrivateTmp=yes RuntimeMaxSec=1h"))
	for _, s := range []string{
		"User=root",
		"ExecStartPre=/bin/true",
		"Type=simple",
		"Slice=other.slice",
		"LimitNOFILE=100",
		"TimeoutStopSec=1",
		"RestartSec=1",
		"Before=multi-user.target",
		"NoNewPrivileges=yes",
		"CapabilityBoundingSet=CAP_CHOWN",
		"KillMode=none",
		"EnvironmentFile=/etc/secret",
		"StandardInputText=x",
	} {
		assert.Errorf(t, Validate("PrivateTmp=yes "+s), "%s is refused", s)
	}
	err := Validate("User=root Group=root")
	assert.ErrorContains(t, err, "User is refused")
	assert.ErrorContains(t, err, "Group is refused")
}

func TestEnvFile(t *testing.T) {
	b, err := EnvFile([]string{
		"A=it's \"q\" \\b $x `c` #h",
		"B=multi\nline",
		"C=  sp  ",
		"D=",
		"NOEQUAL",
	})
	require.NoError(t, err)
	assert.Equal(t, "A=\"it's \\\"q\\\" \\\\b \\$x \\`c\\` #h\"\nB=\"multi\nline\"\nC=\"  sp  \"\nD=\"\"\n", string(b))

	_, err = EnvFile([]string{"1A=x"})
	assert.ErrorContains(t, err, "invalid name")
}

func TestRunArgs(t *testing.T) {
	r := Run{
		Unit:          "opensvc-svc.web-app.main.service",
		Slice:         "opensvc-svc.web-app.main.slice",
		Description:   "svc/web app#main",
		Type:          "notify",
		OwnProperties: []Property{{"Before", ServerUnit}},
		Properties:    []Property{{"ProtectSystem", "strict"}, {"ReadWritePaths", "/a /b"}},
		Argv:          []string{"/usr/bin/om", "exec", "--", "sh", "-c", "echo $HOME"},
	}
	assert.Equal(t, []string{
		"systemd-run",
		"--unit=opensvc-svc.web-app.main.service",
		"--quiet",
		"--slice=opensvc-svc.web-app.main.slice",
		"--description=svc/web app#main",
		"--property=Type=notify",
		"--property=Before=opensvc-server.service",
		"--property=ProtectSystem=strict",
		"--property=ReadWritePaths=/a /b",
		"--",
		"/usr/bin/om", "exec", "--", "sh", "-c", "echo $$HOME",
	}, r.Args())

	r.Wait = true
	assert.Contains(t, r.Args(), "--wait")
	assert.Contains(t, r.Args(), "--pipe")
}

func TestStateIsActive(t *testing.T) {
	for s, want := range map[string]bool{
		"active":       true,
		"activating":   true,
		"deactivating": true,
		"inactive":     false,
		"failed":       false,
		"":             false,
	} {
		assert.Equal(t, want, State{ActiveState: s}.IsActive(), s)
	}
}
