package storeplakar

import (
	"bytes"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kballard/go-shellquote"
	"golang.org/x/crypto/ssh"

	"github.com/opensvc/om3/v3/util/converters"
)

const (
	filePerm    = 0600
	wrapperPerm = 0700

	// identityPrefix opens the name of the file holding the ssh private
	// key of the store, which a digest of the key and of the host key
	// closes: the sftp package of plakar keeps one ssh connection per
	// location, user and identity file for 10 minutes, so a key or a host
	// key changed is a file of another name, and a connection of its own.
	identityPrefix = "identity."

	knownHostsName = "known_hosts"
	caName         = "ca.pem"
	caDirName      = "ca.d"
	wrapperName    = "ssh"

	// defaultPath is the PATH plakar gets after the directory of the ssh
	// command of the store when om has none.
	defaultPath = "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
)

type (
	// credentials is what the keywords of the store make of its
	// credentials: the options of its entry of stores.yml, the files of its
	// configuration directory, and the ssh command of its sftp location.
	// None goes to a command line, the environment or a log.
	credentials struct {
		options map[string]string
		files   map[string][]byte
		wrapper []byte
	}
)

// credentialSchemes are the credential keywords, the region a s3 store
// signs for and its disable_tls, with the scheme of the locations each is
// of.
var credentialSchemes = map[string]string{
	"ssh_private_key": "sftp",
	"host_key":        "sftp",
	"access_key":      "s3",
	"secret_key":      "s3",
	"region":          "s3",
	"disable_tls":     "s3",
	"token":           "https",
	"ca":              "https",
}

// sshPath returns the ssh of the node, replaced by the tests.
var sshPath = func() (string, error) {
	return exec.LookPath("ssh")
}

// plainHTTPMessages are what the s3 client of plakar says of an endpoint
// answering in plain http to its TLS handshake, in the stderr of plakar:
// the first of a server answering the handshake with a http response, as a
// Go server does, the second of one answering anything else.
var plainHTTPMessages = []string{
	"server gave HTTP response to HTTPS client",
	"first record does not look like a TLS handshake",
}

// plainHTTPHint is what the driver adds to the error of plakar on a s3
// store whose endpoint answers in plain http. It is a hint only: the driver
// never reaches a store without TLS by itself.
const plainHTTPHint = "the endpoint answers in plain http: set disable_tls = true for a store meant to be reached without TLS (root only)"

// scheme returns the scheme of a location, in lower case.
func scheme(location string) string {
	s, _, _ := strings.Cut(location, "://")
	return strings.ToLower(s)
}

// isSet tells whether the option is written in the section of the store,
// whatever it evaluates to: a keyword with a default, as disable_tls, has a
// value when unset.
func (t *T) isSet(option string) bool {
	if cfg := t.Config(); cfg != nil && cfg.Get(t.Key(option)) != "" {
		return true
	}
	if option == "disable_tls" {
		v, err := t.disableTLS()
		return v || err != nil
	}
	return t.Get(option) != ""
}

// disableTLS tells whether the store is to be reached without TLS, which
// its disable_tls says, false when unset.
func (t *T) disableTLS() (bool, error) {
	s := t.Get("disable_tls")
	v, err := converters.Bool.Convert(s)
	if err != nil {
		return false, fmt.Errorf("disable_tls %q is not a boolean", s)
	}
	return v.(bool), nil
}

// isKeyReference tells whether a keyword value names a key of a datastore
// rather than holding the value itself.
func isKeyReference(s string) bool {
	return strings.HasPrefix(strings.TrimSpace(s), "from ")
}

// secret returns the content of the key the option designates.
func (t *T) secret(option string) ([]byte, error) {
	km, err := t.keyMeta(option)
	if err != nil {
		return nil, err
	}
	return t.decodeKey(option, km)
}

// public returns what the option holds, or the content of the key it
// designates when written from <datastore> key <key>: a host key is no
// secret, and may be written in the section.
func (t *T) public(option string) ([]byte, error) {
	s := t.Get(option)
	if !isKeyReference(s) {
		return []byte(s), nil
	}
	return t.secret(option)
}

// oneLine returns the content of the key the option designates, which an
// option of stores.yml holds: one line, its trailing newline dropped. The
// error never holds the content.
func (t *T) oneLine(option string) (string, error) {
	b, err := t.secret(option)
	if err != nil {
		return "", err
	}
	if err := checkOneLine(option, b); err != nil {
		return "", err
	}
	return string(bytes.TrimSuffix(b, []byte("\n"))), nil
}

// knownHostsLine returns the line of a known hosts file accepting the key
// of host_key, as ssh-keyscan prints it, with or without the host before it,
// from any host: the store names its server in its url, and the key is
// what recognizes it.
func knownHostsLine(b []byte) (string, error) {
	line := strings.TrimSpace(string(b))
	pk, _, _, _, err := ssh.ParseAuthorizedKey([]byte(line))
	if err != nil {
		// With the host before it.
		if _, rest, ok := strings.Cut(line, " "); ok {
			pk, _, _, _, err = ssh.ParseAuthorizedKey([]byte(rest))
		}
		if err != nil {
			return "", fmt.Errorf("host_key: %w", err)
		}
	}
	return "* " + strings.TrimSpace(string(ssh.MarshalAuthorizedKey(pk))), nil
}

