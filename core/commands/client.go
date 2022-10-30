package commands

import (
	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/daemon/daemonenv"
	"opensvc.com/opensvc/util/funcopt"
)

func newClient(server string) (*client.T, error) {
	clientOptions := []funcopt.O{client.WithURL(server)}
	if server == daemonenv.UrlInetHttp() {
		clientOptions = append(
			clientOptions,
			client.WithInsecureSkipVerify(true),
			client.WithCertificate(daemonenv.CertFile()),
			client.WithKey(daemonenv.KeyFile()),
		)
	}
	return client.New(clientOptions...)
}
