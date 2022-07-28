package object_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/testhelper"
)

func TestVolatileFuncOpt(t *testing.T) {
	t.Run("volatile funcopt", func(t *testing.T) {
		testhelper.Setup(t)
		p, _ := path.Parse("ci/svc/alpha")
		o, err := object.NewSvc(p, object.WithVolatile(true))
		assert.NoError(t, err)
		assert.Equal(t, o.IsVolatile(), true)
	})
}
