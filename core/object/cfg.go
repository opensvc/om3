package object

type (
	//
	// Cfg is the cfg-kind object.
	//
	// These objects are unencrypted key-value store.
	// Values can be binary or text.
	//
	// A Key can be installed as a file in a Vol, then exposed to apps
	// and containers.
	// A Key can be exposed as a environment variable for apps and
	// containers.
	// A Signal can be sent to consumer processes upon exposed key value
	// changes.
	//
	Cfg struct {
		Base
	}
)

// NewCfg allocates a cfg kind object.
func NewCfg(path Path) *Cfg {
	s := &Cfg{}
	s.Base.init(path)
	return s
}
