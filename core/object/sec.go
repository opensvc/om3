package object

import (
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/util/funcopt"
)

type (
	//
	// Sec is the sec-kind object.
	//
	// These objects are encrypted key-value store.
	// Values can be binary or text.
	//
	// A Key can be installed as a file in a Vol, then exposed to apps
	// and containers.
	// A Key can be exposed as a environment variable for apps and
	// containers.
	// A Signal can be sent to consumer processes upon exposed key value
	// changes.
	//
	Sec struct {
		Base
	}
)

// NewSec allocates a sec kind object.
func NewSec(p path.T, opts ...funcopt.O) *Sec {
	s := &Sec{}
	s.Base.init(p, opts...)
	return s
}
