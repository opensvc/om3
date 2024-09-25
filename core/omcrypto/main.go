// Package omcrypto is responsible for Message Encrypt, Decrypt, DecryptWithNode
package omcrypto

import (
	"bytes"
	"compress/zlib"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
)

type (
	encryptedMessage struct {
		ClusterName string `json:"clustername"`
		NodeName    string `json:"nodename"`
		IV          string `json:"iv"`
		Data        string `json:"data"`
	}

	Factory struct {
		NodeName    string
		ClusterName string
		Key         string
	}
)

func (m *Factory) assertValid() {
	if m.ClusterName == "" {
		panic("NewMessage: unexpected empty cluster name")
	}
	if m.Key == "" {
		panic("NewMessage: unexpected empty key")
	}
}

// DecryptWithNode Decrypt the message
//
// returns decodedMsg []byte, encryptorNodename string, error
func (m *Factory) DecryptWithNode(data []byte) ([]byte, string, error) {
	m.assertValid()
	if len(data) == 0 {
		// fast return, Unmarshal will fail
		return nil, "", io.EOF
	}
	var b []byte
	key := []byte(m.Key)
	msg := &encryptedMessage{}
	err := json.Unmarshal(data, msg)
	if err != nil {
		return nil, "", fmt.Errorf("analyse message unmarshal failure: %w", err)
	}
	// TODO: test nodename and clustername, plug blacklist
	b, err = decode(msg.Data, msg.IV, key)
	if err != nil {
		return b, "", fmt.Errorf("analyse message decode failure: %w", err)
	}
	return b, msg.NodeName, err
}

// Decrypt decrypts the message, if the nodename found in the message is a
// cluster node.
func (m *Factory) Decrypt(data []byte) ([]byte, error) {
	m.assertValid()
	b, _, err := m.DecryptWithNode(data)
	return b, err
}

// Encrypt encrypts the message and returns a json with head keys describing
// the sender, and embedding the AES-encypted + Base64-encoded data.
func (m *Factory) Encrypt(data []byte) ([]byte, error) {
	m.assertValid()
	var (
		encoded   string
		encodedIV string
		err       error
	)
	key := []byte(m.Key)
	if encoded, encodedIV, err = encode(data, key); err != nil {
		return nil, err
	}
	msg := &encryptedMessage{
		ClusterName: m.ClusterName,
		NodeName:    m.NodeName,
		IV:          encodedIV,
		Data:        encoded,
	}
	return json.Marshal(msg)
}

func decode(encoded string, iv string, key []byte) ([]byte, error) {
	var (
		decodedIV []byte
		decoded   []byte
		err       error
	)
	decodedIV, err = base64.URLEncoding.DecodeString(iv)
	if err != nil {
		return nil, err
	}
	decoded, err = base64.URLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, err
	}
	decoded, err = decrypt(decoded, key, decodedIV)
	if err != nil {
		return nil, err
	}
	return decompress(decoded)
}

func encode(data []byte, key []byte) (string, string, error) {
	var (
		b   []byte
		iv  []byte
		err error
	)
	b, err = compress(data)
	if err != nil {
		return "", "", err
	}
	b, iv, err = encrypt(b, key)
	if err != nil {
		return "", "", err
	}
	encoded := base64.URLEncoding.EncodeToString(b)
	encodedIV := base64.URLEncoding.EncodeToString(iv)
	return encoded, encodedIV, nil
}

func decrypt(b []byte, key []byte, iv []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	//iv := b[:aes.BlockSize]
	//b = b[aes.BlockSize:]
	if len(b)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("cipherText is not a multiple of the block size")
	}
	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(b, b)
	return unpadPKCSS(b, aes.BlockSize)
}

func encrypt(b []byte, key []byte) ([]byte, []byte, error) {
	iv := newIV()
	padded := padPKCSS(b, aes.BlockSize, len(b))
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, err
	}
	ciphertext := make([]byte, len(padded))
	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(ciphertext, padded)
	return ciphertext, iv, nil
}

func unpadPKCSS(b []byte, blockSize int) ([]byte, error) {
	if blockSize < 1 {
		return nil, fmt.Errorf("block size too small")
	}
	if len(b)%blockSize != 0 {
		return nil, fmt.Errorf("data isn't aligned to blockSize")
	}
	if len(b) == 0 {
		return nil, fmt.Errorf("data is empty")
	}
	paddingLength := int(b[len(b)-1])
	if paddingLength > len(b) {
		return nil, fmt.Errorf("the PKCSS padding (%d) is longer than message (%d)", paddingLength, len(b))
	}
	for _, el := range b[len(b)-paddingLength:] {
		if el != byte(paddingLength) {
			errStr := fmt.Sprintf("padding had malformed entry '%x', expected '%x'", paddingLength, el)
			return nil, fmt.Errorf(errStr)
		}
	}
	return b[:len(b)-paddingLength], nil
}

func padPKCSS(b []byte, blockSize int, after int) []byte {
	padding := (blockSize - len(b)%blockSize)
	padtext := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(b, padtext...)
}

func newIV() []byte {
	b := make([]byte, 16)
	rand.Read(b)
	return b
}

func compress(b []byte) ([]byte, error) {
	var bb bytes.Buffer
	w := zlib.NewWriter(&bb)
	if _, err := w.Write(b); err != nil {
		return nil, err
	}
	if err := w.Flush(); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return bb.Bytes(), nil
}

func decompress(b []byte) ([]byte, error) {
	bb := bytes.NewReader(b)
	r, err := zlib.NewReader(bb)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}
