package daemonapi

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/cluster"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/daemon/daemonauth"
	"github.com/opensvc/om3/v3/daemon/daemonenv"
	"github.com/opensvc/om3/v3/daemon/daemonsubsystem"
	"github.com/opensvc/om3/v3/daemon/rbac"
	"github.com/opensvc/om3/v3/util/converters"
)

// enrollDefaultTimeout is the lifetime of the join token handed to the
// enrolled node when the body sets no timeout. It has to outlive the node
// drain, which has no bounded duration.
const enrollDefaultTimeout = time.Hour

// PostClusterEnroll orders a foreign node to leave its cluster and join ours.
//
// The candidate node is only reachable with the credentials the requester
// provides: an access token with the join role, created on that node. Its ca
// claim is the trust anchor for its listener certificate, and only a token
// carrying that role holds the claim.
//
// The candidate node must be a single node cluster. Enrolling a node that
// still has peers is refused: nothing in the join flow tells them to drop it
// from their cluster.nodes, so they would keep it forever.
func (a *DaemonAPI) PostClusterEnroll(ctx echo.Context) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	log := LogHandler(ctx, "PostClusterEnroll")

	var payload api.ClusterEnrollBody
	if err := ctx.Bind(&payload); err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid body", "%s", err)
	}
	if payload.Node == "" {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid body", "missing 'node' field")
	}
	if payload.Token == "" {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid body", "missing 'token' field")
	}
	timeout := enrollDefaultTimeout
	if payload.Timeout != nil && *payload.Timeout != "" {
		v, err := converters.Duration.Convert(*payload.Timeout)
		if err != nil {
			return JSONProblemf(ctx, http.StatusBadRequest, "Invalid body",
				"field 'timeout' with value '%s' validation error: %s", *payload.Timeout, err)
		}
		if d := *v.(*time.Duration); d > 0 {
			timeout = d
		}
	}

	// The enrolled node has to verify our certificate when it dials us, so
	// check now that it can. A mismatch would otherwise only surface later,
	// inside the join forked on that node, where the operator does not see it.
	joinAddr := ""
	if payload.JoinAddr != nil && *payload.JoinAddr != "" {
		if err := verifyJoinAddr(*payload.JoinAddr); err != nil {
			log.Infof("enroll %s refused: join addr %s: %s", payload.Node, *payload.JoinAddr, err)
			return JSONProblemf(ctx, http.StatusBadRequest, "Invalid body",
				"field 'join_addr' with value '%s': %s", *payload.JoinAddr, err)
		}
		joinAddr = *payload.JoinAddr
	} else {
		joinAddr = a.daemonAddrToJoin()
		log.Debugf("detect join addr: %s", joinAddr)
	}

	// The ca claim of the candidate node token is what lets us verify its
	// certificate: its cluster is not ours, so our own truststore is useless
	// here, and a proxy client (which requires a cluster node) can not be used.
	ca, err := caFromToken(payload.Token)
	if err != nil {
		log.Infof("enroll %s refused: %s", payload.Node, err)
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid body",
			"field 'token' has no usable 'ca' claim: %s (create it with the join role)", err)
	}
	certFile, err := tmpCertFile(ca)
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Server error", "write ca file: %s", err)
	}
	defer func() { _ = os.Remove(certFile) }()

	cli, err := client.New(
		client.WithURL(payload.Node),
		client.WithRootCa(certFile),
		client.WithBearer(payload.Token),
	)
	if err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid body",
			"field 'node' with value '%s': %s", payload.Node, err)
	}

	// This request doubles as the preflight: a 200 proves the candidate node
	// is reachable, the token is valid there, and its ca claim does anchor its
	// certificate. Do it before creating a join token for a request we may
	// still refuse.
	nodes, err := a.enrollCandidateNodes(ctx, cli)
	if err != nil {
		log.Infof("enroll %s refused: %s", payload.Node, err)
		return JSONProblemf(ctx, http.StatusBadGateway, "Candidate node",
			"get cluster nodes of '%s': %s", payload.Node, err)
	}
	switch {
	case len(nodes) == 0:
		return JSONProblemf(ctx, http.StatusConflict, "Candidate node",
			"node '%s' has an empty cluster.nodes: refusing to enroll a node in an unexpected state", payload.Node)
	case len(nodes) > 1:
		log.Infof("enroll %s refused: still a member of a %d nodes cluster", payload.Node, len(nodes))
		return JSONProblemf(ctx, http.StatusConflict, "Candidate node",
			"node '%s' is a member of a %d nodes cluster (%s): make it a single node cluster first, "+
				"else its peers would keep it in their cluster.nodes",
			payload.Node, len(nodes), strings.Join(nodes, ", "))
	}
	candidate := nodes[0]
	if candidate == a.localhost {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid body",
			"node '%s' is localhost", candidate)
	}

	// The join grant makes xClaimForGrants add our certificate chain as the ca
	// claim, which is what the candidate node needs to trust our listener.
	username := userFromContext(ctx).Username
	joinToken, err := a.createAccessTokenWithGrants(username, timeout, daemonauth.TkUseAccess,
		[]string{rbac.GrantJoin.String()})
	if err != nil {
		log.Errorf("create join token for %s: %s", candidate, err)
		return JSONProblemf(ctx, http.StatusInternalServerError, "Server error", "create join token: %s", err)
	}

	timeoutStr := timeout.String()
	joinBody := api.PostDaemonJoinJSONRequestBody{
		Node:    a.localhost,
		Token:   joinToken.AccessToken,
		Timeout: &timeoutStr,
	}

	if joinAddr != "" {
		joinBody.Addr = &joinAddr
	}

	log.Infof("order node %s to join our cluster", candidate)
	resp, err := cli.PostDaemonJoinWithResponse(ctx.Request().Context(), candidate, joinBody)
	if err != nil {
		log.Warnf("post daemon join on %s: %s", candidate, err)
		return JSONProblemf(ctx, http.StatusBadGateway, "Candidate node",
			"post daemon join on '%s': %s", candidate, err)
	}
	if code := resp.StatusCode(); code != http.StatusOK {
		detail := strings.TrimSpace(string(resp.Body))
		log.Warnf("post daemon join on %s: got %d: %s", candidate, code, detail)
		// Relay a refusal from the candidate node under its own status: it
		// re-checks what we checked, and a 409 it raises between our check and
		// this post is the same conflict, not a gateway error.
		if code >= 400 && code < 500 {
			return JSONProblemf(ctx, code, "Candidate node",
				"node '%s' refused the join order: %s", candidate, detail)
		}
		return JSONProblemf(ctx, http.StatusBadGateway, "Candidate node",
			"post daemon join on '%s': got %d wanted %d: %s", candidate, code, http.StatusOK, detail)
	}
	log.Infof("node %s accepted the join order", candidate)
	return ctx.JSON(http.StatusOK, api.ClusterEnrollAccepted{Node: candidate})
}

