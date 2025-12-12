package sshnode

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"

	"github.com/opensvc/om3/v3/util/file"
	"github.com/opensvc/om3/v3/util/funcopt"
)

type (
	AuthorizedKeysMap map[string]any
	KnownHostsMap     map[string]any

	clientOption struct {
		user            string
		port            uint16
		privateKeyFiles []string
		timeout         time.Duration
		signers         []ssh.Signer
		hostKeyCallback ssh.HostKeyCallback
	}
)

func NewClient(nodename string, opts ...funcopt.O) (*ssh.Client, error) {
	if nodename == "" {
		panic("empty hostname is not allowed")
	}

	var (
		option *clientOption
		err    error
	)

	if option, err = defaultClientOption(); err != nil {
		return nil, err
	}

	if err = funcopt.Apply(option, opts...); err != nil {
		return nil, err
	}

	for _, privKeyFile := range option.privateKeyFiles {
		if key, err := os.ReadFile(privKeyFile); err == nil {
			if signer, err := ssh.ParsePrivateKey(key); err == nil {
				option.signers = append(option.signers, signer)
			}
		}
	}
	if len(option.signers) == 0 {
		return nil, fmt.Errorf("the private keys are unusable: %s", option.privateKeyFiles)
	}

	if option.hostKeyCallback, err = getHostKeyCallback(); err != nil {
		return nil, err
	}

	return ssh.Dial("tcp", option.addr(nodename), option.clientConfig())
}

func CreateSSHDir() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	sshDir := filepath.Join(homeDir, ".ssh")
	if ok, err := file.ExistsAndDir(sshDir); err != nil {
		return err
	} else if !ok {
		return os.MkdirAll(filepath.Join(homeDir, ".ssh"), 0700)
	} else {
		return nil
	}
}

func AppendAuthorizedKeys(line []byte) error {
	if len(line) == 0 {
		return nil
	}
	filename, err := authorizedKeysFile()
	if err != nil {
		return err
	}
	file, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer file.Close()
	file.Write(line)
	if string(line[len(line)-1]) != "\n" {
		fmt.Fprintln(file, "")
	}
	return nil
}

// AddKnownHost callers must care for thread safety
func AddKnownHost(host string, key ssh.PublicKey) error {
	knownHostFile, err := knownHostsFile()
	if err != nil {
		return err
	}
	f, err := os.OpenFile(knownHostFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	if hostname, _, err := net.SplitHostPort(host); err == nil {
		host = hostname
	}

	// Non-hashed:
	//   _, err = f.WriteString(knownhosts.Line([]string{encodedHost}, key) + "\n")

	// Hashed
	_, err = fmt.Fprintf(f, "%s %s %s\n",
		knownhosts.HashHostname(host),
		key.Type(),
		base64.StdEncoding.EncodeToString(key.Marshal()),
	)
	return err
}

func AuthorizedKeysFile() (string, error) {
	return authorizedKeysFile()
}

func GetAuthorizedKeysMap() (AuthorizedKeysMap, error) {
	m := make(AuthorizedKeysMap)
	filename, err := authorizedKeysFile()
	file, err := os.Open(filename)
	if errors.Is(err, os.ErrNotExist) {
		return m, nil
	} else if err != nil {
		return nil, err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		data := scanner.Bytes()
		k, _, _, _, err := ssh.ParseAuthorizedKey(bytes.TrimSpace(data))
		if err != nil {
			//fmt.Fprintf(os.Stderr, "skipping invalid key in file %s: %s\n", file.Name(), err)
			continue
		}
		m[ssh.FingerprintSHA256(k)] = nil
	}

	// Check for errors
	if err := scanner.Err(); err != nil {
		return m, err
	}

	return m, nil
}

func (m AuthorizedKeysMap) Has(data []byte) (bool, error) {
	k, _, _, _, err := ssh.ParseAuthorizedKey(data)
	if err != nil {
		return false, err
	}
	if _, ok := m[ssh.FingerprintSHA256(k)]; ok {
		return true, nil
	}
	return false, nil
}

func GetKnownHostsMap() (KnownHostsMap, error) {
	m := make(KnownHostsMap)
	filename, err := knownHostsFile()
	if err != nil {
		return nil, err
	}
	file, err := os.Open(filename)
	if errors.Is(err, os.ErrNotExist) {
		return m, nil
	} else if err != nil {
		return nil, err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		data := scanner.Bytes()
		_, _, k, _, _, err := ssh.ParseKnownHosts(bytes.TrimSpace(data))
		if err != nil {
			//fmt.Fprintf(os.Stderr, "skipping invalid key in file %s: %s\n", file.Name(), err)
			continue
		}
		m[ssh.FingerprintSHA256(k)] = nil
	}

	// Check for errors
	if err := scanner.Err(); err != nil {
		return m, err
	}

	return m, nil
}

func KnownHostsFile() (string, error) {
	return knownHostsFile()
}

func (m KnownHostsMap) Add(host string, k ssh.PublicKey) error {
	if v, err := m.Has(k); err != nil {
		return err
	} else if v {
		return nil
	}
	return AddKnownHost(host, k)
}

func (m KnownHostsMap) Has(k ssh.PublicKey) (bool, error) {
	if _, ok := m[ssh.FingerprintSHA256(k)]; ok {
		return true, nil
	}
	return false, nil
}

func expandUserSSH(basename string) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".ssh", basename), nil
}

func knownHostsFile() (string, error) {
	return expandUserSSH("known_hosts")
}

func authorizedKeysFile() (string, error) {
	return expandUserSSH("authorized_keys")
}

func getHostKeyCallback() (ssh.HostKeyCallback, error) {
	knownHostsPath, err := knownHostsFile()
	if err != nil {
		return nil, err
	}
	return knownhosts.New(knownHostsPath)
}

func (o *clientOption) clientConfig() *ssh.ClientConfig {
	return &ssh.ClientConfig{
		User: o.user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(o.signers...),
		},
		HostKeyCallback: o.hostKeyCallback,
		Timeout:         o.timeout,
	}
}

func (o *clientOption) addr(nodename string) string {
	return fmt.Sprintf("%s:%d", nodename, o.port)
}
