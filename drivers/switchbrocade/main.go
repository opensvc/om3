// Package switchbrocade reads the configuration of a brocade SAN switch, for
// the collector to index its ports, zones and aliases.
//
// It is the port of the v2 brocade sanswitch driver: it runs switchshow,
// nsshow and zoneshow on the switch, and reports their outputs as they are,
// the collector parsing them. It logs in over ssh, with a key or with a
// password held in a secret. The telnet method v2 offered is refused: the
// switches have telnet off by default, and it sends the password in clear.
package switchbrocade

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/opensvc/om3/v3/core/datarecv"
	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/sanswitch"
	"github.com/opensvc/om3/v3/util/sshnode"
)

type (
	// T is a brocade switch, declared by a "switch#<name>" section of type
	// brocade.
	T struct {
		sanswitch.Switch
	}
)

var (
	// Commands are the switch commands the configuration is read from, in
	// the order they run, named as the collector expects them.
	Commands = []string{"switchshow", "nsshow", "zoneshow"}

	// connectTimeout bounds the connection to the switch, as the v2 driver
	// did with ConnectTimeout=5.
	connectTimeout = 5 * time.Second

	// sshPort is the port the switch serves ssh on.
	sshPort = "22"
)

func init() {
	driver.Register(driver.NewID(driver.GroupSwitch, "brocade"), NewDriver)
}

func NewDriver() sanswitch.Driver {
	return &T{}
}

// Report runs the switch commands and returns their outputs, by command.
func (t *T) Report(ctx context.Context) (map[string]string, error) {
	client, err := t.connect()
	if err != nil {
		return nil, err
	}
	defer client.Close()
	// A switch that stops answering holds a session open: closing the
	// client ends it when the context is done.
	stop := context.AfterFunc(ctx, func() { _ = client.Close() })
	defer stop()

	data := make(map[string]string, len(Commands))
	for _, cmd := range Commands {
		out, err := t.run(client, cmd)
		if err != nil {
			if ctx.Err() != nil {
				return nil, fmt.Errorf("%s: %w", cmd, ctx.Err())
			}
			return nil, fmt.Errorf("%s: %w", cmd, err)
		}
		data[cmd] = out
	}
	return data, nil
}

// run runs cmd in a session of its own. A firmware answering "command not
// found" wants the command run in a login shell, as the v2 driver found.
func (t *T) run(client *ssh.Client, cmd string) (string, error) {
	stdout, stderr, err := t.runSession(client, cmd)
	if strings.Contains(stderr, "command not found") {
		stdout, stderr, err = t.runSession(client, "bash --login -c "+cmd)
	}
	if err != nil {
		if msg := strings.TrimSpace(stderr); msg != "" {
			return "", fmt.Errorf("%w: %s", err, msg)
		}
		return "", err
	}
	return stdout, nil
}

func (t *T) runSession(client *ssh.Client, cmd string) (string, string, error) {
	session, err := client.NewSession()
	if err != nil {
		return "", "", err
	}
	defer session.Close()
	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr
	err = session.Run(cmd)
	return stdout.String(), stderr.String(), err
}

// connect logs in to the switch over ssh.
func (t *T) connect() (*ssh.Client, error) {
	if method := t.method(); method != "ssh" {
		return nil, fmt.Errorf("%s: method %s is not supported, set %s = ssh", t.Name(), method, t.Key("method"))
	}
	username := t.username()
	if username == "" {
		return nil, fmt.Errorf("%s: %s is not set", t.Name(), t.Key("username"))
	}
	auth, err := t.authMethods()
	if err != nil {
		return nil, err
	}
	hostKeyCallback, err := sshnode.TrustOnFirstUseHostKeyCallback()
	if err != nil {
		return nil, err
	}
	config := &ssh.ClientConfig{
		User:            username,
		Auth:            auth,
		HostKeyCallback: hostKeyCallback,
		Timeout:         connectTimeout,
	}
	return ssh.Dial("tcp", t.addr(), config)
}

// ReportName is the name the switch is reached by, without its port, which
// the collector knows it by, as v2 reported it.
func (t *T) ReportName() string {
	host := t.host()
	if h, _, err := net.SplitHostPort(host); err == nil {
		return h
	}
	return host
}

// addr is the address to dial: the host on the ssh port, or the host and
// port the name keyword gives, as "sansw1:2222" for a switch reached
// through a port forward.
func (t *T) addr() string {
	host := t.host()
	if _, _, err := net.SplitHostPort(host); err == nil {
		return host
	}
	return net.JoinHostPort(host, sshPort)
}

// authMethods logs in with the key when one is set, with the password when
// one is set, and tries the key first when both are.
func (t *T) authMethods() ([]ssh.AuthMethod, error) {
	var l []ssh.AuthMethod
	if keyFile := t.keyFile(); keyFile != "" {
		b, err := os.ReadFile(keyFile)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", t.Key("key"), err)
		}
		signer, err := ssh.ParsePrivateKey(b)
		if err != nil {
			return nil, fmt.Errorf("%s: %s: %w", t.Key("key"), keyFile, err)
		}
		l = append(l, ssh.PublicKeys(signer))
	}
	if t.Config().GetString(t.Key("password")) != "" {
		password, err := t.password()
		if err != nil {
			return nil, err
		}
		l = append(l, ssh.Password(password))
	}
	if len(l) == 0 {
		return nil, fmt.Errorf("%s: set %s or %s", t.Name(), t.Key("key"), t.Key("password"))
	}
	return l, nil
}

// host is the name the switch is reached by: the name keyword, else the
// name of the section.
func (t *T) host() string {
	if s := t.Config().GetString(t.Key("name")); s != "" {
		return s
	}
	return t.ShortName()
}

func (t *T) method() string {
	if s := t.Config().GetString(t.Key("method")); s != "" {
		return s
	}
	return "ssh"
}

func (t *T) username() string {
	return t.Config().GetString(t.Key("username"))
}

func (t *T) keyFile() string {
	return t.Config().GetString(t.Key("key"))
}

// password reads the password from the secret the password keyword names,
// as the array drivers do: "from system/sec/sansw1 key password", or the
// secret alone, whose password key is read.
func (t *T) password() (string, error) {
	s, err := t.Config().GetStringStrict(t.Key("password"))
	if err != nil {
		return "", err
	}
	km, err := datarecv.ParseKeyMetaRelWithFallback(s, naming.NsSys, "password")
	if err != nil {
		return "", fmt.Errorf("%s: %w", t.Key("password"), err)
	}
	b, err := km.RootDecode()
	if err != nil {
		return "", fmt.Errorf("%s: %w", t.Key("password"), err)
	}
	return string(b), nil
}
