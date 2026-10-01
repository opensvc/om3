package sshnode

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"os"
	"sync"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

var (
	// trustOnFirstUseMu serializes the known hosts file updates of the
	// callbacks TrustOnFirstUseHostKeyCallback returns.
	trustOnFirstUseMu sync.Mutex
)

// TrustOnFirstUseHostKeyCallback returns a host key callback checking the
// host keys against the known hosts file of the user, as ssh does with
// StrictHostKeyChecking=accept-new: the key of a host the file does not know
// is added to it and accepted, a key the file knows is accepted, and a key
// that differs from the one the file knows for the host is refused, as the
// host may not be the one it was.
//
// It serves the devices om logs in to on their name alone, a SAN switch for
// one, where there is no key to learn from the cluster.
func TrustOnFirstUseHostKeyCallback() (ssh.HostKeyCallback, error) {
	filename, err := knownHostsFile()
	if err != nil {
		return nil, err
	}
	if err := CreateSSHDir(); err != nil {
		return nil, err
	}
	if f, err := os.OpenFile(filename, os.O_CREATE|os.O_RDONLY, 0600); err != nil {
		return nil, err
	} else {
		_ = f.Close()
	}
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		trustOnFirstUseMu.Lock()
		defer trustOnFirstUseMu.Unlock()
		// The file is read at each connection, so a key added by a
		// previous connection of this process is known.
		check, err := knownhosts.New(filename)
		if err != nil {
			return err
		}
		err = check(hostname, remote, key)
		var keyErr *knownhosts.KeyError
		switch {
		case err == nil:
			return nil
		case errors.As(err, &keyErr) && len(keyErr.Want) == 0:
			if err := addKnownHostNormalized(filename, hostname, key); err != nil {
				return fmt.Errorf("add the %s key of %s to %s: %w", key.Type(), hostname, filename, err)
			}
			return nil
		case errors.As(err, &keyErr):
			return fmt.Errorf("the %s key of %s differs from the one %s knows: remove it there if the host was replaced: %w", key.Type(), hostname, filename, err)
		default:
			return err
		}
	}, nil
}

// addKnownHostNormalized appends the key of the host to the known hosts file,
// the host hashed in the form the check looks it up by: the bare name for the
// ssh port, "[name]:port" for another, so a host on another port is not taken
// for a new one at each connection.
func addKnownHostNormalized(filename, hostname string, key ssh.PublicKey) error {
	f, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintf(f, "%s %s %s\n",
		knownhosts.HashHostname(knownhosts.Normalize(hostname)),
		key.Type(),
		base64.StdEncoding.EncodeToString(key.Marshal()),
	)
	return err
}
