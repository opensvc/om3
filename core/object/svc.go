package object

type (
	// Svc is the svc-kind object.
	// svc objects contain front facing resources like app and containers.
	Svc struct {
		Base
	}
)

func NewSvc(path Path) *Svc {
	s := &Svc{}
	s.Path = path
	return s
}
