package object

type (
	//
	// Svc is the svc-kind object.
	//
	// These objects contain front facing resources like app and containers.
	//
	Svc struct {
		Base
	}
)

// NewSvc allocates a svc kind object.
func NewSvc(path Path) *Svc {
	s := &Svc{}
	s.Base.init(path)
	return s
}
