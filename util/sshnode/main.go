package sshnode

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

type (
	AuthorizedKeysMap map[string]any
	KnownHostsMap     map[string]any
)

func expandUserSSH(basename string) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".ssh", basename), nil
}

func NewClient(n string) (*ssh.Client, error) {
	if n == "" {
		panic("empty hostname is not allowed")
	}
	signers := make([]ssh.Signer, 0)
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	privKeyFiles, err := filepath.Glob(filepath.Join(homeDir, ".ssh/id_*"))
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
	hostKeyCallback, err := getHostKeyCallback()
	if err != nil {
		return nil, err
	}

	config := &ssh.ClientConfig{
		User: "root",
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signers...),
		},
		HostKeyCallback: hostKeyCallback,
		Timeout:         time.Second * 10,
	}
	return ssh.Dial("tcp", n+":22", config)
}

func getHostKeyCallback() (ssh.HostKeyCallback, error) {
	knownHostsPath, err := expandUserSSH("known_hosts")
	if err != nil {
		return nil, err
	}
	return knownhosts.New(knownHostsPath)
}

func AppendAuthorizedKeys(line []byte) error {
	if len(line) == 0 {
		return nil
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	filename := filepath.Join(homeDir, ".ssh", "authorized_keys")
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

func AddKnownHostLine(host string, encodedKey []byte) error {
	key, _, _, _, err := ssh.ParseAuthorizedKey(encodedKey)
	if err != nil {
		return err
	}
	return AddKnownHost(host, key)
}

// AddKnownHost callers must care for thread safety
func AddKnownHost(host string, key ssh.PublicKey) error {
	knownHostFile, err := expandUserSSH("known_hosts")
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

func GetAuthorizedKeysMap() (AuthorizedKeysMap, error) {
	filename := os.ExpandEnv("$HOME/.ssh/authorized_keys")
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	m := make(AuthorizedKeysMap)
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
	filename := os.ExpandEnv("$HOME/.ssh/known_hosts")
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	m := make(KnownHostsMap)
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
