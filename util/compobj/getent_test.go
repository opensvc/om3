package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A name is looked up as a name, not read as an option, and one the
// database does not hold is told apart from a failure.
func TestGetentID(t *testing.T) {
	uid, err := getentID("passwd", "root")
	require.NoError(t, err)
	assert.Equal(t, 0, uid)
	gid, err := getentID("group", "root")
	require.NoError(t, err)
	assert.Equal(t, 0, gid)
	_, err = getentID("passwd", "nosuchuser-om3-test")
	assert.ErrorIs(t, err, errGetentNotFound)
	_, err = getentID("passwd", "-s")
	assert.ErrorIs(t, err, errGetentNotFound, "a name starting with '-' is a name")
}

// A user or group name is refused when the account tools would read it as an
// option, past the '-' marking a name to delete where a rule allows it.
func TestCheckAccountName(t *testing.T) {
	assert.NoError(t, checkAccountName("bob", true))
	assert.NoError(t, checkAccountName("-bob", true))
	assert.Error(t, checkAccountName("--bob", true))
	assert.Error(t, checkAccountName("-", true))
	assert.Error(t, checkAccountName("-bob", false))
	assert.Error(t, checkAccountName("", false))
}

// The password hash reaches chpasswd on its stdin, its '$' unexpanded.
func TestPasswordHashIsWrittenToChpasswd(t *testing.T) {
	cmd := execPasswordHashCommand("bob", "$6$salt$hash")
	assert.Equal(t, []string{"chpasswd", "-e"}, cmd.Args)
	b := new(strings.Builder)
	_, err := cmd.Stdin.(*strings.Reader).WriteTo(b)
	require.NoError(t, err)
	assert.Equal(t, "bob:$6$salt$hash\n", b.String())
}

// A sysctl key with a '=' would set the key on a check, and one starting with
// a '-' is an option.
func TestASysctlKeyIsAKeyOnly(t *testing.T) {
	for _, key := range []string{"vm.swappiness=0", "-w", "vm.swappiness 0"} {
		obj := CompSysctls{Obj: &Obj{rules: make([]interface{}, 0), verbose: true}}
		assert.Error(t, obj.Add(`[{"key": "`+key+`", "index": 0, "op": "=", "value": 1}]`), key)
	}
	obj := CompSysctls{Obj: &Obj{rules: make([]interface{}, 0), verbose: true}}
	assert.NoError(t, obj.Add(`[{"key": "vm.swappiness", "index": 0, "op": "=", "value": 1}]`))
}

// A ref is fetched from a verified server, and an error status is no
// content.
func TestGetNormalFileRefusesAnErrorAndAnUnverifiedServer(t *testing.T) {
	notFound := httptest.NewServer(http.NotFoundHandler())
	defer notFound.Close()
	_, err := getNormalFile(notFound.URL + "/x")
	assert.Error(t, err)

	tlsServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("content"))
	}))
	defer tlsServer.Close()
	_, err = getNormalFile(tlsServer.URL)
	assert.Error(t, err, "a server whose certificate is not trusted")

	ok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("content"))
	}))
	defer ok.Close()
	b, err := getNormalFile(ok.URL)
	require.NoError(t, err)
	assert.Equal(t, "content", string(b))
}
