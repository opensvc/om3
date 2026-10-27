package omcmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/clusterdump"
	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/core/env"
	"github.com/opensvc/om3/v3/core/event"
	"github.com/opensvc/om3/v3/core/keyop"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/node"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/daemon/daemonenv"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/daemon/remoteconfig"
	"github.com/opensvc/om3/v3/util/file"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/key"
)

type (
	CmdClusterJoin struct {
		CmdDaemonCommon

		Node string

		// TokenFile is the path of a file holding an access token with the
		// join role, created on the node to join. When empty, the
		// OSVC_JOIN_TOKEN environment variable is used.
		TokenFile string

		// token is the join token, resolved from TokenFile or from the
		// environment by checkParams.
		token string

		// Addr is the location of the Node api, in the
		// [<scheme>://]<addr>[:<port>] format. When empty, Node is used.
		// Set it when this node cannot resolve Node.
		Addr string

		// Timeout is the maximum duration for join
		Timeout time.Duration
	}
)

var (
	ErrCmdClusterJoin = errors.New("command cluster join")

	// objectDiscoverTimeout bounds the wait for the object configurations
	// the daemon installs from its peers after the join, before the checks
	// and disks pushes.
	objectDiscoverTimeout = 5 * time.Minute

	// objectDiscoverInterval is the interval between the checks of the
	// object configurations installed.
	objectDiscoverInterval = 2 * time.Second
)

func (t *CmdClusterJoin) Run() error {
	t.setLogger("cluster join: ")
	err := t.run()
	if err != nil {
		t.logger().Errorf("%s", err)
		return fmt.Errorf("%w: %w", ErrCmdClusterJoin, err)
	}
	return nil
}

func (t *CmdClusterJoin) run() error {
	var (
		certFile string
		cli      *client.T
	)
	if err := t.checkParams(); err != nil {
		return err
	}
	// The join moves the node config aside: read the collector registration
	// id before, for the api node to register this node with it.
	prevUUID := localNodeUUID()
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
	if t.Addr != "" {
		url = t.Addr
	}
	cli, err = client.New(
		client.WithURL(url),
		client.WithRootCa(certFile),
		client.WithBearer(t.token),
	)
	if err != nil {
		return err
	}

	t.logger().Infof("Fetch cluster config from %s", t.Node)
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
		"JoinSuccess,added_node=" + localhost,
		"JoinError,candidate_node=" + localhost,
		"JoinIgnored,candidate_node=" + localhost,
	}
	ctx, cancel := context.WithTimeout(context.Background(), t.Timeout)
	defer cancel()

	// The stream has a client of its own, with no timeout: a client timeout
	// bounds the whole answer, the body read included, and would cut the
	// stream after it. The context bounds it instead.
	streamClient, err := client.New(
		client.WithURL(url),
		client.WithRootCa(certFile),
		client.WithBearer(t.token),
		client.WithTimeout(0),
	)
	if err != nil {
		return err
	}
	evReader, err := streamClient.NewGetEvents().
		SetRelatives(false).
		SetFilters(filters).
		SetDuration(t.Timeout).
		GetReader(ctx)

	if err != nil {
		return err
	}
	defer func() {
		_ = evReader.Close()
	}()

	t.logger().Infof("Add localhost node to the remote cluster configuration on %s", t.Node)
	t.logger().Infof("Daemon join")
	params := api.PostClusterJoinParams{
		Node: hostname.Hostname(),
	}
	if resp, err := cli.PostClusterJoin(context.Background(), &params); err != nil {
		return fmt.Errorf("%w: %w", commoncmd.ErrClientRequest, err)
	} else if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: got %d wanted %d", commoncmd.ErrClientStatusCode, resp.StatusCode, http.StatusOK)
	}

	if err := t.waitJoinResult(ctx, evReader); err != nil {
		return fmt.Errorf("wait join result: %w", err)
	}
	err = t.onJoined(ctx, cli, prevUUID)
	if err != nil {
		return fmt.Errorf("on joined: %w", err)
	}
	return err
}

