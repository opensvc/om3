package cmd

import (
	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/daemon/daemonenv"
	"github.com/opensvc/om3/util/funcopt"
)

func newClient() (*client.T, error) {
	clientOptions := []funcopt.O{client.WithURL(serverFlag)}
	if serverFlag == daemonenv.UrlInetHttp() {
		clientOptions = append(
			clientOptions,
			client.WithInsecureSkipVerify(true),
			client.WithCertificate(daemonenv.CertChainFile()),
			client.WithKey(daemonenv.KeyFile()),
		)
	}
	return client.New(clientOptions...)
}
