package cmd

import (
	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/daemon/daemonenv"
	"opensvc.com/opensvc/util/funcopt"
)

func newClient() (*client.T, error) {
	clientOptions := []funcopt.O{client.WithURL(serverFlag)}
	if serverFlag == daemonenv.UrlInetHttp {
		clientOptions = append(clientOptions,
			client.WithInsecureSkipVerify())

		clientOptions = append(clientOptions,
			client.WithCertificate(daemonenv.CertFile))

		clientOptions = append(clientOptions,

			client.WithKey(daemonenv.KeyFile),
		)
	}
	return client.New(clientOptions...)
}
