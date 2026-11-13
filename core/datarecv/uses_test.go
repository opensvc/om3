package datarecv

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// The datastores a section reads keys of are said by its configuration
// alone: the install lines, a pattern as a pattern, the environment
// references, and the deprecated keywords naming a datastore.
func TestSectionDatastores(t *testing.T) {
	values := map[string][]string{
		"install": {
			"/etc/", "mode", "0750",
			"/etc/haproxy.cfg", "from", "./cfg/haproxy", "key", "haproxy.cfg",
			"/etc/ssl/certs/*.pem", "from", "./sec/web-*", "key", "fullpem",
			"/etc/ca.pem", "from", "other/sec/ca", "key", "ca.pem",
			"/web/index.html", "required", "source", "https://example.com/index.html",
		},
		"configs":             {"d/*:/cfg", "cfg/e/k:/e"},
		"secrets":             {"d/*:/sec", "usr/u/k:/u"},
		"environment":         {"SLIP=slob", "DB_PASSWORD", "from", "./sec/db", "key", "password"},
		"configs_environment": {"PORT=http/port", "webapp/app1*"},
		"secrets_environment": {"CRT=cert1/server.pem"},
	}
	get := func(option string) []string { return values[option] }
	assert.Equal(t, []string{
		"ns1/cfg/d",
		"ns1/cfg/e",
		"ns1/cfg/haproxy",
		"ns1/cfg/http",
		"ns1/cfg/webapp",
		"ns1/sec/cert1",
		"ns1/sec/d",
		"ns1/sec/db",
		"ns1/sec/web-*",
		"ns1/usr/u",
		"other/sec/ca",
	}, SectionDatastores(get, "ns1"))

	assert.Empty(t, SectionDatastores(func(string) []string { return nil }, "ns1"), "a section reading no key")
}
