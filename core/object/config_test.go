package object

import (
	"testing"

	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/xconfig"
	"github.com/opensvc/om3/util/key"
	"github.com/stretchr/testify/require"
)

func TestConfigDerefScopedEnv(t *testing.T) {
	cf := []byte(`
[DEFAULT]
priority = {env.priority}

[env]
priority = 1
priority@n2 = 2
`)

	p, _ := naming.ParsePath("test/svc/svc1")
	o, err := NewSvc(p, WithConfigData(cf))
	require.NoError(t, err)

	v, err := o.Config().EvalAs(key.Parse("priority"), "n2")
	require.NoError(t, err)
	require.Equal(t, 2, v)

	v, err = o.Config().EvalAs(key.Parse("priority"), "")
	require.NoError(t, err)
	require.Equal(t, 1, v)
}

func TestConfigDerefIntraSection(t *testing.T) {
	cf := []byte(`
[env]
a = {b}
b = foo
`)

	p, _ := naming.ParsePath("test/svc/svc1")
	o, err := NewSvc(p, WithConfigData(cf))
	require.NoError(t, err)

	v, err := o.Config().Eval(key.Parse("env.a"))
	require.NoError(t, err)
	require.Equal(t, "foo", v)
}

func TestConfigDerefLoop(t *testing.T) {
	cf := []byte(`
[env]
a = {b}
b = {a}
`)

	p, _ := naming.ParsePath("test/svc/svc1")
	o, err := NewSvc(p, WithConfigData(cf))
	require.NoError(t, err)

	_, err = o.Config().Eval(key.Parse("env.a"))
	require.ErrorIs(t, err, xconfig.ErrInfiniteDeferenceRecursion)
}

func TestConfigDerefName(t *testing.T) {
	cf := []byte(`
[env]
name = {name}
`)

	p, _ := naming.ParsePath("test/svc/svc1")
	o, err := NewSvc(p, WithConfigData(cf))
	require.NoError(t, err)

	value, err := o.Config().Eval(key.Parse("env.name"))
	require.NoError(t, err)
	require.Equal(t, "svc1", value)
}

func TestConfigDerefWithRepeatedRef(t *testing.T) {
	cf := []byte(`
[env]
a = foo
b = {a} {a}
`)

	p, _ := naming.ParsePath("test/svc/svc1")
	o, err := NewSvc(p, WithConfigData(cf))
	require.NoError(t, err)

	value, err := o.Config().Eval(key.Parse("env.b"))
	require.NoError(t, err)
	require.Equal(t, "foo foo", value)
}

func TestConfigValidateExposedDev(t *testing.T) {
	cf := []byte(`
[env]
a = {disk#1.exposed_devs[0]}
`)

	p, _ := naming.ParsePath("test/svc/svc1")
	o, err := NewSvc(p, WithConfigData(cf))
	require.NoError(t, err)

	alerts, err := o.Config().Validate()
	require.NoError(t, err)
	require.Len(t, alerts, 0)
}

func TestConfigCountConverterList(t *testing.T) {
	cf := []byte(`
[DEFAULT]
affinity = a b c

[env]
a = {#DEFAULT.affinity}
`)

	p, _ := naming.ParsePath("test/svc/svc1")
	o, err := NewSvc(p, WithConfigData(cf))
	require.NoError(t, err)

	value, err := o.Config().Eval(key.Parse("env.a"))
	require.NoError(t, err)
	require.Equal(t, "3", value)
}

func TestConfigDerefFromEnvList(t *testing.T) {
	cf := []byte(`
[DEFAULT]
affinity = {env.l[1]} {env.l[0]}

[env]
l = a b
`)

	p, _ := naming.ParsePath("test/svc/svc1")
	o, err := NewSvc(p, WithConfigData(cf))
	require.NoError(t, err)

	value, err := o.Config().Eval(key.Parse("affinity"))
	require.NoError(t, err)
	require.Equal(t, []string{"b", "a"}, value)
}

func TestConfigDerefResourceType(t *testing.T) {
	cf := []byte(`
[fs#1]
type = {env.fstype}

[env]
fstype = flag
`)

	p, _ := naming.ParsePath("test/svc/svc1")
	o, err := NewSvc(p, WithConfigData(cf))
	require.NoError(t, err)

	_, err = SetClusterConfig()
	o.ConfigureResources()
	r := o.ResourceByID("fs#1")
	require.NotNil(t, r)
}