// sshWrapper is the ssh command the sftp package of plakar runs for the
// store, found first in the PATH of its plakar: the package runs the ssh of
// the PATH with the options of its own, and has none to keep ssh off what
// the user om runs as holds. The command runs the ssh of the node with the
// key and the known hosts of the store only: no configuration file, of the
// user or of the system, no agent, no other identity, no other known hosts,
// and a server presenting another key refused. ssh keeps the first value it
// gets of an option, so the ones the package adds after these do not
// replace them.
func sshWrapper(ref, identity, knownHosts string) ([]byte, error) {
	bin, err := sshPath()
	if err != nil {
		return nil, fmt.Errorf("ssh is not installed: the sftp package of plakar runs it: %w", err)
	}
	cmd := shellquote.Join(
		bin,
		"-F", "/dev/null",
		"-o", "IdentitiesOnly=yes",
		"-o", "IdentityAgent=none",
		"-o", "IdentityFile="+identity,
		"-o", "CertificateFile=none",
		"-o", "PreferredAuthentications=publickey",
		"-o", "UserKnownHostsFile="+knownHosts,
		"-o", "GlobalKnownHostsFile=/dev/null",
		"-o", "StrictHostKeyChecking=yes",
		"-o", "UpdateHostKeys=no",
	)
	return []byte("#!/bin/sh\n# Written by om for the store " + strings.ReplaceAll(ref, "\n", " ") + ": do not edit.\nexec " + cmd + " \"$@\"\n"), nil
}

// credentials reads the credential keywords of the store into what plakar
// is handed for the location, and refuses the store whose location would be
// reached with what the node holds instead:
//
//   - a sftp:// store logs in with the key of ssh_private_key and recognizes
//     its server by host_key, both required: without them the ssh of the
//     package would try the keys, the agent and the configuration of the
//     user om runs as, root, and trust its known hosts.
//   - a s3:// store signs with access_key and secret_key, both required:
//     without them the package asks the credentials of the node, as the
//     role of its instance. Its region, when set, is the one the package
//     signs for instead of the one its s3 client finds by itself, the one
//     the server tells for the bucket or us-east-1. It is reached with
//     TLS, as the package does by default, unless its disable_tls is true:
//     the driver never falls back to plain http by itself.
//   - a https:// store may send a token, and may trust the certificate
//     authorities of ca only, instead of the ones of the system.
//
// A credential keyword, a region or a disable_tls, of another scheme than
// the one of the url is refused, rather than silently not used. The other schemes have no
// credential keyword.
func (t *T) credentials(location string, d dirs) (credentials, error) {
	c := credentials{
		options: make(map[string]string),
		files:   make(map[string][]byte),
	}
	sch := scheme(location)
	options := make([]string, 0, len(credentialSchemes))
	for option := range credentialSchemes {
		options = append(options, option)
	}
	sort.Strings(options)
	for _, option := range options {
		if of := credentialSchemes[option]; of != sch && t.isSet(option) {
			return c, fmt.Errorf("%s is a keyword of a %s:// store, not of the url %s", option, of, t.Get("url"))
		}
	}
	switch sch {
	case "sftp":
		if t.Get("ssh_private_key") == "" || t.Get("host_key") == "" {
			return c, errors.New("a sftp:// store needs ssh_private_key and host_key: a store never logs in with the ssh keys, agent and configuration of root, nor trusts its known hosts")
		}
		key, err := t.secret("ssh_private_key")
		if err != nil {
			return c, err
		}
		if _, err := ssh.ParseRawPrivateKey(key); err != nil {
			// The error of the parser names the format, never the key.
			return c, fmt.Errorf("ssh_private_key: %w", err)
		}
		if !bytes.HasSuffix(key, []byte("\n")) {
			key = append(key, '\n')
		}
		hostKey, err := t.public("host_key")
		if err != nil {
			return c, err
		}
		line, err := knownHostsLine(hostKey)
		if err != nil {
			return c, err
		}
		sum := sha256.New()
		sum.Write(key)
		sum.Write([]byte{0})
		sum.Write([]byte(line))
		identityName := identityPrefix + hex.EncodeToString(sum.Sum(nil))[:16]
		identity := filepath.Join(d.config, identityName)
		knownHosts := filepath.Join(d.config, knownHostsName)
		c.files[identityName] = key
		c.files[knownHostsName] = []byte(line + "\n")
		// The options of the package say the same as the command: the
		// identity names the connection the package keeps, and the host
		// key is still required should the command not be the ssh run.
		c.options["identity"] = identity
		c.options["host_key"] = line
		if c.wrapper, err = sshWrapper(t.Ref(), identity, knownHosts); err != nil {
			return c, err
		}
	case "s3":
		if t.Get("access_key") == "" || t.Get("secret_key") == "" {
			return c, errors.New("a s3:// store needs access_key and secret_key: a store never signs with the credentials of the node, as the role of its instance")
		}
		accessKey, err := t.oneLine("access_key")
		if err != nil {
			return c, err
		}
		secretKey, err := t.oneLine("secret_key")
		if err != nil {
			return c, err
		}
		c.options["access_key"] = accessKey
		c.options["secret_access_key"] = secretKey
		// A value of the section, as a region is no secret. The option
		// is left out when unset: the package then lets its s3 client
		// find the region.
		if region := strings.TrimSpace(t.Get("region")); strings.ContainsAny(region, " \t\r\n") {
			return c, fmt.Errorf("region %q is not the name of a region", region)
		} else if region != "" {
			c.options["region"] = region
		}
		// The use_tls option of the package is a boolean in a string,
		// true when left out. tls_insecure_no_verify, its option to
		// accept any certificate, is never given: a store is reached
		// with a verified TLS, or with none when root says so.
		if disableTLS, err := t.disableTLS(); err != nil {
			return c, err
		} else if disableTLS {
			c.options["use_tls"] = "false"
		}
	case "https":
		if t.Get("token") != "" {
			token, err := t.oneLine("token")
			if err != nil {
				return c, err
			}
			c.options["auth_token"] = token
		}
		if t.Get("ca") != "" {
			ca, err := t.secret("ca")
			if err != nil {
				return c, err
			}
			if !x509.NewCertPool().AppendCertsFromPEM(ca) {
				return c, errors.New("ca: no PEM certificate found")
			}
			c.files[caName] = ca
		}
	}
	return c, nil
}

