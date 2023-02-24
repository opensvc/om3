package draincommand

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_SetError(t *testing.T) {
	err := make(chan error)
	var o ErrC = err
	var i interface{}
	i = o
	go i.(ErrorSetter).SetError(nil)
	assert.NoError(t, <-err)
}

func TestDoDo(t *testing.T) {
	type op struct {
		ErrC
		b bool
	}
	cmdC := make(chan any)
	err1 := make(chan error)
	err2 := make(chan error)

	go func() {
		cmdC <- op{ErrC: err1, b: true}
		cmdC <- op{ErrC: err2, b: false}
	}()
	Do(cmdC, time.Millisecond)
	assert.ErrorIs(t, <-err1, ErrDrained)
	assert.ErrorIs(t, <-err2, ErrDrained)
}
