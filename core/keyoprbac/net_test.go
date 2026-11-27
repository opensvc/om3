package keyoprbac

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/daemon/rbac"
)

// TestNetKeywordsAreTheNamespaceAdministrators pins that net_rules is
// set by a user holding no root grant on the app, task and container
// resources, whatever the driver of the section: a filter narrows
// what the resource reaches, and takes nothing of the node.
func TestNetKeywordsAreTheNamespaceAdministrators(t *testing.T) {
	for _, tc := range []struct {
		section string
		typ     string
	}{
		{"app#1", "forking"},
		{"app#1", "simple"},
		{"task#1", "host"},
		{"task#1", "docker"},
		{"task#1", "podman"},
		{"task#1", "oci"},
		{"container#1", "docker"},
		{"container#1", "podman"},
		{"container#1", "oci"},
		{"container#1", "lxc"},
	} {
		for _, option := range []string{"net_rules", "net_rules@node1", "net_rules@nodes"} {
			for _, value := range []string{"allow node n2", "deny any", "allow address 10.0.0.0/8\ndeny network backend node n1", "deny cluster\nallow any"} {
				assert.NoErrorf(t, Denied(noGrant, naming.KindSvc, tc.section, option, value, none), "%s.%s=%s", tc.section, option, value)
				assert.NoErrorf(t, DeniedByDriver(noGrant, naming.KindSvc, tc.section, option, value, tc.typ, none), "%s.%s=%s of a %s", tc.section, option, value, tc.typ)
				assert.NoErrorf(t, DeniedUnset(noGrant, naming.KindSvc, tc.section, option, value, none), "unset %s.%s=%s", tc.section, option, value)
			}
		}
		for _, option := range []string{"net_rules"} {
			assert.Equalf(t, "A rule naming an object of another namespace than the one of the object, or another namespace, requires the guest, operator or admin grant on that namespace, or the root grant, a rule reading its addresses from a datastore of another namespace than the one of the object requires the datastore to share its keys with the namespace of the object, by its share keyword, and a rule reading its addresses from a datastore named by a reference that does not evaluate requires the root grant.",
				Doc(naming.KindSvc, tc.section, option), "%s.%s documents the one thing asking for a grant", tc.section, option)
		}
	}
}

// TestNetKeywordsOpenNothingElse pins that opening the keyword of an
// app, a group the policy refused whole, leaves its other keywords, and the
// sections of the drivers a user may not choose, root's.
func TestNetKeywordsOpenNothingElse(t *testing.T) {
	for _, option := range []string{"type", "start", "stop", "check", "script", "user", "systemd_properties", "environment", "a_keyword_of_a_driver_added_later"} {
		assert.EqualErrorf(t, Denied(noGrant, naming.KindSvc, "app#1", option, "x", none), "this driver group requires the root grant", "app#1.%s", option)
	}
	assert.Error(t, DeniedByDriver(noGrant, naming.KindSvc, "app#1", "start", "/bin/true", "forking", none))
	assert.Error(t, DeniedByDriver(noGrant, naming.KindSvc, "task#1", "command", "/bin/true", "host", none),
		"a keyword of a task of the host type is still root's")
	assert.NoError(t, DeniedByDriver(rbac.Grants{rbac.GrantRoot}, naming.KindSvc, "app#1", "start", "/bin/true", "forking", none))

	// The groups the keyword is no keyword of stay as they were, and so
	// do the two keywords it replaces, which are no keyword any more.
	for _, section := range []string{"disk#1", "share#1", "ip#1", "fs#1", "sync#1"} {
		assert.Errorf(t, Denied(noGrant, naming.KindSvc, section, "net_rules", "allow any", none), "%s.net_rules", section)
	}
	for _, option := range []string{"net_allow", "net_deny"} {
		assert.Errorf(t, Denied(noGrant, naming.KindSvc, "app#1", option, "any", none), "app#1.%s", option)
		assert.Errorf(t, DeniedByDriver(noGrant, naming.KindSvc, "container#1", option, "any", "kvm", none), "%s of a kvm", option)
	}
	// A kvm section answers keyword by keyword: it is open there, and
	// its others as they were.
	for _, option := range []string{"net_rules", "net_rules@n1"} {
		assert.NoError(t, DeniedByDriver(noGrant, naming.KindSvc, "container#1", option, "deny any port 22 proto tcp", "kvm", none), option)
	}
	assert.False(t, deniesKVMKeyword("net_rules", "deny any"))
	assert.False(t, deniesKVMKeyword("net_rules", "deny object prod/svc/db"), "the references are judged by DeniedNetRules")
	for _, option := range []string{"network_filter", "bridge", "rcmd", "virtinst_args"} {
		assert.Error(t, DeniedByDriver(noGrant, naming.KindSvc, "container#1", option, "x", "kvm", none), option)
	}
	// A lxc section is root's to make, and its keywords with it: this
	// one is open there, for the administrator of the namespace to filter a
	// container root gave it, and nothing else is.
	assert.Error(t, Denied(noGrant, naming.KindSvc, "container#1", "type", "lxc", none), "the lxc type")
	for _, option := range []string{"net_rules", "net_rules@n1"} {
		assert.NoError(t, DeniedByDriver(noGrant, naming.KindSvc, "container#1", option, "deny address 10.0.0.0/8", "lxc", none), option)
	}
	for _, option := range []string{"cf", "rootfs", "data_dir", "rcmd", "template"} {
		assert.Error(t, DeniedByDriver(noGrant, naming.KindSvc, "container#1", option, "x", "lxc", none), option)
	}
}

