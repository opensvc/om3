package object

import "github.com/pkg/errors"

// FullPEM returns the PEM format string of the private key and certificate
// chain stored in this secure keystore
func (t *sec) FullPEM() (string, error) {
	var s string
	for _, key := range []string{"private_key", "certificate_chain"} {
		if !t.HasKey(key) {
			return s, errors.Errorf("%s does not exist", key)
		}
		buff, err := t.DecodeKey(key)
		if err != nil {
			return s, err
		}
		s += string(buff)
	}
	return s, nil
}
