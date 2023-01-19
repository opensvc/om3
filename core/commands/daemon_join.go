package commands

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/pkg/errors"

	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/core/client/api"
	"opensvc.com/opensvc/core/event"
	"opensvc.com/opensvc/core/kind"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/daemon/daemonauth"
	"opensvc.com/opensvc/daemon/daemonenv"
	"opensvc.com/opensvc/daemon/msgbus"
	"opensvc.com/opensvc/daemon/remoteconfig"
	"opensvc.com/opensvc/util/command"
	"opensvc.com/opensvc/util/hostname"
)

type (
	CmdDaemonJoin struct {
		Node  string
		Token string
	}
)

func (t *CmdDaemonJoin) Run() error {
	var (
		certFile string
		cli      *client.T
	)
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
		client.WithBearer(t.Token),
	)
	if err != nil {
		return err
	}

	_, _ = fmt.Fprintf(os.Stderr, "Fetch cluster config from %s\n", t.Node)
	file, _, err := remoteconfig.FetchObjectFile(cli, path.Cluster)
	if err != nil {
		return err
	}
	defer func() {
		_ = os.Remove(file)
	}()
	clusterCfg, err := object.NewCcfg(path.Cluster, object.WithConfigFile(file))
	if err != nil {
		return err
	}
	clusterName := clusterCfg.Name()

	localhost := hostname.Hostname()
	filters := []string{
		"JoinSuccess,added=" + localhost + ",node=" + t.Node,
		"JoinError,join-node=" + localhost,
		"JoinIgnored,join-node=" + localhost,
	}
	duration := 5 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	evReader, err := cli.NewGetEvents().
		SetRelatives(false).
		SetFilters(filters).
		SetDuration(duration).
		GetReader()

	if err != nil {
		return err
	}
	defer func() {
		_ = evReader.Close()
	}()

	_, _ = fmt.Fprintf(os.Stderr, "Add localhost node to the remote cluster configuration on %s\n", t.Node)
	join := api.NewPostDaemonJoin(cli)
	join.SetNode(hostname.Hostname())
	_, _ = fmt.Fprintf(os.Stderr, "Daemon join\n")
	if b, err := join.Do(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Daemon join %s error %s: %s\n", t.Node, err, b)
	}

	if err := t.waitJoinResult(ctx, evReader, duration) ; err != nil {
		return err
	}
	err = t.onJoined(cli, clusterName)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "onJoined error: %s\n", err)
	}
	return err
}

func (t *CmdDaemonJoin) checkParams() error {
	if t.Node == "" {
		return errors.New("need a cluster node to join cluster")
	}
	if t.Token == "" {
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

	token, _, err = parser.ParseUnverified(t.Token, &joinClaim{})
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

func (t *CmdDaemonJoin) onJoined(cli *client.T, clusterName string) (err error) {
	filePaths := make(map[string]path.T)
	toFetch := []path.T{
		path.Cluster,
		{Namespace: "system", Kind: kind.Sec, Name: "ca-" + clusterName},
		{Namespace: "system", Kind: kind.Sec, Name: "cert-" + clusterName},
	}
	downloadedFiles := make([]string, 0)
	defer func([]string) {
		for _, f := range downloadedFiles {
			_ = os.Remove(f)
		}
	}(downloadedFiles)

	for _, p := range toFetch {
		var file string
		_, _ = fmt.Fprintf(os.Stderr, "Fetch %s from %s\n", p, t.Node)
		file, _, err = remoteconfig.FetchObjectFile(cli, p)
		if err != nil {
			return err
		}
		downloadedFiles = append(downloadedFiles, file)
		filePaths[file] = p
	}

	args := []string{"daemon", "stop"}
	cmd := command.New(
		command.WithName(os.Args[0]),
		command.WithArgs(args),
	)
	_, _ = fmt.Fprintf(os.Stderr, "Stop daemon\n")
	err = cmd.Run()
	if err != nil {
		return err
	}

	// TODO backup conf files before install files from remote

	for fileName, p := range filePaths {
		_, _ = fmt.Fprintf(os.Stderr, "Install fetched config %s\n", p)
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
	_, _ = fmt.Fprintf(os.Stderr, "Start daemon\n")
	err = cmd.Run()
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(os.Stderr, "Joined\n")
	return nil
}

func (t *CmdDaemonJoin) waitJoinResult(ctx context.Context, evReader event.Reader, duration time.Duration) error {
	for {
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			ev, err := evReader.Read()
			if err != nil {
				return err
			}
			switch ev.Kind {
			case msgbus.JoinSuccess{}.Kind():
				_, _ = fmt.Fprintf(os.Stderr, "cluster nodes updated\n")
				return nil
			case msgbus.JoinError{}.Kind():
				err := errors.Errorf("Join error: %s", ev.Data)
				return err
			case msgbus.JoinIgnored{}.Kind():
				// TODO parse Reason
				_, _ = fmt.Fprintf(os.Stderr, "Join ignored: %s", ev.Data)
				return nil
			default:
				return errors.Errorf("unexpected event %s %v", ev.Kind, ev.Data)
			}
		}
	}
}
