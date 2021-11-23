package object

import (
	"testing"

	"github.com/stretchr/testify/require"

	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/core/rawconfig"
)

func TestConfigFile(t *testing.T) {
	tests := map[string]struct {
		name      string
		namespace string
		kind      string
		cf        string
		root      string
	}{
		"namespaced, package install": {
			name:      "svc1",
			namespace: "ns1",
			kind:      "svc",
			cf:        "/etc/opensvc/namespaces/ns1/svc/svc1.conf",
			root:      "",
		},
		"rooted svc, package install": {
			name:      "svc1",
			namespace: "",
			kind:      "svc",
			cf:        "/etc/opensvc/svc1.conf",
			root:      "",
		},
		"rooted cfg, package install": {
			name:      "cfg1",
			namespace: "",
			kind:      "cfg",
			cf:        "/etc/opensvc/cfg/cfg1.conf",
			root:      "",
		},
		"cluster cfg, package install": {
			name:      "cluster",
			namespace: "",
			kind:      "ccfg",
			cf:        "/etc/opensvc/cluster.conf",
			root:      "",
		},
		"namespaced, dev install": {
			name:      "svc1",
			namespace: "ns1",
			kind:      "svc",
			cf:        "/opt/opensvc/etc/namespaces/ns1/svc/svc1.conf",
			root:      "/opt/opensvc",
		},
		"rooted svc, dev install": {
			name:      "svc1",
			namespace: "",
			kind:      "svc",
			cf:        "/opt/opensvc/etc/svc1.conf",
			root:      "/opt/opensvc",
		},
		"rooted cfg, dev install": {
			name:      "cfg1",
			namespace: "",
			kind:      "cfg",
			cf:        "/opt/opensvc/etc/cfg/cfg1.conf",
			root:      "/opt/opensvc",
		},
		"cluster cfg, dev install": {
			name:      "cluster",
			namespace: "",
			kind:      "ccfg",
			cf:        "/opt/opensvc/etc/cluster.conf",
			root:      "/opt/opensvc",
		},
	}
	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			rawconfig.Load(map[string]string{
				"osvc_root_path": test.root,
			})
			p, _ := path.New(test.name, test.namespace, test.kind)
			o, err := NewFromPath(p)
			require.Nil(t, err, "NewFromPath(p) mustn't return an error")
			require.Equal(t, test.cf, o.(Configurer).ConfigFile())
		})
	}
}