// enrollCandidateNodes returns the cluster.nodes of the node cli is
// connected to.
func (a *DaemonAPI) enrollCandidateNodes(ctx echo.Context, cli *client.T) ([]string, error) {
	evaluate := true
	kw := api.InQueryKeywords{"cluster.nodes"}
	params := api.GetClusterConfigParams{
		Evaluate: &evaluate,
		Kw:       &kw,
	}
	resp, err := cli.GetClusterConfigWithResponse(ctx.Request().Context(), &params)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("got %d wanted %d: %s", resp.StatusCode(), http.StatusOK, strings.TrimSpace(string(resp.Body)))
	}
	if resp.JSON200 == nil || len(resp.JSON200.Items) == 0 {
		return nil, fmt.Errorf("no cluster.nodes item in response")
	}
	item := resp.JSON200.Items[0]
	if item.Evaluated != nil {
		if l, ok := anyToStrings(*item.Evaluated); ok {
			return l, nil
		}
	}
	return strings.Fields(item.Value), nil
}

// anyToStrings converts the evaluated value of a list keyword, which reaches
// us as a json decoded []any.
func anyToStrings(i any) ([]string, bool) {
	switch v := i.(type) {
	case []string:
		return v, true
	case string:
		return strings.Fields(v), true
	case []any:
		l := make([]string, 0, len(v))
		for _, e := range v {
			s, ok := e.(string)
			if !ok {
				return nil, false
			}
			l = append(l, s)
		}
		return l, true
	}
	return nil, false
}

// daemonAddrToJoin returns the location the enrolled node should reach us at,
// picked from the names our certificate is valid for, so that node does not
// have to resolve our nodename and its tls client can verify us.
//
// A name designating us is preferred over any other. The certificate is a
// cluster-wide object, so the plain names it carries are usually those of the
// node that bootstrapped the cluster: returning one of those would send the
// enrolled node to a peer instead of to us. A wildcard is expanded with our
// own label, which makes a cluster-wide certificate designate each node of the
// cluster in turn.
//
// The loopback address our certificate always carries is skipped: the enrolled
// node cannot reach us there. An empty string leaves the join to the nodename.
func (a *DaemonAPI) daemonAddrToJoin() string {
	cert, err := leafCert()
	if err != nil {
		return ""
	}
	fallback := ""
	for _, name := range certNames(cert) {
		if isLoopback(name) {
			continue
		}
		// Expanding a wildcard can produce a name the certificate does not
		// actually match, so confirm before handing it over.
		if ours := a.ourCertName(name); ours != "" && cert.VerifyHostname(ours) == nil {
			return joinURL(ours, a.localPort())
		}
		if fallback == "" {
			fallback = name
		}
	}
	if fallback == "" {
		return ""
	}
	// This name designates a peer, so the port our listener bound says nothing
	// about it: only the cluster-wide configuration does.
	return joinURL(fallback, clusterPort())
}

