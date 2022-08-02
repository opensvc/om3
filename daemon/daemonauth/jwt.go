package daemonauth

import (
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"

	"github.com/dgrijalva/jwt-go"
	"github.com/go-chi/jwtauth/v5"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/ssh"
	"opensvc.com/opensvc/daemon/daemonenv"
)

var (
	TokenAuth        *jwtauth.JWTAuth
	verifyBytes      []byte
	verifyKey        *rsa.PublicKey
	signKey          *rsa.PrivateKey
	jwtSignKeyFile   string
	jwtVerifyKeyFile string
)

func jsonEncode(w io.Writer, data interface{}) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "    ")
	return enc.Encode(data)
}

func initJWT() error {
	log.Logger.Info().Msg("init token factory")
	jwtSignKeyFile = daemonenv.KeyFile()
	jwtVerifyKeyFile = daemonenv.CertFile()

	if jwtSignKeyFile == "" && jwtVerifyKeyFile == "" {
		return fmt.Errorf("the system/sec/cert-{clustername} listener private_key and certificate must exist.")
	} else if jwtSignKeyFile != "" {
		signBytes, err := ioutil.ReadFile(jwtSignKeyFile)
		if err != nil {
			return err
		}
		if signKey, err = jwt.ParseRSAPrivateKeyFromPEM(signBytes); err != nil {
			return err
		}
		if jwtVerifyKeyFile == "" {
			return fmt.Errorf("key file is set to the path of a RSA key. In this case, the certificate file must also be set to the path of the RSA public key.")
		}
		if verifyBytes, err = ioutil.ReadFile(jwtVerifyKeyFile); err != nil {
			return err
		}
		if verifyKey, err = jwt.ParseRSAPublicKeyFromPEM(verifyBytes); err != nil {
			return err
		} else {
			if pk, err := ssh.NewPublicKey(verifyKey); err != nil {
				log.Logger.Info().Msgf("  load verify key: %s", err)
			} else {
				finger := ssh.FingerprintLegacyMD5(pk)
				log.Logger.Info().Msgf("  verify key sig: %s", finger)
			}
			TokenAuth = jwtauth.New("RS256", signKey, verifyKey)
		}
	} else {
		return errors.Errorf("the system/sec/cert-{clustername} listener private_key must exist.")
		// If we want to support less secure HMAC token from a static sign key:
		//	TokenAuth = jwtauth.New("HMAC", []byte(jwtSignKey), nil)
	}
	return nil
}
