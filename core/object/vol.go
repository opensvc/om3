package object

import (
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/util/funcopt"
)

type (
	//
	// Vol is the vol-kind object.
	//
	// These objects contain cluster-dependent fs, disk and sync resources.
	//
	// They are created by feeding a volume resource configuration (cluster
	// independant) to a pool.
	//
	Vol struct {
		Base
	}
)

// NewVol allocates a vol kind object.
func NewVol(p path.T, opts ...funcopt.O) *Vol {
	s := &Vol{}
	s.Base.init(p, opts...)
	return s
}
