package object

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	reqjsonrpc "opensvc.com/opensvc/core/client/requester/jsonrpc"
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
		Keystore
	}
)

// NewSec allocates a sec kind object.
func NewSec(p path.T, opts ...funcopt.O) *Sec {
	s := &Sec{}
	s.Base.init(p, opts...)
	return s
}

func (t Sec) Decode(options OptsDecode) ([]byte, error) {
	return t.decode(options.Key, t)
}

func (t Sec) CustomDecode(s string) ([]byte, error) {
	if !strings.HasPrefix(s, "crypt:") {
		return []byte{}, fmt.Errorf("unsupported value (no crypt prefix)")
	}

	// decode base64
	b, err := base64.URLEncoding.DecodeString(s[6:])
	if err != nil {
		return []byte{}, err
	}

	// remove the trailing \r
	b = b[:len(b)-1]

	// decrypt AES
	m := reqjsonrpc.NewMessage(b)
	b, err = m.Decrypt()
	if err != nil {
		return []byte{}, err
	}

	err = json.Unmarshal(b, &s)
	if err != nil {
		return []byte{}, err
	}
	return []byte(s), nil
}
