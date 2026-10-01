package console

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/golang-jwt/jwt/v5"

	"github.com/opensvc/om3/v3/core/console"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/nodesinfo"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/daemon/daemonenv"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/key"
	"github.com/opensvc/om3/v3/util/plog"
)

// DefaultPort is the port the console listener listens on.
const DefaultPort = 1216

// Dir is where the console sessions of the node keep what they share.
func Dir() string {
	return filepath.Join(rawconfig.Paths.Var, "console")
}

// SessionDir is where the running sessions are recorded.
func SessionDir() string {
	return filepath.Join(Dir(), "sessions")
}

// Port returns the console port of the node configuration.
func Port() int {
	node, err := object.NewNode()
	if err != nil {
		return DefaultPort
	}
	if port := node.MergedConfig().GetInt(key.New("console", "port")); port > 0 {
		return port
	}
	return DefaultPort
}

// ServeInherited serves the console session of the connection the process
// inherited from the daemon.
func ServeInherited() error {
	log := plog.NewDefaultLogger().Attr("pkg", "daemon/console").WithPrefix("console: ")
	err := serveInherited(log)
	if err != nil {
		// Nobody reads what this process prints: the log is where a
		// session that could not be served is said.
		log.Errorf("%s", err)
	}
	return err
}

func serveInherited(log *plog.Logger) error {
	connFile := os.NewFile(sessionConnFD, "console connection")
	if connFile == nil {
		return errors.New("no console connection inherited")
	}
	conn, err := net.FileConn(connFile)
	_ = connFile.Close()
	if err != nil {
		return fmt.Errorf("console connection: %w", err)
	}
	defer conn.Close()
	authFile := os.NewFile(sessionAuthFD, "console authentication")

	cert, err := tls.LoadX509KeyPair(daemonenv.CertChainFile(), daemonenv.KeyFile())
	if err != nil {
		return fmt.Errorf("console certificate: %w", err)
	}
	b, err := os.ReadFile(daemonenv.CAsCertFile())
	if err != nil {
		return fmt.Errorf("console ticket verify key: %w", err)
	}
	verifyKey, err := jwt.ParseRSAPublicKeyFromPEM(b)
	if err != nil {
		return fmt.Errorf("console ticket verify key: %w", err)
	}
	port := Port()

	return Serve(conn, Config{
		Hostname:    hostname.Hostname(),
		Certificate: cert,
		VerifyKey:   verifyKey,
		TicketDir:   filepath.Join(Dir(), "tickets"),
		SessionDir:  SessionDir(),
		Command:     enterCommand,
		PeerURL: func(node string) (string, error) {
			return peerURL(node, port)
		},
		// A peer is reached as the api reaches it: its certificate is
		// not verified, the ticket relayed to it being of no use to
		// anyone once the peer has marked it used.
		PeerTLSConfig: &tls.Config{InsecureSkipVerify: true},
		Authenticated: func() {
			if authFile != nil {
				_ = authFile.Close()
			}
		},
		Log: log,
	})
}

// enterCommand is the command entering the resource the session targets.
//
// The target comes from a ticket this cluster signed, and is checked anyway:
// it becomes the arguments of a command.
func enterCommand(target console.Target) ([]string, error) {
	path, err := naming.ParsePath(target.Path)
	if err != nil {
		return nil, fmt.Errorf("console target: %w", err)
	}
	args := []string{os.Args[0], path.String(), "enter"}
	if target.RID != "" {
		if strings.HasPrefix(target.RID, "-") || strings.ContainsAny(target.RID, " \t\n") {
			return nil, fmt.Errorf("console target: invalid resource id %q", target.RID)
		}
		args = append(args, "--rid", target.RID)
	}
	return args, nil
}

// peerURL is the url of the console endpoint of a peer node: the address its
// api listens on, and the console port, which is the same on every node of
// the cluster.
func peerURL(node string, port int) (string, error) {
	addr := node
	if nodesInfo, err := nodesinfo.Load(); err == nil {
		info, ok := nodesInfo[node]
		if !ok {
			return "", fmt.Errorf("%s is not a node of this cluster", node)
		}
		switch a := info.Lsnr.Addr; a {
		case "", "::", "0.0.0.0":
		default:
			addr = a
		}
	}
	return "wss://" + net.JoinHostPort(addr, fmt.Sprint(port)) + "/", nil
}
