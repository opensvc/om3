package object_test

import (
	"context"
	"os"
	"testing"

	"github.com/iancoleman/orderedmap"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/actioncontext"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/om"
	"github.com/opensvc/om3/v3/testhelper"
	"github.com/opensvc/om3/v3/util/plog"

	_ "github.com/opensvc/om3/v3/core/driverdb"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/util/file"
	"github.com/opensvc/om3/v3/util/key"
)

var sectionApp0 = []byte(`
[app#0]
start = /usr/bin/touch {env.flag0}
stop = /usr/bin/rm -f {env.flag0}
check = /usr/bin/test -f {env.flag0}
`)

var sectionApp1 = []byte(`
[app#1]
start = /usr/bin/touch {env.flag1}
stop = /usr/bin/rm -f {env.flag1}
check = /usr/bin/test -f {env.flag1}
`)

var sectionEnv = []byte(`
[env]
flag0 = /tmp/TestAppStart.{fqdn}.0
flag1 = /tmp/TestAppStart.{fqdn}.1
`)

func TestMain(m *testing.M) {
	testhelper.Main(m, om.ExecuteArgs)
}

func TestAppStart(t *testing.T) {
	testhelper.Setup(t)
	t.Run("conf1", func(t *testing.T) {
		var conf []byte
		conf = append(conf, sectionApp0...)
		conf = append(conf, sectionApp1...)
		conf = append(conf, sectionEnv...)

		p, err := naming.ParsePath("conf1")
		assert.NoError(t, err)

		logger := plog.NewDefaultLogger().Attr("pkg", "core/object").WithPrefix("core: object: test: ")
		s, err := object.NewSvc(p,
			object.WithConfigData(conf),
			object.WithLogger(logger),
		)
		assert.NoError(t, err)

		fpath := s.Config().GetString(key.T{Section: "env", Option: "flag0"})
		assert.NotEqual(t, fpath, "")

		defer func() {
			_ = os.RemoveAll(fpath)
		}()

		require.NoErrorf(t, os.RemoveAll(fpath), "%s should not exist before start", fpath)

		ctx := context.Background()
		ctx = actioncontext.WithForce(ctx, true)
		ctx = actioncontext.WithRID(ctx, "app#0")
		err = s.Start(ctx)
		assert.NoErrorf(t, err, "Start() should not err")
		require.True(t, file.Exists(fpath), "%s should exist after start", fpath)
		// TODO: need dedicated test with log action (no more explicit object log)
		//events, err := streamlog.GetEventsFromFile(p.LogFile(), map[string]interface{}{"sid": xsession.ID.String()})
		//assert.NoError(t, err)
		//assert.Truef(t, events.MatchString("cmd", ".*touch.*"), "logs should contain a cmd~/touch/ event")
	})
}

// TestWithConfigData exercises different data types passed to object.WithConfigData(any)
func TestWithConfigData(t *testing.T) {
	testhelper.Setup(t)
	t.Run("conf1", func(t *testing.T) {
		var (
			o   object.Svc
			err error
		)
		p, _ := naming.ParsePath("conf1")
		conf1 := map[string]map[string]string{
			"app#1": {
				"start": "/usr/bin/touch {env.flag1}",
				"stop":  "/usr/bin/rm -f {env.flag1}",
				"check": "/usr/bin/test -f {env.flag1}",
			},
			"env": {
				"flag0": "/tmp/{fqdn}.0",
				"flag1": "/tmp/{fqdn}.1",
				"foo1":  "1",
			},
		}
		o, err = object.NewSvc(p, object.WithConfigData(conf1))
		assert.NoError(t, err)
		assert.Equal(t, "1", o.Config().GetString(key.Parse("env.foo1")))

		conf2 := map[string]map[string]any{
			"app#1": {
				"start": "/usr/bin/touch {env.flag1}",
				"stop":  "/usr/bin/rm -f {env.flag1}",
				"check": "/usr/bin/test -f {env.flag1}",
			},
			"env": {
				"flag0": "/tmp/{fqdn}.0",
				"flag1": "/tmp/{fqdn}.1",
				"foo1":  1,
			},
		}
		o, err = object.NewSvc(p, object.WithConfigData(conf2))
		assert.NoError(t, err)
		assert.Equal(t, "1", o.Config().GetString(key.Parse("env.foo1")))

		env3 := orderedmap.New()
		env3.Set("foo1", 1)
		conf3 := orderedmap.New()
		conf3.Set("env", env3)
		o, err = object.NewSvc(p, object.WithConfigData(conf3))
		assert.NoError(t, err)
		assert.Equal(t, "1", o.Config().GetString(key.Parse("env.foo1")))

		conf4 := orderedmap.New()
		conf4.Set("env", map[string]any{
			"foo1": 1,
		})
		o, err = object.NewSvc(p, object.WithConfigData(conf4))
		assert.NoError(t, err)
		assert.Equal(t, "1", o.Config().GetString(key.Parse("env.foo1")))

	})
}