// env returns the environment plakar is run with for the store, added to
// the one of om. It holds no secret:
//
//   - the PATH of a sftp:// store opens with the directory of its ssh
//     command, which the sftp package of plakar then runs.
//   - a https:// store with a ca trusts the certificates of that file only:
//     the http client of plakar has no option for it, and reads the roots
//     of the system, which SSL_CERT_FILE and SSL_CERT_DIR replace.
func (t *T) env(d dirs) []string {
	location, _, err := t.location()
	if err != nil {
		return nil
	}
	switch scheme(location) {
	case "sftp":
		// An empty element of a PATH is the working directory: om run
		// with no PATH gives plakar the one of a system.
		path := os.Getenv("PATH")
		if path == "" {
			path = defaultPath
		}
		return []string{"PATH=" + d.bin + string(os.PathListSeparator) + path}
	case "https":
		if t.Get("ca") != "" {
			return []string{
				"SSL_CERT_FILE=" + filepath.Join(d.config, caName),
				"SSL_CERT_DIR=" + filepath.Join(d.config, caDirName),
			}
		}
	}
	return nil
}

// upToDate tells whether the file holds b with the mode perm.
func upToDate(path string, b []byte, perm os.FileMode) bool {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != perm {
		return false
	}
	got, err := os.ReadFile(path)
	return err == nil && bytes.Equal(got, b)
}

// forget removes what the store holds of its credentials on the node: the
// configuration holding their options, as the keys of a s3 store, their
// files and the ssh command. The configuration is written again by the next
// call of the store found valid.
func forget(d dirs) error {
	if err := os.Remove(filepath.Join(d.config, "stores.yml")); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if _, err := os.Stat(d.config); errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return credentials{}.install(d)
}

// install writes the credential files of the store in its configuration
// directory and its ssh command in its bin directory, the ones missing or
// stale only, and removes the ones the keywords no longer ask for: the key
// of a store whose keyword is unset, or changed, does not stay on the node.
func (c credentials) install(d dirs) error {
	entries, err := os.ReadDir(d.config)
	if err != nil {
		return err
	}
	for _, e := range entries {
		name := e.Name()
		if name != knownHostsName && name != caName && !strings.HasPrefix(name, identityPrefix) {
			continue
		}
		if _, ok := c.files[name]; ok {
			continue
		}
		if err := os.Remove(filepath.Join(d.config, name)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	for name, b := range c.files {
		if upToDate(filepath.Join(d.config, name), b, filePerm) {
			continue
		}
		if err := writeFile(d.config, name, b); err != nil {
			return err
		}
	}
	// The directory SSL_CERT_DIR names, empty: the certificates of the
	// system are not read from their directory either.
	caDir := filepath.Join(d.config, caDirName)
	if _, ok := c.files[caName]; ok {
		if err := mkdirPrivate(caDir); err != nil {
			return err
		}
	} else if err := os.Remove(caDir); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	wrapper := filepath.Join(d.bin, wrapperName)
	if c.wrapper == nil {
		if err := os.Remove(wrapper); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	if err := mkdirPrivate(d.bin); err != nil {
		return err
	}
	if upToDate(wrapper, c.wrapper, wrapperPerm) {
		return nil
	}
	return writeFileMode(d.bin, wrapperName, c.wrapper, wrapperPerm)
}