// TestNetRulesReadingADatastoreOfAnotherNamespaceAskForItsSharing pins the
// guard of the rules reading their addresses from a key: a datastore of the
// namespace of the object is its administrator's to name, however it is
// written, and one of another namespace asks for its keys to be shared
// with the namespace of the object, a grant on its namespace being no grant
// to read the keys of its sec, and no grant being asked once it shares
// them. A datastore not known until a reference evaluates asks for the root
// grant.
func TestNetRulesReadingADatastoreOfAnotherNamespaceAskForItsSharing(t *testing.T) {
	root := rbac.Grants{rbac.GrantRoot}
	both := rbac.Grants{rbac.NewGrant(rbac.RoleAdmin, "test"), rbac.NewGrant(rbac.RoleAdmin, "prod")}
	// infra shares its datastores with the test namespace, prod with
	// the dev one only.
	asked := make([]string, 0)
	shared := func(datastore naming.Path, namespace string) bool {
		asked = append(asked, datastore.String()+" "+namespace)
		return datastore.Namespace == "infra" && namespace == "test"
	}
	for _, value := range []string{
		"allow address from ./cfg/acl key allowed",
		"deny address from ./sec/acl key allowed port 22 proto tcp",
		"allow address from test/cfg/acl key allowed",
		"allow address from ./svc/acl key allowed",
		"deny address 10.0.0.1",
	} {
		assert.NoError(t, DeniedNetRules(noGrant, "test", "net_rules", "", value, shared), value)
	}
	assert.Empty(t, asked, "a datastore of the namespace of the object needs no sharing")

	// Shared with the namespace of the object: no grant is asked, on
	// the namespace of the datastore either.
	for _, value := range []string{
		"allow address from infra/cfg/acl key allowed",
		"allow node n1\ndeny address from infra/sec/acl key peers port 22 proto tcp",
	} {
		assert.NoError(t, DeniedNetRules(noGrant, "test", "net_rules", "", value, shared), value)
		assert.Error(t, DeniedNetRules(noGrant, "dev", "net_rules", "", value, shared), "the sharing is one with the namespace of the object: %s", value)
		assert.Error(t, DeniedNetRules(noGrant, "test", "net_rules", "", value, nil), "nobody says it is shared: %s", value)
	}
	assert.Contains(t, asked, "infra/sec/acl test")

	for value, text := range map[string]string{
		"allow address from prod/cfg/acl key allowed":        "allow address from prod/cfg/acl key allowed",
		"allow node n1 deny address from prod/sec/acl key k": "deny address from prod/sec/acl key k",
		"deny address from cfg/acl key k":                    "deny address from cfg/acl key k",
	} {
		for _, grants := range []rbac.Grants{noGrant, both} {
			assert.EqualError(t, DeniedNetRules(grants, "test", "net_rules@n1", "", value, shared),
				text+": a rule reading its addresses from a datastore of another namespace than the one of the object requires the datastore to share its keys with the namespace of the object, by its share keyword", value)
		}
		assert.NoError(t, DeniedNetRules(root, "test", "net_rules", "", value, shared), value)
		assert.NoError(t, DeniedNetRules(noGrant, "test", "net_rules", "allow localhost "+value, "deny node n3 "+value+" allow node n2", shared),
			"a rule root wrote stays when the others are edited, wherever it is in the list now")
	}

	// A reference that does not evaluate may name any datastore: no
	// sharing is looked for, and the root grant is asked.
	asked = asked[:0]
	for _, value := range []string{
		"allow address from {env.ds} key k",
		"deny address from ./cfg/{env.name} key k port 1 proto udp",
		"allow address from infra/cfg/{env.name} key k",
	} {
		for _, grants := range []rbac.Grants{noGrant, both} {
			assert.EqualError(t, DeniedNetRules(grants, "test", "net_rules@n1", "", value, func(naming.Path, string) bool { asked = append(asked, value); return true }),
				value+": a rule reading its addresses from a datastore named by a reference that does not evaluate requires the root grant", value)
		}
		assert.NoError(t, DeniedNetRules(root, "test", "net_rules", "", value, shared), value)
	}
	assert.Empty(t, asked)
}

