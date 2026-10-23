package storetar

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"

	"github.com/opensvc/om3/v3/core/datarecv"
	"github.com/opensvc/om3/v3/core/naming"
)

type (
	// storeFS is where the store keeps its files: a directory of the
	// node, or of an sftp server.
	storeFS interface {
		MkdirAll(dir string) error
		Create(name string) (storeFile, error)
		Rename(oldname, newname string) error
		ReadFile(name string) ([]byte, error)
		WriteFile(name string, b []byte) error
		Glob(pattern string) ([]string, error)
		Stat(name string) error
		OpenIn(dir, name string) (io.ReadCloser, error)
		Remove(name string) error
		RemoveAll(dir string) error

		// Free returns the space left in the filesystem of dir, and
		// false when it can not tell.
		Free(dir string) (int64, bool, error)
		Close() error
	}

	// storeFile is a file of the store being written.
	storeFile interface {
		io.Writer
		Sync() error
		Close() error
	}

	localFS struct{}

	sftpFS struct {
		conn   *ssh.Client
		client *sftp.Client
	}
)

func (localFS) MkdirAll(dir string) error { return os.MkdirAll(dir, 0700) }

func (localFS) Create(name string) (storeFile, error) {
	return os.OpenFile(name, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
}

func (localFS) Rename(oldname, newname string) error { return os.Rename(oldname, newname) }

func (localFS) ReadFile(name string) ([]byte, error) { return os.ReadFile(name) }

func (localFS) WriteFile(name string, b []byte) error { return os.WriteFile(name, b, 0600) }

func (localFS) Glob(pattern string) ([]string, error) { return filepath.Glob(pattern) }

func (localFS) Stat(name string) error {
	_, err := os.Stat(name)
	return err
}

// OpenIn opens the file name of dir, which no symlink escapes.
func (localFS) OpenIn(dir, name string) (io.ReadCloser, error) { return os.OpenInRoot(dir, name) }

func (localFS) Remove(name string) error { return os.Remove(name) }

func (localFS) RemoveAll(dir string) error { return os.RemoveAll(dir) }

func (localFS) Free(dir string) (int64, bool, error) { return localFree(dir) }

func (localFS) Close() error { return nil }

func (t *sftpFS) MkdirAll(dir string) error { return t.client.MkdirAll(dir) }

// Free asks the server through the statvfs@openssh.com extension, which a
// server not offering it answers with an error, said as not telling.
func (t *sftpFS) Free(dir string) (int64, bool, error) {
	st, err := t.client.StatVFS(dir)
	if err != nil {
		return 0, false, nil
	}
	return int64(st.Bavail * st.Frsize), true, nil
}

func (t *sftpFS) Create(name string) (storeFile, error) {
	f, err := t.client.OpenFile(name, os.O_CREATE|os.O_TRUNC|os.O_WRONLY)
	if err != nil {
		return nil, err
	}
	return sftpFile{f}, nil
}

// Rename replaces newname, as a rename does on a node: an sftp rename refuses
// an existing target, the posix-rename extension does not.
func (t *sftpFS) Rename(oldname, newname string) error {
	return t.client.PosixRename(oldname, newname)
}

func (t *sftpFS) ReadFile(name string) ([]byte, error) {
	f, err := t.client.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}

func (t *sftpFS) WriteFile(name string, b []byte) error {
	f, err := t.Create(name)
	if err != nil {
		return err
	}
	if _, err := f.Write(b); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

func (t *sftpFS) Glob(pattern string) ([]string, error) { return t.client.Glob(pattern) }

func (t *sftpFS) Stat(name string) error {
	_, err := t.client.Stat(name)
	return err
}

// OpenIn opens the file name of dir, a name that is one path element of its
// own: the sftp server confines the user to what it may read.
func (t *sftpFS) OpenIn(dir, name string) (io.ReadCloser, error) {
	return t.client.Open(filepath.Join(dir, name))
}

func (t *sftpFS) Remove(name string) error { return t.client.Remove(name) }

func (t *sftpFS) RemoveAll(dir string) error { return t.client.RemoveAll(dir) }

func (t *sftpFS) Close() error {
	err := t.client.Close()
	if t.conn != nil {
		err = errors.Join(err, t.conn.Close())
	}
	return err
}

// sftpFile is a file of an sftp store: a server without the fsync extension
// leaves the sync to its own writes.
type sftpFile struct {
	*sftp.File
}

func (t sftpFile) Sync() error {
	var status *sftp.StatusError
	err := t.File.Sync()
	if errors.As(err, &status) && status.FxCode() == sftp.ErrSSHFxOpUnsupported {
		return nil
	}
	return err
}

// location returns the file system and the directory of the store, from its
// url: an absolute path or a file:// url for a directory of the node, an
// sftp://<user>@<host>[:<port>]/<path> url for a directory of an sftp
// server, connected to once and kept until Close.
func (t *T) location() (storeFS, string, error) {
	s := t.Get("url")
	switch {
	case s == "":
		return nil, "", fmt.Errorf("store %s: no url", t.Ref())
	case strings.HasPrefix(s, "sftp://"):
		u, err := url.Parse(s)
		if err != nil {
			return nil, "", fmt.Errorf("store %s: url %q: %w", t.Ref(), s, err)
		}
		dir := filepath.Clean(u.Path)
		if !filepath.IsAbs(dir) {
			return nil, "", fmt.Errorf("store %s: url %q holds no absolute path", t.Ref(), s)
		}
		if t.fsys == nil {
			fsys, err := t.dialSFTP(u)
			if err != nil {
				return nil, "", fmt.Errorf("store %s: %w", t.Ref(), err)
			}
			t.fsys = fsys
		}
		return t.fsys, dir, nil
	}
	dir := strings.TrimPrefix(s, "file://")
	if !filepath.IsAbs(dir) {
		return nil, "", fmt.Errorf("store %s: url %q is not an absolute path", t.Ref(), s)
	}
	if t.fsys != nil {
		// The directory of a peer, reached over ssh.
		return t.fsys, filepath.Clean(dir), nil
	}
	return localFS{}, filepath.Clean(dir), nil
}

// IsLocal says whether the store is a directory of the node.
func (t *T) IsLocal() bool {
	return !strings.HasPrefix(t.Get("url"), "sftp://")
}

// IsShared implements backup.Sharer: an sftp store is the same for every
// node, and a directory of the node is when its shared keyword says so, as
// an nfs mount.
func (t *T) IsShared() bool {
	if !t.IsLocal() {
		return true
	}
	shared, _ := strconv.ParseBool(t.Get("shared"))
	return shared
}

// OverSSH implements backup.Sharer: the store is the directory of the url on
// the node conn is connected to, read and written over sftp, conn closed at
// Close.
func (t *T) OverSSH(conn *ssh.Client) error {
	client, err := sftp.NewClient(conn)
	if err != nil {
		return err
	}
	t.fsys = &sftpFS{conn: conn, client: client}
	return nil
}

// dialSFTP connects to the sftp server of u as its user, authenticated by
// the key ssh_private_key designates, the server authenticated by host_key:
// a store is never written to a server it did not recognize.
func (t *T) dialSFTP(u *url.URL) (*sftpFS, error) {
	user := u.User.Username()
	if user == "" {
		return nil, fmt.Errorf("url %s names no user", u.Redacted())
	}
	hostKeyLine := t.Get("host_key")
	if hostKeyLine == "" {
		return nil, fmt.Errorf("no host_key: set it to the public key of the sftp server, as ssh-keyscan prints it")
	}
	hostKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(hostKeyLine))
	if err != nil {
		// As ssh-keyscan prints it: the host first.
		if _, line, ok := strings.Cut(hostKeyLine, " "); ok {
			hostKey, _, _, _, err = ssh.ParseAuthorizedKey([]byte(line))
		}
		if err != nil {
			return nil, fmt.Errorf("host_key: %w", err)
		}
	}
	keyPEM, err := t.decodeKey("ssh_private_key")
	if err != nil {
		return nil, err
	}
	signer, err := ssh.ParsePrivateKey(keyPEM)
	if err != nil {
		return nil, fmt.Errorf("ssh_private_key: %w", err)
	}
	addr := u.Host
	if u.Port() == "" {
		addr = net.JoinHostPort(u.Hostname(), "22")
	}
	// The server is asked for the key of host_key: one holding keys of
	// several types would otherwise present the one of the type the client
	// prefers.
	hostKeyAlgorithms := []string{hostKey.Type()}
	if hostKey.Type() == ssh.KeyAlgoRSA {
		hostKeyAlgorithms = []string{ssh.KeyAlgoRSASHA512, ssh.KeyAlgoRSASHA256, ssh.KeyAlgoRSA}
	}
	conn, err := ssh.Dial("tcp", addr, &ssh.ClientConfig{
		User:              user,
		Auth:              []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback:   ssh.FixedHostKey(hostKey),
		HostKeyAlgorithms: hostKeyAlgorithms,
		Timeout:           sftpDialTimeout,
	})
	if err != nil {
		return nil, fmt.Errorf("connect %s: %w", u.Redacted(), err)
	}
	client, err := sftp.NewClient(conn)
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("sftp %s: %w", u.Redacted(), err)
	}
	return &sftpFS{conn: conn, client: client}, nil
}

// decodeKey returns the content of the key the option designates, written
// from <datastore> key <key>: a secret is never written in the section.
//
// The key of a store declared in a sec is read as the object reading the
// store, whose namespace the datastore must share with, "from ." naming the
// sec itself. The key of a store of the cluster configuration is read as
// root.
func (t *T) decodeKey(option string) ([]byte, error) {
	s := t.Get(option)
	if s == "" {
		return nil, fmt.Errorf("no %s: set it to from <sec> key <key>", option)
	}
	var km datarecv.KeyMeta
	var err error
	if owner := t.Owner(); !owner.IsZero() {
		km, err = datarecv.ParseKeyMetaRelSelf(s, owner)
	} else {
		km, err = datarecv.ParseKeyMetaRel(s, naming.NsSys)
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w: write it from <sec> key <key>", option, err)
	}
	var b []byte
	if reader := t.Reader(); reader != "" {
		km.From = reader
		b, err = km.Decode()
	} else {
		b, err = km.RootDecode()
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", option, err)
	}
	return b, nil
}
