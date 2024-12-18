package omcmd

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/event"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/daemonenv"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/daemon/remoteconfig"
	"github.com/opensvc/om3/util/hostname"
)

type (
	CmdDaemonJoin struct {
		CmdDaemonCommon

		Node  string
		Token string

		// Timeout is the maximum duration for leave
		Timeout time.Duration
	}
)

var (
	ErrCmdDaemonJoin = errors.New("command daemon join")
)

func (t *CmdDaemonJoin) Run() error {
	err := t.run()
	if err != nil {
		return fmt.Errorf("%w: %w", ErrCmdDaemonJoin, err)
	}
	return nil
}

func (t *CmdDaemonJoin) run() error {
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

	url := daemonenv.HTTPNodeURL(t.Node)
	cli, err = client.New(
		client.WithURL(url),
		client.WithRootCa(certFile),
		client.WithBearer(t.Token),
	)
	if err != nil {
		return err
	}

	_, _ = fmt.Fprintf(os.Stdout, "Fetch cluster config from %s\n", t.Node)
	file, _, err := remoteconfig.FetchObjectConfigFile(cli, naming.Cluster)
	if err != nil {
		return err
	}
	defer func() {
		_ = os.Remove(file)
	}()
	if _, err := object.NewCluster(object.WithConfigFile(file)); err != nil {
		return err
	}

	localhost := hostname.Hostname()
	filters := []string{
		"JoinSuccess,added=" + localhost + ",node=" + t.Node,
		"JoinError,join-node=" + localhost,
		"JoinIgnored,join-node=" + localhost,
	}
	ctx, cancel := context.WithTimeout(context.Background(), t.Timeout)
	defer cancel()

	evReader, err := cli.NewGetEvents().
		SetRelatives(false).
		SetFilters(filters).
		SetDuration(t.Timeout).
		GetReader()

	if err != nil {
		return err
	}
	defer func() {
		_ = evReader.Close()
	}()

	_, _ = fmt.Fprintf(os.Stdout, "Add localhost node to the remote cluster configuration on %s\n", t.Node)
	_, _ = fmt.Fprintf(os.Stdout, "Daemon join\n")
	params := api.PostDaemonJoinParams{
		Node: hostname.Hostname(),
	}
	if resp, err := cli.PostDaemonJoin(context.Background(), &params); err != nil {
		return fmt.Errorf("%w: %w", ErrClientRequest, err)
	} else if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: got %d wanted %d", ErrClientStatusCode, resp.StatusCode, http.StatusOK)
	}

	if err := t.waitJoinResult(ctx, evReader); err != nil {
		return fmt.Errorf("wait join result: %w", err)
	}
	err = t.onJoined(ctx, cli)
	if err != nil {
		return fmt.Errorf("on joined: %w", err)
	}
	return err
}

func (t *CmdDaemonJoin) checkParams() error {
	if t.Node == "" {
		return fmt.Errorf("%w: node is empty", ErrFlagInvalid)
	}
	if t.Token == "" {
		return fmt.Errorf("%w: token is empty", ErrFlagInvalid)
	}
	return nil
}

func (t *CmdDaemonJoin) extractCaClaim() (ca []byte, err error) {
	type (
		joinClaim struct {
			Ca string `json:"ca"`
			*jwt.RegisteredClaims
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
		err = fmt.Errorf("invalid token claims")
	}
	if len(ca) == 0 {
		err = fmt.Errorf("token claim ca is empty")
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

func (t *CmdDaemonJoin) onJoined(ctx context.Context, cli *client.T) (err error) {
	filePaths := make(map[string]naming.Path)
	toFetch := []naming.Path{
		naming.Cluster,
		{Namespace: "system", Kind: naming.KindSec, Name: "ca"},
		{Namespace: "system", Kind: naming.KindSec, Name: "cert"},
	}
	downloadedFiles := make([]string, 0)
	defer func([]string) {
		for _, f := range downloadedFiles {
			_ = os.Remove(f)
		}
	}(downloadedFiles)

	for _, p := range toFetch {
		var file string
		_, _ = fmt.Fprintf(os.Stdout, "Fetch %s from %s\n", p, t.Node)
		file, _, err = remoteconfig.FetchObjectConfigFile(cli, p)
		if err != nil {
			return fmt.Errorf("%w: for path %s: %w", ErrFetchFile, p, err)
		}
		downloadedFiles = append(downloadedFiles, file)
		filePaths[file] = p
	}

	if t.isRunning() {
		if err := t.nodeDrain(ctx); err != nil {
			return err
		}
		_, _ = fmt.Fprintf(os.Stdout, "Stop daemon\n")
		if err := (&CmdDaemonStop{}).Run(); err != nil {
			return err
		}
	}

	if err := t.backupLocalConfig(".pre-daemon-join"); err != nil {
		return err
	}

	if err := t.deleteLocalConfig(); err != nil {
		return err
	}

	for fileName, p := range filePaths {
		_, _ = fmt.Fprintf(os.Stdout, "Install fetched config %s\n", p)
		err := os.Rename(fileName, p.ConfigFile())
		if err != nil {
			return fmt.Errorf("%w: config %s from file %s: %w", ErrInstallFile, p, fileName, err)
		}
	}

	_, _ = fmt.Fprintf(os.Stdout, "Start daemon\n")
	if err := (&CmdDaemonStart{}).Run(); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(os.Stdout, "Joined\n")
	return nil
}

func (t *CmdDaemonJoin) waitJoinResult(ctx context.Context, evReader event.Reader) error {
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
			case (&msgbus.JoinSuccess{}).Kind():
				_, _ = fmt.Fprintf(os.Stdout, "Cluster nodes updated\n")
				return nil
			case (&msgbus.JoinError{}).Kind():
				err := fmt.Errorf("join error event %s", ev.Data)
				return err
			case (&msgbus.JoinIgnored{}).Kind():
				// TODO parse Reason
				_, _ = fmt.Fprintf(os.Stdout, "Join ignored: %s", ev.Data)
				return nil
			default:
				return fmt.Errorf("%w: %s data: %v", ErrEventKindUnexpected, ev.Kind, ev.Data)
			}
		}
	}
}
