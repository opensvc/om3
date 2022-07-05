package object

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"opensvc.com/opensvc/core/path"
)

func TestVolatileFuncOpt(t *testing.T) {
	t.Run("volatile funcopt", func(t *testing.T) {
		p, _ := path.Parse("ci/svc/alpha")
		o, err := NewSvc(p, WithVolatile(true))
		assert.NoError(t, err)
		assert.Equal(t, o.IsVolatile(), true)
	})
}
