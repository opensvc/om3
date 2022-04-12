package sshnode

import (
	"os"
	"os/user"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

func NewClient(n string) (*ssh.Client, error) {
	u, err := user.Current()
	if err != nil {
		return nil, err
	}
	hostKeyCallback, err := knownhosts.New(u.HomeDir + "/.ssh/known_hosts")
	if err != nil {
		return nil, err
	}
	signers := make([]ssh.Signer, 0)
	if key, err := os.ReadFile(u.HomeDir + "/.ssh/id_rsa"); err == nil {
		if signer, err := ssh.ParsePrivateKey(key); err == nil {
			signers = append(signers, signer)
		}
	}
	config := &ssh.ClientConfig{
		User: "root",
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signers...),
		},
		HostKeyCallback: hostKeyCallback,
		Timeout:         time.Duration(time.Second * 10),
	}
	client, err := ssh.Dial("tcp", n+":22", config)
	return client, err
}
