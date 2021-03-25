package object

type (
	//
	// Ccfg is the clusterwide configuration store.
	//
	// The content is the same as node.conf, and is overriden by
	// the definition found in node.conf.
	//
	Ccfg struct {
		Base
	}
)

// NewCcfg allocates a ccfg kind object.
func NewCcfg(path Path) *Ccfg {
	s := &Ccfg{}
	s.Base.init(path)
	return s
}
