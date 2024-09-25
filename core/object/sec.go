package object

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/opensvc/om3/core/keywords"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/omcrypto"
	"github.com/opensvc/om3/util/funcopt"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/key"
)

type (
	sec struct {
		keystore
	}

	secEncodeDecode struct {
		encryptDecrypter encryptDecrypter
	}

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
	Sec interface {
		Keystore
		SecureKeystore
	}

	encryptDecrypter interface {
		Decrypt([]byte) (b []byte, err error)
		Encrypt([]byte) ([]byte, error)
	}
)

var (
	secEncryptDecrypterMutex sync.Mutex
	secEncryptDecrypterCache encryptDecrypter
)

func GetSecEncryptDecrypter() (encryptDecrypter, error) {
	secEncryptDecrypterMutex.Lock()
	defer secEncryptDecrypterMutex.Unlock()
	if secEncryptDecrypterCache != nil {
		return secEncryptDecrypterCache, nil
	}
	cluster, err := NewCluster()
	if err != nil {
		return nil, err
	}
	secEncryptDecrypterCache = &omcrypto.Factory{
		NodeName:    hostname.Hostname(),
		ClusterName: cluster.Config().GetString(key.T{Section: "cluster", Option: "name"}),
		Key:         cluster.Config().GetString(key.T{Section: "cluster", Option: "secret"}),
	}
	return secEncryptDecrypterCache, nil
}

// NewSec allocates a sec kind object.
func NewSec(path naming.Path, opts ...funcopt.O) (*sec, error) {
	s := &sec{}
	s.path = path
	s.path.Kind = naming.KindSec
	if err := s.init(s, path, opts...); err != nil {
		return s, err
	}
	ed, err := GetSecEncryptDecrypter()
	if err != nil {
		return s, err
	}
	s.encodeDecoder = &secEncodeDecode{
		encryptDecrypter: ed,
	}
	s.Config().RegisterPostCommit(s.postCommit)
	return s, nil
}

func (t *sec) KeywordLookup(k key.T, sectionType string) keywords.Keyword {
	return keywordLookup(keywordStore, k, t.path.Kind, sectionType)
}

func (t *secEncodeDecode) Encode(b []byte) (string, error) {
	b, err := t.encryptDecrypter.Encrypt(b)
	if err != nil {
		return "", err
	}
	return "crypt:" + base64.URLEncoding.Strict().EncodeToString(b), nil
}

func (t *secEncodeDecode) Decode(s string) ([]byte, error) {
	if !strings.HasPrefix(s, "crypt:") {
		return []byte{}, fmt.Errorf("unsupported value (no crypt prefix)")
	}

	// decode base64
	b, err := base64.URLEncoding.DecodeString(s[6:])
	if err != nil {
		return []byte{}, err
	}

	// remove the trailing \r
	last := len(b) - 1
	if b[last] == '\x00' {
		b = b[:last]
	}

	// decrypt AES
	b, err = t.encryptDecrypter.Decrypt(b)
	if err != nil {
		return []byte{}, err
	}

	err = json.Unmarshal(b, &s)
	if err != nil {
		return b, nil
	}
	return []byte(s), nil
}
