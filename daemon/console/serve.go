package console

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/opensvc/om3/v3/core/console"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/nodesinfo"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/core/vpath"
	"github.com/opensvc/om3/v3/daemon/daemonenv"
	"github.com/opensvc/om3/v3/daemon/listener/tlssecs"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/key"
	"github.com/opensvc/om3/v3/util/plog"
	"github.com/opensvc/om3/v3/util/volfile"
)

// DefaultPort is the port the console listener listens on.
const DefaultPort = 1216

// graphicsDialTimeout bounds the search of a display and its connection.
const graphicsDialTimeout = 10 * time.Second

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
	conn, err := inheritedConn(connFile)
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
	peerCAs := x509.NewCertPool()
	if !peerCAs.AppendCertsFromPEM(b) {
		return fmt.Errorf("console peer authorities: no certificate in %s", daemonenv.CAsCertFile())
	}
	port := Port()
	certs := tlssecs.New(log).WithDefault(daemonenv.CertChainFile(), daemonenv.KeyFile())

	return Serve(conn, Config{
		Hostname:       hostname.Hostname(),
		Certificate:    cert,
		GetCertificate: certs.GetCertificate,
		VerifyKey:      verifyKey,
		TicketDir:      filepath.Join(Dir(), "tickets"),
		SessionDir:     SessionDir(),
		Command:        enterCommand,
		Graphics:       graphicsDial,
		Upload:         volumeUpload,
		Download:       volumeDownload,
		PeerURL: func(node string) (string, error) {
			return peerURL(node, port)
		},
		// A peer is reached as the api reaches it: its certificate is
		// verified by the cluster authorities, for its node name. The
		// ticket relayed to it would otherwise be presented to whoever
		// answered, before the peer marked it used.
		PeerTLSConfig: &tls.Config{RootCAs: peerCAs, MinVersion: tls.VersionTLS13},
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
	if target.Kind == console.KindShell {
		return []string{loginShell(), "-l"}, nil
	}
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

// graphicsDial connects to the display of the resource of a target.
func graphicsDial(target console.Target) (net.Conn, error) {
	path, err := naming.ParsePath(target.Path)
	if err != nil {
		return nil, fmt.Errorf("console target: %w", err)
	}
	o, err := object.NewActor(path)
	if err != nil {
		return nil, fmt.Errorf("console target: %w", err)
	}
	i, ok := o.(interface {
		GraphicsDial(ctx context.Context, rid string) (net.Conn, error)
	})
	if !ok {
		return nil, fmt.Errorf("console target: %s has no display", path)
	}
	ctx, cancel := context.WithTimeout(context.Background(), graphicsDialTimeout)
	defer cancel()
	return i.GraphicsDial(ctx, target.RID)
}

// volumeUpload writes r to the file of a target, within the size of the volume.
func volumeUpload(target console.Target, user string, r io.Reader) error {
	path, err := naming.ParsePath(target.Path)
	if err != nil {
		return fmt.Errorf("console target: %w", err)
	}
	if path.Kind != naming.KindVol || !volfile.IsName(target.File) {
		return fmt.Errorf("console target: %s %q is no file of a volume", path, target.File)
	}
	dir, err := volfile.CleanDir(target.Dir)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), graphicsDialTimeout)
	defer cancel()
	resolved, err := vpath.Resolve(ctx, path.Name+"/", path.Namespace, nil)
	if err != nil {
		return err
	}
	if resolved.Head == "" || resolved.Vol == nil {
		return fmt.Errorf("%s holds no filesystem", path)
	}
	size, err := resolved.Vol.ConfiguredSize()
	if err != nil {
		return err
	}
	if size <= 0 {
		size = -1
	}
	return volfile.Write(resolved.Head, dir, target.File, r, size, volfile.Upload{By: user, Replace: target.Replace})
}

// volumeDownload opens the file of a target, for reading.
func volumeDownload(target console.Target) (*os.File, error) {
	head, dir, err := volumeTargetDir(target)
	if err != nil {
		return nil, err
	}
	return volfile.Open(head, dir, target.File)
}

// volumeTargetDir returns the head of the volume of a target, and its directory.
func volumeTargetDir(target console.Target) (string, string, error) {
	path, err := naming.ParsePath(target.Path)
	if err != nil {
		return "", "", fmt.Errorf("console target: %w", err)
	}
	if path.Kind != naming.KindVol || !volfile.IsName(target.File) {
		return "", "", fmt.Errorf("console target: %s %q is no file of a volume", path, target.File)
	}
	dir, err := volfile.CleanDir(target.Dir)
	if err != nil {
		return "", "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), graphicsDialTimeout)
	defer cancel()
	resolved, err := vpath.Resolve(ctx, path.Name+"/", path.Namespace, nil)
	if err != nil {
		return "", "", err
	}
	if resolved.Head == "" {
		return "", "", fmt.Errorf("%s holds no filesystem", path)
	}
	return resolved.Head, dir, nil
}

// loginShell returns the shell of the user running the session, /bin/sh when
// it can not tell.
func loginShell() string {
	const fallback = "/bin/sh"
	u, err := user.Current()
	if err != nil {
		return fallback
	}
	b, err := os.ReadFile("/etc/passwd")
	if err != nil {
		return fallback
	}
	return shellOf(string(b), u.Username, fallback)
}

// shellOf returns the shell of username in the passwd file s, fallback when it
// has none.
func shellOf(s, username, fallback string) string {
	for line := range strings.Lines(s) {
		fields := strings.Split(strings.TrimSpace(line), ":")
		if len(fields) >= 7 && fields[0] == username && fields[6] != "" {
			return fields[6]
		}
	}
	return fallback
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
