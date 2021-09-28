package object

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"opensvc.com/opensvc/core/path"
)

func TestVolatileFuncOpt(t *testing.T) {
	t.Run("volatile funcopt", func(t *testing.T) {
		p, _ := path.Parse("ci/svc/alpha")
		o, err := NewFromPath(p, WithVolatile(true))
		i := o.(*Svc)
		assert.Nil(t, err, "NewFromPath(p) mustn't return an error")
		assert.Equal(t, i.IsVolatile(), true)
	})
}