func (t *CmdClusterJoin) checkParams() error {
	if t.Node == "" {
		return fmt.Errorf("%w: node is empty", commoncmd.ErrFlagInvalid)
	}
	// A token on the command line is readable by any user through the process
	// table, so --token names a file, and the daemon hands the token to the
	// join it forks over the environment.
	token, err := commoncmd.SecretFromFileOrEnv(t.TokenFile, env.JoinTokenVar)
	if err != nil {
		return fmt.Errorf("%w: --token: %w", commoncmd.ErrFlagInvalid, err)
	}
	if token == "" {
		return fmt.Errorf("%w: token is empty: use --token or env %s", commoncmd.ErrFlagInvalid, env.JoinTokenVar)
	}
	t.token = token
	return nil
}

func (t *CmdClusterJoin) extractCaClaim() (ca []byte, err error) {
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

	// Parse the token without verifying the signature to extract the CA certificate.
	// This is part of a secure bootstrap trust mechanism:
	// 1. The CA certificate is used only to establish TLS trust with the cluster node
	// 2. The same token is sent as a Bearer token and validated server-side with signature verification
	// 3. The server validates the JWT signature (daemon/daemonauth/jwt.go) using the cluster's RSA public key
	// 4. The server checks for RoleJoin grant (daemon/daemonapi/post_cluster_join.go) before processing the join request
	// 5. No privileged operations occur until server-side signature validation succeeds
	// The CA extraction is safe because authentication happens server-side, not client-side.
	token, _, err = parser.ParseUnverified(t.token, &joinClaim{})
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

func (t *CmdClusterJoin) createTmpCertFile(b []byte) (certFile string, err error) {
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

func (t *CmdClusterJoin) onJoined(ctx context.Context, cli *client.T, prevUUID string) (err error) {
	filePaths := make(map[string]naming.Path)
	fileTimestamps := make(map[string]time.Time)
	toFetch := []naming.Path{
		naming.Cluster,
		naming.SecCa,
		naming.SecCert,
		naming.SecHb,
	}
	downloadedFiles := make([]string, 0)
	defer func() {
		for _, f := range downloadedFiles {
			_ = os.Remove(f)
		}
	}()

	fetchObjectConfigData := make(map[naming.Path][]byte)
	for _, p := range toFetch {
		var (
			file      string
			timestamp time.Time
		)
		t.logger().Infof("Fetch %s from %s", p, t.Node)
		file, timestamp, err = remoteconfig.FetchObjectConfigFile(cli, p)
		if err != nil {
			return fmt.Errorf("%w: for path %s: %w", commoncmd.ErrFetchFile, p, err)
		}
		downloadedFiles = append(downloadedFiles, file)
		fetchObjectConfigData[p], err = os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("%w: for path %s: %w", commoncmd.ErrFetchFile, p, err)
		}
		filePaths[file] = p
		fileTimestamps[file] = timestamp
	}

	if t.isRunning() {
		if err := t.nodeDrain(ctx); err != nil {
			return err
		}
		t.logger().Infof("Stop daemon")
		if err := (&CmdDaemonStop{}).Run(); err != nil {
			return err
		}
	}

	if err := t.backupLocalConfig(".pre-cluster-join-etc"); err != nil {
		return err
	}

	// The config is gone: the daemon is to start with the fetched one
	// whatever the cleanup left, so the node joins.
	cleanupErr := t.cleanupVarDir()
	if err := rawconfig.CreateMandatoryDirectories(); err != nil {
		return err
	}

	for fileName, p := range filePaths {
		t.logger().Infof("Install fetched config %s with timestamp %s", p, fileTimestamps[fileName])
		configFile := p.ConfigFile()
		dir := filepath.Dir(configFile)
		if err := os.MkdirAll(dir, os.ModePerm); err != nil {
			return fmt.Errorf("%w: config %s from file %s: %w", commoncmd.ErrInstallFile, p, fileName, err)
		}
		if err := os.WriteFile(configFile, fetchObjectConfigData[p], 0600); err != nil {
			return fmt.Errorf("%w: config %s from file %s: %w", commoncmd.ErrInstallFile, p, fileName, err)
		}
		if err := file.Touch(configFile, fileTimestamps[fileName]); err != nil {
			return fmt.Errorf("%w: config %s set file timestamp %s: %w", commoncmd.ErrInstallFile, p, configFile, err)
		}
		if err := file.Sync(configFile); err != nil {
			return fmt.Errorf("%w: config %s sync file %s: %w", commoncmd.ErrInstallFile, p, fileName, err)
		}
	}

	// Not bound to the join timeout, which a long drain may have used up:
	// the client timeout bounds the request.
	registered := t.collectorRegister(context.Background(), cli, prevUUID)

	t.logger().Infof("Start daemon")
	if err := (&CmdDaemonStart{}).Run(); err != nil {
		return err
	}

	// After the daemon start, which the inventory collection would delay.
	if registered {
		t.collectorPush(context.Background())
	}
	t.logger().Infof("Joined")
	if cleanupErr != nil {
		return fmt.Errorf("cleanup opensvc var dir: %w", cleanupErr)
	}
	return nil
}

func (t *CmdClusterJoin) waitJoinResult(ctx context.Context, evReader event.Reader) error {
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
			msg, err := msgbus.EventToMessage(*ev)
			if err != nil {
				return err
			}
			switch msg.(type) {
			case *msgbus.JoinSuccess:
				t.logger().Infof("Cluster nodes updated")
				return nil
			case *msgbus.JoinError:
				err := fmt.Errorf("join error event %s", ev.Data)
				return err
			case *msgbus.JoinIgnored:
				// TODO parse Reason
				t.logger().Infof("Join ignored: %s", ev.Data)
				return nil
			default:
				return fmt.Errorf("%w: %s data: %v", commoncmd.ErrEventKindUnexpected, ev.Kind, ev.Data)
			}
		}
	}
}

