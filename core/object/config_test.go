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
