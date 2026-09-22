package freeze

import (
	"time"

	"github.com/opensvc/om3/v3/core/flagfile"
)

func Freeze(p string) error {
	return flagfile.Set(p)
}

func Unfreeze(p string) error {
	return flagfile.Unset(p)
}

func Frozen(p string) time.Time {
	return flagfile.At(p)
}
