package sshnode

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"

	"github.com/opensvc/om3/util/file"
)

var (
	knownHostFile = os.ExpandEnv("$HOME/.ssh/known_hosts")
)

func NewClient(n string) (*ssh.Client, error) {
	if n == "" {
		panic("empty hostname is not allowed")
	}
	ip := net.ParseIP(n)
	if ip == nil {
		if ips, err := net.LookupIP(n); err != nil {
			return nil, err
		} else if len(ips) == 0 {
			return nil, fmt.Errorf("no ip address found for host %s", n)
		} else {
			ip = ips[0]
		}
	}
	signers := make([]ssh.Signer, 0)
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	privKeyFiles, err := filepath.Glob(filepath.Join(home, ".ssh/id_*"))
	if err != nil {
		return nil, err
	}
	if len(privKeyFiles) == 0 {
		return nil, fmt.Errorf("no private key found in ~/.ssh/id_*")
	}
	for _, privKeyFile := range privKeyFiles {
		if strings.Contains(filepath.Base(privKeyFile), ".") {
			continue
		}
		if key, err := os.ReadFile(privKeyFile); err == nil {
			if signer, err := ssh.ParsePrivateKey(key); err == nil {
				signers = append(signers, signer)
			}
		}
	}
	config := &ssh.ClientConfig{
		User: "root",
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signers...),
		},
		HostKeyCallback: AddingKnownHostCallback,
		Timeout:         time.Second * 10,
	}
	client, err := ssh.Dial("tcp", fmt.Sprintf("%s:22", ip), config)
	return client, err
}

func AddingKnownHostCallback(host string, remote net.Addr, key ssh.PublicKey) error {
	var keyErr *knownhosts.KeyError

	callback, err := knownhosts.New(knownHostFile)

	if os.IsNotExist(err) {
		if err := file.Touch(knownHostFile, time.Now()); err != nil {
			return err
		}
		callback, err = knownhosts.New(knownHostFile)
	}

	if err != nil {
		return err
	}
	err = callback(host, remote, key)
	if err == nil {
		return nil
	}
	v := errors.As(err, &keyErr)
	if v && len(keyErr.Want) > 0 {
		return fmt.Errorf("%s: conflicting %s +%d", keyErr, keyErr.Want[0].Filename, keyErr.Want[0].Line)
	}
	if v && len(keyErr.Want) == 0 {
		return AddKnownHost(host, remote, key)
	}
	if err != nil {
		return err
	}
	return nil
}

func AddKnownHost(host string, remote net.Addr, key ssh.PublicKey) (err error) {
	f, err := os.OpenFile(knownHostFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	knownHost := knownhosts.Normalize(remote.String())
	_, err = f.WriteString(knownhosts.Line([]string{knownHost}, key) + "\n")
	return err
}
