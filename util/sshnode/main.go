package sshnode

import (
	"bufio"
	"bytes"
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
	config := &ssh.ClientConfig{
		User: "root",
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signers...),
		},
		HostKeyCallback: AddingKnownHostCallback,
		Timeout:         time.Second * 10,
	}
	if ip.To4() == nil {
		return ssh.Dial("tcp", fmt.Sprintf("[%s]:22", ip), config)
	} else {
		return ssh.Dial("tcp", fmt.Sprintf("%s:22", ip), config)
	}
}

func AddingKnownHostCallback(host string, remote net.Addr, key ssh.PublicKey) error {
	var keyErr *knownhosts.KeyError

	knownHostFile, err := expandUserSSH("known_hosts")
	if err != nil {
		return err
	}
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

func AddKnownHost(host string, remote net.Addr, key ssh.PublicKey) error {
	knownHostFile, err := expandUserSSH("known_hosts")
	if err != nil {
		return err
	}
	f, err := os.OpenFile(knownHostFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	knownHost := knownhosts.Normalize(remote.String())
	_, err = f.WriteString(knownhosts.Line([]string{knownHost}, key) + "\n")
	return err
}

type AuthorizedKeysMap map[string]any

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

func GetAuthorizedKeysMap() (AuthorizedKeysMap, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	filename := filepath.Join(homeDir, ".ssh", "authorized_keys")
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