// TestNetRulesNamingAnotherNamespaceAreRoots pins the guard of the rules
// naming objects and namespaces: the ones of the namespace of the object
// are its administrator's, however they are written, and one of another
// namespace asks for a grant reading that namespace, or the root grant. A
// pattern of namespaces asks for a grant on the same pattern, and a
// reference that did not evaluate for the root grant.
func TestNetRulesNamingAnotherNamespaceAreRoots(t *testing.T) {
	root := rbac.Grants{rbac.GrantRoot}
	for _, value := range []string{
		"allow object ./svc/db",
		"deny object ./svc/db rid ip#1",
		"allow object test/svc/db",
		"deny namespace .",
		"allow namespace test",
		"allow node n1\ndeny object ./svc/db\nallow namespace .\nallow address 10.0.0.0/8",
		"allow object ./db",
		"allow netwrok lan",
		"object prod/svc/db",
		"",
	} {
		for _, option := range []string{"net_rules", "net_rules@n1"} {
			assert.NoErrorf(t, DeniedNetRules(noGrant, "test", option, "", value, nil), "%s=%s", option, value)
		}
	}
	// Either verb: a deny of the objects of another namespace tells of
	// them as an allow does.
	for value, tc := range map[string]struct{ ref, namespace string }{
		"allow object prod/svc/db":                             {"allow object prod/svc/db", "prod"},
		"deny object prod/svc/db":                              {"deny object prod/svc/db", "prod"},
		"allow object prod/svc/db rid ip#1":                    {"allow object prod/svc/db rid ip#1", "prod"},
		"deny object svc/db":                                   {"deny object db", "root"},
		"allow object db":                                      {"allow object db", "root"},
		"allow namespace prod":                                 {"allow namespace prod", "prod"},
		"deny namespace prod":                                  {"deny namespace prod", "prod"},
		"deny namespace root":                                  {"deny namespace root", "root"},
		"allow node n1 allow object ./svc/db deny namespace *": {"deny namespace *", "*"},
		"allow object {env.ns}/svc/db":                         {"allow object {env.ns}/svc/db", ""},
		"deny namespace {env.ns}":                              {"deny namespace {env.ns}", ""},
	} {
		for _, option := range []string{"net_rules", "net_rules@n1"} {
			assert.EqualErrorf(t, DeniedNetRules(noGrant, "test", option, "", value, nil),
				tc.ref+": a rule naming an object of another namespace than the one of the object, or another namespace, requires the guest, operator or admin grant on that namespace, or the root grant", "%s=%s", option, value)
			assert.NoErrorf(t, DeniedNetRules(root, "test", option, "", value, nil), "%s=%s", option, value)
			for _, role := range []rbac.Role{rbac.RoleGuest, rbac.RoleOperator, rbac.RoleAdmin} {
				reader := rbac.Grants{rbac.NewGrant(rbac.RoleAdmin, "test"), rbac.NewGrant(role, tc.namespace)}
				if tc.namespace == "" {
					assert.Errorf(t, DeniedNetRules(reader, "test", option, "", value, nil), "%s=%s: a reference not evaluated names no namespace a grant reads", option, value)
					continue
				}
				assert.NoErrorf(t, DeniedNetRules(reader, "test", option, "", value, nil), "%s=%s: %s of %s", option, value, role, tc.namespace)
			}
			// Reading another namespace than the one named is no help.
			other := rbac.Grants{rbac.NewGrant(rbac.RoleAdmin, "test"), rbac.NewGrant(rbac.RoleAdmin, "dev")}
			assert.Errorf(t, DeniedNetRules(other, "test", option, "", value, nil), "%s=%s: admin of dev", option, value)
		}
	}
	// A pattern of namespaces asks for the same pattern, not a namespace it
	// selects.
	assert.Error(t, DeniedNetRules(rbac.Grants{rbac.NewGrant(rbac.RoleGuest, "prod")}, "test", "net_rules", "", "allow namespace prod*", nil))
	assert.NoError(t, DeniedNetRules(rbac.Grants{rbac.NewGrant(rbac.RoleGuest, "prod*")}, "test", "net_rules", "", "allow object prod1/svc/db", nil))
	// The same path is the own namespace of an object of prod.
	assert.NoError(t, DeniedNetRules(noGrant, "prod", "net_rules", "", "allow object prod/svc/db\ndeny namespace prod", nil))
	assert.Error(t, DeniedNetRules(noGrant, "prod", "net_rules", "", "allow object ./svc/db\nallow namespace test", nil))
	// Another keyword is none of this guard, the two this one replaces
	// included.
	for _, option := range []string{"start", "net_allow", "net_deny"} {
		assert.NoError(t, DeniedNetRules(noGrant, "test", option, "", "allow object prod/svc/db", nil), option)
	}
}

