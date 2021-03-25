package object

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
func NewVol(path Path) *Vol {
	s := &Vol{}
	s.Base.init(path)
	return s
}
