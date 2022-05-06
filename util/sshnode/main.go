package sshnode

import (
	"fmt"
	"net"
	"os"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

var (
	knownHostFile = os.ExpandEnv("$HOME/.ssh/known_hosts")
)

func NewClient(n string) (*ssh.Client, error) {
	ip := net.ParseIP(n)
	if ip == nil {
		if ips, err := net.LookupIP(n); err != nil {
			return nil, err
		} else if len(ips) == 0 {
			return nil, errors.Errorf("no ip address found for host %s", n)
		} else {
			ip = ips[0]
		}
	}
	privKeyFile := os.ExpandEnv("$HOME/.ssh/id_rsa")
	signers := make([]ssh.Signer, 0)
	if key, err := os.ReadFile(privKeyFile); err == nil {
		if signer, err := ssh.ParsePrivateKey(key); err == nil {
			signers = append(signers, signer)
		}
	}
	config := &ssh.ClientConfig{
		User: "root",
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signers...),
		},
		HostKeyCallback: AddingKnownHostCallback,
		Timeout:         time.Duration(time.Second * 10),
	}
	client, err := ssh.Dial("tcp", fmt.Sprintf("%s:22", ip), config)
	return client, err
}

func AddingKnownHostCallback(host string, remote net.Addr, key ssh.PublicKey) error {
	var keyErr *knownhosts.KeyError
	callback, err := knownhosts.New(knownHostFile)
	if err != nil {
		return err
	}
	err = callback(host, remote, key)
	if err == nil {
		return nil
	}
	v := errors.As(err, &keyErr)
	if v && len(keyErr.Want) > 0 {
		return errors.Errorf("%s: conflicting %s +%d", keyErr, keyErr.Want[0].Filename, keyErr.Want[0].Line)
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
