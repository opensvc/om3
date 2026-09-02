//go:build !windows

package resapp

import (
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/opensvc/om3/v3/util/usergroup"
)

// execCredential is the identity an action command runs as.
type execCredential struct {
	// User and Group are what om exec demotes to. They are a name or an
	// id, either of which its flags accept.
	User  string
	Group string

	// Name and Home are the passwd entry of User, for the environment
	// the command reads its identity from. They are empty when the
	// passwd database has no entry for it.
	Name string
	Home string
}

// credential returns the identity argv is to run as.
//
// This is the v2 policy, restored: a script installed by an operator
// runs as that operator, without the object configuration having to
// name them, which is what makes an app resource usable by someone who
// cannot edit the configuration.
//
//   - When argv[0] is a file owned by a user other than root, and the
//     passwd database knows that user, the command runs as that owner,
//     with the group of the file. The user and group keywords are
//     ignored: the file on disk is the authority.
//
//   - Otherwise, which is a root owned file, a file owned by a uid no
//     passwd entry names, and a command that needs a shell, the user
//     keyword decides and defaults to root. The group keyword overrides
//     the primary group of that user.
func (t *T) credential(argv []string) execCredential {
	if uid, gid, ok := commandOwner(argv); ok && uid != 0 {
		if u, err := user.LookupId(strconv.FormatUint(uint64(uid), 10)); err == nil {
			return execCredential{
				User:  u.Uid,
				Group: strconv.FormatUint(uint64(gid), 10),
				Name:  u.Username,
				Home:  u.HomeDir,
			}
		}
	}
	cred := execCredential{User: t.User, Group: t.Group}
	if cred.User == "" {
		cred.User = "root"
	}
	if u, err := usergroup.LookupUser(cred.User); err == nil {
		cred.Name = u.Username
		cred.Home = u.HomeDir
		if cred.Group == "" {
			// Demoting the user without the group leaves the command in
			// the group of the daemon, which is root's.
			cred.Group = u.Gid
		}
	}
	return cred
}

// commandOwner returns the owner of the file argv runs, when it runs a
// file this policy can read an identity from.
//
// It reads none from a command needing a shell, which is run as
// /bin/sh -c and whose first word says nothing about who wrote it, nor
// from a bare command name, which is resolved in PATH at exec time
// rather than being a path to stat. Both fall back to the keywords, as
// they did in v2.
func commandOwner(argv []string) (uid, gid uint32, ok bool) {
	if len(argv) == 0 {
		return 0, 0, false
	}
	if len(argv) > 1 && argv[0] == "/bin/sh" && argv[1] == "-c" {
		return 0, 0, false
	}
	if !strings.Contains(argv[0], "/") {
		return 0, 0, false
	}
	path, err := filepath.EvalSymlinks(argv[0])
	if err != nil {
		return 0, 0, false
	}
	info, err := os.Stat(path)
	if err != nil {
		return 0, 0, false
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, 0, false
	}
	return stat.Uid, stat.Gid, true
}

// env returns the identity variables a demoted command expects to find,
// as v2 set them. Without them a script running as its owner still
// reads the HOME of the daemon that started it.
func (c execCredential) env() []string {
	if c.Name == "" {
		return nil
	}
	env := []string{
		"LOGNAME=" + c.Name,
		"USER=" + c.Name,
	}
	if c.Home != "" {
		env = append(env, "HOME="+c.Home)
	}
	return env
}