// TestNetRulesAreJudgedOnWhatTheChangeAdds pins that the references root
// wrote are not asked for again: an edit leaving them as they resolve,
// moving them in the list, or taking some away, needs no grant, and one
// adding another does, a narrowing one and the same reference under the
// other verb included.
func TestNetRulesAreJudgedOnWhatTheChangeAdds(t *testing.T) {
	before := "allow object prod/svc/db\ndeny namespace ops\nallow node n1"
	for _, after := range []string{
		before,
		"allow node n2\nallow object prod/svc/db\ndeny namespace ops\nallow object ./svc/cache",
		"deny namespace ops allow object prod/svc/db",
		"deny namespace ops\ndeny cluster\nallow object prod/svc/db\nallow any",
		"allow object prod/svc/db",
		"allow node n1",
	} {
		assert.NoErrorf(t, DeniedNetRules(noGrant, "test", "net_rules", before, after, nil), "%s", after)
	}
	for _, after := range []string{
		before + "\nallow object prod/svc/api",
		"allow object prod/svc/db rid ip#1\ndeny namespace ops",
		"allow object prod/svc/db\ndeny namespace ops2",
		// The reference held by a rule of a verb is not held by one
		// of the other.
		"deny object prod/svc/db\ndeny namespace ops",
		"allow object prod/svc/db\nallow namespace ops",
		before + "\ndeny object prod/svc/db",
	} {
		assert.Errorf(t, DeniedNetRules(noGrant, "test", "net_rules", before, after, nil), "%s", after)
	}
	// A reader of the namespace named adds what it reads, and no more.
	reader := rbac.Grants{rbac.NewGrant(rbac.RoleGuest, "prod")}
	assert.NoError(t, DeniedNetRules(reader, "test", "net_rules", before, before+"\ndeny object prod/svc/db", nil))
	assert.Error(t, DeniedNetRules(reader, "test", "net_rules", before, before+"\nallow namespace ops", nil))
}
