package object

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"

	"software.sslmate.com/src/go-pkcs12"
)

// PKCS returns the PKCS#12 format bytes of the private key and certificate
// chain stored in this keyStore
func (t *sec) PKCS(password []byte) ([]byte, error) {
	if !t.HasKey("private_key") {
		return nil, fmt.Errorf("private_key does not exist")
	}
	if !t.HasKey("certificate_chain") {
		return nil, fmt.Errorf("certificate_chain does not exist")
	}
	privateKeyBytes, err := t.DecodeKey("private_key")
	if err != nil {
		return nil, err
	}
	certificateChainBytes, err := t.DecodeKey("certificate_chain")
	if err != nil {
		return nil, err
	}
	return PKCS(privateKeyBytes, certificateChainBytes, password)
}

func PKCS(privateKeyBytes, certificateChainBytes, passwordBytes []byte) ([]byte, error) {
	block, rest := pem.Decode(privateKeyBytes)
	if block == nil || block.Type != "PRIVATE KEY" {
		return nil, fmt.Errorf("failed to decode PEM block containing private key")
	}
	privateKey, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		privateKey, err = x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, err
		}
	}

	l := make([]*x509.Certificate, 0)
	for {
		block, certificateChainBytes = pem.Decode(certificateChainBytes)
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" {
			return nil, fmt.Errorf("failed to decode PEM block containing certificate. actual type %s", block.Type)
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, err
		}
		l = append(l, cert)
		if rest == nil {
			break
		}
	}
	if len(l) < 1 {
		return nil, fmt.Errorf("certificate_chain has no valid certificate")
	}
	return pkcs12.Modern.Encode(privateKey, l[0], l[1:], string(passwordBytes))
}