// ourCertName returns name as it designates us, or an empty string when it
// designates another node.
//
// A wildcard is expanded with our own first label: x509 matches a wildcard
// against a single label, so 'node2.example.com' is what '*.example.com'
// designates on node2, whether our nodename is spelled short or fully
// qualified.
func (a *DaemonAPI) ourCertName(name string) string {
	label, _, _ := strings.Cut(a.localhost, ".")
	if suffix, ok := strings.CutPrefix(name, "*."); ok {
		if label == "" || suffix == "" {
			return ""
		}
		return label + "." + suffix
	}
	if name == a.localhost || (label != "" && name == label) {
		return name
	}
	return ""
}

// verifyJoinAddr reports whether the enrolled node will be able to verify our
// certificate when it dials addr.
//
// VerifyHostname is the function the tls client of that node runs, so this
// predicts its result instead of guessing it: wildcards, ip names and the
// common name being ignored are all handled the same way there and here.
func verifyJoinAddr(addr string) error {
	host, err := joinAddrHost(addr)
	if err != nil {
		return err
	}
	if isLoopback(host) {
		return fmt.Errorf("'%s' is a loopback address: the enrolled node can not reach us there", host)
	}
	cert, err := leafCert()
	if err != nil {
		return fmt.Errorf("read our certificate: %w", err)
	}
	if err := cert.VerifyHostname(host); err != nil {
		return fmt.Errorf("%w (our certificate is valid for %s)", err, strings.Join(certNames(cert), ", "))
	}
	return nil
}

// joinAddrHost returns the host of a [<scheme>://]<addr>[:<port>] location.
func joinAddrHost(addr string) (string, error) {
	s := addr
	if !strings.Contains(s, "://") {
		s = "https://" + s
	}
	u, err := url.Parse(s)
	if err != nil {
		return "", err
	}
	host := u.Hostname()
	if host == "" {
		return "", fmt.Errorf("no host in '%s'", addr)
	}
	return host, nil
}

// joinURL formats host and port as a location the enrolled node can be handed.
func joinURL(host, port string) string {
	if ip := net.ParseIP(host); ip != nil && ip.To4() == nil {
		host = "[" + host + "]"
	}
	return daemonenv.HTTPNodeAndPortURL(host, port)
}

// localPort returns the port the enrolled node has to reach us at.
//
// The port our listener actually bound is preferred: a daemon not restarted
// since the last configuration change is not serving the configured one. It
// only tells us about ourselves, so it is of no use for a peer.
func (a *DaemonAPI) localPort() string {
	if lsnr := daemonsubsystem.DataListener.Get(a.localhost); lsnr != nil && lsnr.Port != "" {
		return lsnr.Port
	}
	return clusterPort()
}

// clusterPort returns the configured listener port, then the default one a
// cluster is free not to use. It is what a name designating a peer has to be
// reached at, the port that peer bound being its own and unknown to us.
func clusterPort() string {
	if cluster.ConfigData.IsSet() {
		if port := cluster.ConfigData.Get().Listener.Port; port != 0 {
			return fmt.Sprint(port)
		}
	}
	return fmt.Sprint(daemonenv.HTTPPort)
}

// leafCert returns the certificate our listener serves. ServeTLS loads the
// chain file as a key pair, which puts the leaf first.
func leafCert() (*x509.Certificate, error) {
	filename := daemonenv.CertChainFile()
	b, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	for len(b) > 0 {
		var block *pem.Block
		block, b = pem.Decode(b)
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" {
			continue
		}
		return x509.ParseCertificate(block.Bytes)
	}
	return nil, fmt.Errorf("no certificate in %s", filename)
}

// certNames returns the names cert is valid for, in the spelling x509 matches
// a dialed host against.
func certNames(cert *x509.Certificate) []string {
	l := append([]string{}, cert.DNSNames...)
	for _, ip := range cert.IPAddresses {
		l = append(l, ip.String())
	}
	return l
}

func isLoopback(host string) bool {
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback()
	}
	return host == "localhost"
}
