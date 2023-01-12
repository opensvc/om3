package commands

import (
	"fmt"
	"os"

	"github.com/dgrijalva/jwt-go"
	"github.com/pkg/errors"

	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/core/kind"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/daemon/daemonauth"
	"opensvc.com/opensvc/daemon/daemonenv"
	"opensvc.com/opensvc/daemon/remoteconfig"
	"opensvc.com/opensvc/util/command"
)

type (
	CmdDaemonJoin struct {
		OptsGlobal
		Node string
		Tk   string
	}
)

func (t *CmdDaemonJoin) Run() error {
	var (
		certFile string
		cli      *client.T
	)
	_, _ = fmt.Fprintf(os.Stderr, "Joining...\n")
	if err := t.checkParams(); err != nil {
		return err
	}
	certChain, err := t.extractCaClaim()
	if err != nil {
		return err
	}
	certFile, err = t.createTmpCertFile(certChain)
	if err != nil {
		return err
	}
	defer func(name string) {
		_ = os.Remove(name)
	}(certFile)

	cli, err = client.New(
		client.WithURL(daemonenv.UrlHttpNode(t.Node)),
		client.WithRootCa(certFile),
		client.WithBearer(t.Tk),
	)
	if err != nil {
		return err
	}

	clusterPath := path.T{Name: "cluster", Kind: kind.Ccfg}
	_, _ = fmt.Fprintf(os.Stderr, "Fetching initial %s from %s\n", clusterPath, t.Node)
	file, _, err := remoteconfig.FetchObjectFile(cli, clusterPath)
	if err != nil {
		return err
	}
	defer func() {
		_ = os.Remove(file)
	}()
	clusterCfg, err := object.NewCcfg(clusterPath, object.WithConfigFile(file))
	if err != nil {
		return err
	}
	filePaths := make(map[string]path.T)
	clusterName := clusterCfg.Name()

	_, _ = fmt.Fprintf(os.Stderr, "Adding localhost to remote cluster %s\n", t.Node)
	// TODO POST daemon join

	toFetch := []path.T{
		clusterPath,
		{Namespace: "system", Kind: kind.Sec, Name: "ca-" + clusterName},
		{Namespace: "system", Kind: kind.Sec, Name: "cert-" + clusterName},
	}
	for _, p := range toFetch {
		var file string
		_, _ = fmt.Fprintf(os.Stderr, "Fetching %s from %s\n", p, t.Node)
		file, _, err = remoteconfig.FetchObjectFile(cli, p)
		if err != nil {
			return err
		}
		defer func(name string) {
			_ = os.Remove(name)
		}(file)
		filePaths[file] = p
	}

	args := []string{"daemon", "stop"}
	cmd := command.New(
		command.WithName(os.Args[0]),
		command.WithArgs(args),
	)
	_, _ = fmt.Fprintf(os.Stderr, "Stopping daemon...\n")
	err = cmd.Run()
	if err != nil {
		return err
	}

	// TODO backup conf files before install files from remote

	for fileName, p := range filePaths {
		_, _ = fmt.Fprintf(os.Stderr, "Installing fetched config %s\n", p)
		err := os.Rename(fileName, p.ConfigFile())
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "can't install fetched config %s from file %s\n", p, fileName)
		}
	}

	args = []string{"daemon", "start"}
	cmd = command.New(
		command.WithName(os.Args[0]),
		command.WithArgs(args),
	)
	_, _ = fmt.Fprintf(os.Stderr, "Starting daemon...\n")
	err = cmd.Run()
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(os.Stderr, "Joined\n")
	return nil
}

func (t *CmdDaemonJoin) checkParams() error {
	if t.Node == "" {
		return errors.New("need a cluster node to join cluster")
	}
	if t.Tk == "" {
		return errors.New("need a token to join cluster")
	}
	return nil
}

func (t *CmdDaemonJoin) extractCaClaim() (ca []byte, err error) {
	type (
		joinClaim struct {
			Ca string `json:"ca"`
			*daemonauth.ApiClaims
		}
	)
	var (
		parser = jwt.Parser{}
		token  *jwt.Token
	)

	token, _, err = parser.ParseUnverified(t.Tk, &joinClaim{})
	if err != nil {
		return
	}
	if claim, ok := token.Claims.(*joinClaim); ok {
		ca = []byte(claim.Ca)
	} else {
		err = errors.New("invalid token claims")
	}
	if len(ca) == 0 {
		err = errors.New("token claim ca is empty")
	}
	return
}

func (t *CmdDaemonJoin) createTmpCertFile(b []byte) (certFile string, err error) {
	var (
		tmpFile *os.File
	)
	tmpFile, err = os.CreateTemp("", "cert.pem")
	if err != nil {
		return
	}
	certFile = tmpFile.Name()
	defer func(name string) {
		_ = tmpFile.Close()
	}(certFile)

	_, err = tmpFile.Write(b)
	if err != nil {
		defer func(name string) {
			_ = os.Remove(certFile)
		}(certFile)
	}
	return
}
