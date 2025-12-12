package object_test

import (
	"testing"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/testhelper"
	"github.com/stretchr/testify/assert"
)

func TestVolatileFuncOpt(t *testing.T) {
	t.Run("volatile funcopt", func(t *testing.T) {
		testhelper.Setup(t)
		p, _ := naming.ParsePath("ci/svc/alpha")
		o, err := object.NewSvc(p, object.WithVolatile(true))
		assert.NoError(t, err)
		assert.Equal(t, o.IsVolatile(), true)
	})
}
