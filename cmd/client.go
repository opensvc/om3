package cmd

import (
	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/daemon/daemonenv"
	"opensvc.com/opensvc/util/funcopt"
)

func newClient() (*client.T, error) {
	clientOptions := []funcopt.O{client.WithURL(serverFlag)}
	if serverFlag == daemonenv.UrlInetHttp() {
		clientOptions = append(
			clientOptions,
			client.WithInsecureSkipVerify(true),
			client.WithCertificate(daemonenv.CertFile()),
			client.WithKey(daemonenv.KeyFile()),
		)
	}
	return client.New(clientOptions...)
}
