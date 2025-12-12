package sshnode

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/opensvc/om3/v3/util/funcopt"
)

// WithUser sets the ssh user value (default is root)
func WithUser(user string) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*clientOption)
		t.user = user
		return nil
	})
}

// WithPort sets the ssh port value (default is 22)
func WithPort(port uint16) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*clientOption)
		t.port = port
		return nil
	})
}

// WithTimeout sets the ssh ClientConfig.Timeout (default is 10 * time.Second)
func WithTimeout(timeout time.Duration) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*clientOption)
		t.timeout = timeout
		return nil
	})
}

// WithPrivateKeyFiles specifies private key files to be added at the start of the
// default list of private key files.
func WithPrivateKeyFiles(privateKeyFiles ...string) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*clientOption)
		for _, identityFile := range privateKeyFiles {
			if identityFile == "" {
				continue
			}
			if slices.Contains(t.privateKeyFiles, identityFile) {
				continue
			}
			t.privateKeyFiles = append([]string{identityFile}, t.privateKeyFiles...)
		}
		return nil
	})
}

func defaultClientOption() (*clientOption, error) {
	var privateKeyFiles []string

	if homeDir, err := os.UserHomeDir(); err != nil {
		return nil, err
	} else if sshIdFiles, err := filepath.Glob(filepath.Join(homeDir, ".ssh/id_*")); err != nil {
		// TODO: use an explicit list of candidates
		return nil, err
	} else {
		for _, sshIdFile := range sshIdFiles {
			if strings.Contains(filepath.Base(sshIdFile), ".") {
				continue
			}
			privateKeyFiles = append(privateKeyFiles, sshIdFile)
		}
	}

	return &clientOption{
		user:            "root",
		port:            22,
		timeout:         time.Second * 10,
		privateKeyFiles: privateKeyFiles,
	}, nil
}