// localNodeUUID returns the collector registration id of the node, empty when
// it has none.
func localNodeUUID() string {
	n, err := object.NewNode()
	if err != nil {
		return ""
	}
	return n.Config().GetString(key.Parse("node.uuid"))
}

// collectorRegister has the api node register this node on the collector
// with the registration id it held before the join, and stores the new one
// in the node config, so the daemon speaks to the collector as soon as it
// starts.
//
// The join does not depend on the collector: a node without a registration
// id is left as it is, and a refused one is warned to register itself.
//
// It returns true when the node is registered.
func (t *CmdClusterJoin) collectorRegister(ctx context.Context, cli *client.T, prevUUID string) bool {
	if prevUUID == "" {
		return false
	}
	t.logger().Infof("Register on the collector through %s", t.Node)
	if err := t.collectorRegisterThroughAPINode(ctx, cli, prevUUID); err != nil {
		t.logger().Warnf("register on the collector: %s: register with 'om node register'", err)
		return false
	}
	return true
}

// collectorPush sends the collector the initial inventory 'om node register'
// sends, so the collector knows the node registered by the join as well.
//
// The checks and disks pushes replace what the collector holds of the node
// with what the node reports of its svc and vol objects, which the daemon
// installs from its peers after it starts: they wait for these objects, or
// are left to the scheduled pushes, not to delete the checks and disk claims
// of the objects not installed yet.
//
// The join does not depend on the collector: a failure is warned, and left
// to the scheduled pushes.
func (t *CmdClusterJoin) collectorPush(ctx context.Context) {
	n, err := object.NewNode()
	if err != nil {
		t.logger().Warnf("push the node inventory to the collector: %s: left to the scheduled pushes", err)
		return
	}
	t.logger().Infof("Wait for the object configurations to push the node inventory to the collector")
	waitCtx, cancel := context.WithTimeout(ctx, objectDiscoverTimeout)
	defer cancel()
	if err := waitObjectsDiscovered(waitCtx); err != nil {
		t.logger().Warnf("%s: push the node inventory to the collector but the checks and disks, left to the scheduled pushes", err)
		t.pushObjectFreeInventory(n)
		return
	}
	t.logger().Infof("Push the node inventory to the collector")
	if err := n.PushRegistered(ctx); err != nil {
		t.logger().Warnf("push the node inventory to the collector: %s: left to the scheduled pushes", err)
	}
}

// pushObjectFreeInventory sends the collector the part of the initial
// inventory that does not depend on the objects installed.
func (t *CmdClusterJoin) pushObjectFreeInventory(n *object.Node) {
	if _, err := n.PushAsset(); err != nil {
		t.logger().Warnf("push asset: %s", err)
	}
	if _, err := n.PushPkg(); err != nil {
		t.logger().Warnf("push pkg: %s", err)
	}
	if err := n.Sysreport(); err != nil {
		t.logger().Warnf("sysreport: %s", err)
	}
}

