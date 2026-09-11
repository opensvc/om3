package daemonapi

import (
	"fmt"
	"os"

	"github.com/golang-jwt/jwt/v5"
)

type (
	// caClaim is the subset of the token claims the enroll flow reads: the
	// certificate chain of the cluster that issued the token.
	caClaim struct {
		Ca string `json:"ca"`
		*jwt.RegisteredClaims
	}
)

// caFromToken extracts the ca claim from tk without verifying its signature.
//
// The claim is only used to trust the TLS certificate of the node the token
// was created on, before talking to it. It grants nothing by itself:
//
//  1. The same token is sent to that node as a Bearer token, and that node
//     validates its signature with its own cluster public key
//  2. A forged token would only point us at a node of the forger's choosing,
//     which a root requester (the only role this endpoint accepts) can already
//     reach, and hand it a join token it could have created itself
func caFromToken(tk string) ([]byte, error) {
	parser := jwt.Parser{}
	token, _, err := parser.ParseUnverified(tk, &caClaim{})
	if err != nil {
		return nil, err
	}
	claim, ok := token.Claims.(*caClaim)
	if !ok {
		return nil, fmt.Errorf("invalid token claims")
	}
	if claim.Ca == "" {
		return nil, fmt.Errorf("token claim ca is empty")
	}
	return []byte(claim.Ca), nil
}

// tmpCertFile writes b to a new temporary file and returns its name. The
// caller is responsible for removing it.
func tmpCertFile(b []byte) (string, error) {
	f, err := os.CreateTemp("", "cert.pem")
	if err != nil {
		return "", err
	}
	name := f.Name()
	defer func() { _ = f.Close() }()
	if _, err := f.Write(b); err != nil {
		_ = os.Remove(name)
		return "", err
	}
	return name, nil
}
