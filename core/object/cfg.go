package object

import (
	"encoding/base64"
	"strings"
	"unicode"

	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/util/funcopt"
)

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
		Keystore
	}
)

// NewCfg allocates a cfg kind object.
func NewCfg(p path.T, opts ...funcopt.O) *Cfg {
	s := &Cfg{}
	s.CustomEncode = cfgEncode
	s.CustomDecode = cfgDecode
	s.Base.init(p, opts...)
	s.Config().RegisterPostCommit(s.postCommit)
	return s
}

func cfgEncode(b []byte) (string, error) {
	switch {
	case isAsciiPrintable(b):
		return "literal:" + string(b), nil
	default:
		return "base64:" + base64.RawURLEncoding.Strict().EncodeToString(b), nil
	}
}

func cfgDecode(s string) ([]byte, error) {
	switch {
	case strings.HasPrefix(s, "base64:"):
		return base64.URLEncoding.DecodeString(s[7:])
	case strings.HasPrefix(s, "literal:"):
		return []byte(s[8:]), nil
	default:
		return []byte(s), nil
	}
}

func isAsciiPrintable(bytes []byte) bool {
	for _, b := range bytes {
		r := rune(b)
		if r > unicode.MaxASCII || !unicode.IsPrint(r) {
			return false
		}
	}
	return true
}