// waitObjectsDiscovered waits for the local daemon to have installed the
// configurations of the svc and vol objects scoped on this node.
func waitObjectsDiscovered(ctx context.Context) error {
	c, err := client.New()
	if err != nil {
		return err
	}
	ticker := time.NewTicker(objectDiscoverInterval)
	defer ticker.Stop()
	for {
		missing, err := objectsNotDiscovered(c)
		if err == nil && len(missing) == 0 {
			return nil
		}
		select {
		case <-ctx.Done():
			if err != nil {
				return fmt.Errorf("wait for the object configurations: %w: %w", ctx.Err(), err)
			}
			return fmt.Errorf("wait for the object configurations: %w: missing %s", ctx.Err(), strings.Join(missing, ", "))
		case <-ticker.C:
		}
	}
}

// objectsNotDiscovered returns the svc and vol objects scoped on this node
// whose configuration is not installed yet.
//
// The daemon knows the objects of the cluster from the data of its peers, so
// the answer only holds once it has left the rejoin state: it has the data of
// every peer, or has stopped waiting for the ones that did not answer.
//
// The object statuses are built from that data by the object monitors,
// a moment after the daemon leaves the rejoin state, so the instance
// configurations of the peers are read too: an object is not taken for
// discovered for not having a status yet.
func objectsNotDiscovered(c *client.T) ([]string, error) {
	b, err := c.NewGetClusterStatus().Get()
	if err != nil {
		return nil, err
	}
	var data clusterdump.Data
	if err := json.Unmarshal(b, &data); err != nil {
		return nil, err
	}
	localhost := hostname.Hostname()
	switch data.Cluster.Node[localhost].Monitor.State {
	case node.MonitorStateInit, node.MonitorStateRejoin:
		return nil, fmt.Errorf("the daemon is rejoining its peers")
	}
	installed, err := naming.InstalledPaths()
	if err != nil {
		return nil, err
	}
	return notInstalledObjects(objectScopes(data), installed, localhost), nil
}

// objectScopes returns the scope of each object of the cluster status, from
// the object statuses and from the instance configurations of every node.
func objectScopes(data clusterdump.Data) map[string][]string {
	m := make(map[string][]string)
	for s, o := range data.Cluster.Object {
		m[s] = append(m[s], o.Scope...)
	}
	for _, n := range data.Cluster.Node {
		for s, i := range n.Instance {
			if i.Config != nil {
				m[s] = append(m[s], i.Config.Scope...)
			}
		}
	}
	return m
}

// notInstalledObjects returns the svc and vol objects of scopes, the objects
// by path and the nodes they are scoped on, that are scoped on nodename and
// not in installed.
func notInstalledObjects(scopes map[string][]string, installed naming.Paths, nodename string) []string {
	installedMap := installed.StrMap()
	l := make([]string, 0)
	for s, scope := range scopes {
		p, err := naming.ParsePath(s)
		if err != nil || (p.Kind != naming.KindSvc && p.Kind != naming.KindVol) {
			continue
		}
		if !slices.Contains(scope, nodename) || installedMap.Has(s) {
			continue
		}
		l = append(l, s)
	}
	sort.Strings(l)
	return l
}

func (t *CmdClusterJoin) collectorRegisterThroughAPINode(ctx context.Context, cli *client.T, prevUUID string) error {
	body := api.ClusterJoinRegisterBody{Node: hostname.Hostname(), Uuid: prevUUID}
	resp, err := cli.PostClusterJoinRegisterWithResponse(ctx, body)
	if err != nil {
		return fmt.Errorf("%w: %w", commoncmd.ErrClientRequest, err)
	}
	if resp.JSON200 == nil {
		var problem api.Problem
		_ = json.Unmarshal(resp.Body, &problem)
		return fmt.Errorf("%w: %s: %s", commoncmd.ErrClientStatusCode, resp.Status(), problem.Detail)
	}
	return storeCollectorRegistration(resp.JSON200.Uuid)
}

// storeCollectorRegistration sets the collector registration id of the node
// in the node config.
func storeCollectorRegistration(nodeUUID string) error {
	n, err := object.NewNode()
	if err != nil {
		return err
	}
	if err := n.Config().Set(*keyop.New(key.Parse("node.uuid"), keyop.Set, nodeUUID, 0)); err != nil {
		return fmt.Errorf("store the collector registration: %w", err)
	}
	return nil
}
