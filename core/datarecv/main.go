package datarecv

import (
	"context"
	"embed"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"

	"github.com/opensvc/om3/v3/core/keywords"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/volsignal"
	"github.com/opensvc/om3/v3/util/plog"
)

type (
	DataRecv struct {
		Install []string     `json:"install"`
		User    string       `json:"user"`
		Group   string       `json:"group"`
		Perm    *os.FileMode `json:"perm"`
		DirPerm *os.FileMode `json:"dirperm"`
		Signal  string       `json:"signal"`

		// Deprecated
		Configs     []string `json:"configs"`
		Secrets     []string `json:"secrets"`
		Directories []string `json:"directories"`

		to receiver
	}

	// SigRoute is a relation between a signal number and the id of a resource supporting signaling
	SigRoute struct {
		Signum syscall.Signal
		RID    string
	}

	receiver interface {
		Head() string
		Log() *plog.Logger
		StatusLog() resource.StatusLogger
		GetObject() any
	}

	signaler interface {
		SignalResource(context.Context, string, syscall.Signal) error
		Path() naming.Path
	}
)

var (
	//go:embed text
	fs embed.FS

	// defaultSecPerm is the default KVInstall.AccessControl.Perm for
	// secrets parseReference when driver perm is undefined.
	defaultSecPerm = os.FileMode(0600)

	// defaultCfgPerm is the default KVInstall.AccessControl.Perm for
	// configs parseReference when driver perm is undefined.
	defaultCfgPerm = os.FileMode(0644)

	// defaultDirPerm
	defaultDirPerm = os.FileMode(0700)
)

// pop returns "a", "b c d" from "a b c d".
// This function is used in a loop to iterate all words of a line until pop returns "", "".
func pop(words []string) (string, []string) {
	if len(words) == 0 {
		return "", words
	}
	return words[0], words[1:]
}

func Keywords(prefix string) []keywords.Keyword {
	return []keywords.Keyword{
		keywords.Keyword{
			Attr:      prefix + "Install",
			Converter: "shlex",
			Example:   "file from sec {name} key password to path /data/password mode 0600 user 1000 group 1000",
			Option:    "install",
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/install"),
		},
		keywords.Keyword{
			Attr:      prefix + "Configs",
			Converter: "shlex",
			Example:   "conf/mycnf:/etc/mysql/my.cnf:ro conf/sysctl:/etc/sysctl.d/01-db.conf",
			Option:    "configs",
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/configs"),
		},
		keywords.Keyword{
			Attr:      prefix + "Secrets",
			Converter: "shlex",
			Default:   "",
			Example:   "cert/pem:server.pem cert/key:server.key",
			Option:    "secrets",
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/secrets"),
			Types:     []string{"shm"},
		},
		keywords.Keyword{
			Attr:      prefix + "Directories",
			Converter: "list",
			Default:   "",
			Example:   "a/b/c d /e",
			Option:    "directories",
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/directories"),
		},
		keywords.Keyword{
			Attr:     prefix + "User",
			Example:  "1001",
			Option:   "user",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/user"),
		},
		keywords.Keyword{
			Attr:     prefix + "Group",
			Example:  "1001",
			Option:   "group",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/group"),
		},
		keywords.Keyword{
			Attr:      prefix + "Perm",
			Converter: "filemode",
			Example:   "660",
			Option:    "perm",
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/perm"),
		},
		keywords.Keyword{
			Attr:      prefix + "DirPerm",
			Converter: "filemode",
			// Default value is fmt.Sprintf("%o", defaultDirPerm)
			Default:  "700",
			Example:  "750",
			Option:   "dirperm",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/dirperm"),
		},
		keywords.Keyword{
			Attr:     prefix + "Signal",
			Example:  "hup:container#1",
			Option:   "signal",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/signal"),
		},
	}
}

func (t *DataRecv) SetReceiver(to receiver) {
	t.to = to
}

// getDirPerm returns the driver dir perm value. When t.DirPerm is nil (when kw
// has no default value or unexpected value) the defaultDirPerm is returned.
func (t *DataRecv) getDirPerm() *os.FileMode {
	if t.DirPerm == nil {
		return &defaultDirPerm
	}
	return t.DirPerm
}

func (t *DataRecv) getRefs() []string {
	refs := make([]string, 0)
	refs = append(refs, t.getRefsByKind(naming.KindSec)...)
	refs = append(refs, t.getRefsByKind(naming.KindCfg)...)
	return refs
}

func (t *DataRecv) getRefsByKind(kind naming.Kind) []string {
	refs := make([]string, 0)
	switch kind {
	case naming.KindSec:
		refs = append(refs, t.Secrets...)
	case naming.KindCfg:
		refs = append(refs, t.Configs...)
	}
	return refs
}

func (t *DataRecv) getMetadata() []object.KVInstall {
	head := t.to.Head()
	l := make([]object.KVInstall, 0)
	_, files := t.getInstallMetadata(head)
	l = append(l, files...)
	l = append(l, t.getMetadataByKind(head, naming.KindSec, naming.KindCfg)...)
	return l
}

func (t *DataRecv) getMetadataByKind(head string, kinds ...naming.Kind) []object.KVInstall {
	l := make([]object.KVInstall, 0)
	for _, kind := range kinds {
		refs := t.getRefsByKind(kind)
		if len(refs) == 0 {
			return l
		}
		if head == "" {
			return l
		}
		for _, ref := range refs {
			md := t.parseReference(ref, kind, head)
			if md.IsZero() {
				continue
			}
			l = append(l, md)
		}
	}
	return l
}

// HasMetadata returns true if the volume has a configs or secrets reference to
// <namespace>/<kind>/<name>[/<key>]
func (t *DataRecv) HasMetadata(path naming.Path, k string) bool {
	head := t.to.Head()
	_, installMetadata := t.getInstallMetadata(head)
	for _, md := range installMetadata {
		if md.FromStore != path {
			continue
		}
		if k == "" || md.FromPattern == k {
			return true
		}
	}
	for _, md := range t.getMetadataByKind(head, path.Kind) {
		if md.FromStore != path {
			continue
		}
		if k == "" || md.FromPattern == k {
			return true
		}
	}
	return false
}

func (t *DataRecv) parseReference(s string, withKind naming.Kind, head string) object.KVInstall {
	path := t.to.GetObject().(object.Core).Path()
	if head == "" {
		return object.KVInstall{}
	}

	// s = "sec/s1/k[12]:/here/"
	l := strings.SplitN(s, ":", 2)
	if len(l) != 2 {
		return object.KVInstall{}
	}
	toPath := filepath.Join(head, l[1])
	// toPath = "/here"

	if strings.HasSuffix(l[1], "/") {
		// BEWARE: do not drop the trailing /, as it's the directory marker
		//         filepath.Join() drops it, so let's restore it.
		toPath = toPath + "/"
	}

	from := strings.TrimLeft(l[0], "/")
	// from = "sec/s1/k[12]"

	kind := withKind

	switch {
	case strings.HasPrefix(from, "usr/"):
		kind = naming.KindUsr
		from = from[4:]
		if withKind == naming.KindCfg {
			return object.KVInstall{}
		}
	case strings.HasPrefix(from, "sec/"):
		kind = naming.KindSec
		from = from[4:]
		if withKind == naming.KindCfg {
			return object.KVInstall{}
		}
	case strings.HasPrefix(from, "cfg/"):
		kind = naming.KindCfg
		from = from[4:]
		if withKind == naming.KindSec {
			return object.KVInstall{}
		}
	}
	// kind = path.KindSec
	// from = s1/k[12]

	l = strings.SplitN(from, "/", 2)
	if len(l) != 2 {
		return object.KVInstall{}
	}
	if p, err := naming.NewPath(path.Namespace, kind, l[0]); err != nil {
		return object.KVInstall{}
	} else {
		var perm *os.FileMode
		if t.Perm == nil {
			switch kind {
			case naming.KindSec:
				perm = &defaultSecPerm
			case naming.KindCfg:
				perm = &defaultCfgPerm
			}
		} else {
			perm = t.Perm
		}
		return object.KVInstall{
			ToPath:      toPath, // /here
			ToHead:      head,
			FromPattern: l[1], // k[12]
			FromStore:   p,    // <volns>/sec/s1
			AccessControl: object.KVInstallAccessControl{
				User:         t.User,
				Group:        t.Group,
				Perm:         perm,
				DirUser:      t.User,
				DirGroup:     t.Group,
				DirPerm:      t.getDirPerm(),
				MakedirUser:  t.User,
				MakedirGroup: t.Group,
				MakedirPerm:  t.getDirPerm(),
			},
		}
	}
}

func (t *DataRecv) Status() {
	path := t.to.GetObject().(object.Core).Path()
	for _, md := range t.getMetadata() {
		if !md.FromStore.Exists() {
			t.to.StatusLog().Warn("store %s does not exist: key %s data can not be installed in the volume", md.FromStore, md.FromPattern)
			continue
		}
		dataStore, err := object.NewDataStore(md.FromStore, object.WithVolatile(true))
		if err != nil {
			t.to.StatusLog().Warn("store %s init error: %s", md.FromStore, err)
			continue
		}
		shares := dataStore.Shares()
		if !slices.Contains(shares, "*") && !slices.Contains(shares, path.Namespace) {
			path := strings.TrimPrefix(md.ToPath, md.ToHead)
			t.to.StatusLog().Warn("unauthorized install ...%s from %s key %s", path, md.FromStore, md.FromPattern)
			continue
		}
		matches, err := dataStore.MatchingKeys(md.FromPattern)
		if err != nil {
			t.to.StatusLog().Error("store %s keymatch %s: %s", md.FromStore, md.FromPattern, err)
			continue
		}
		if len(matches) == 0 {
			t.to.StatusLog().Warn("store %s has no keys matching %s: data can not be installed in the volume", md.FromStore, md.FromPattern)
		}
	}
}

type dirDefinition struct {
	Path     string
	Perm     *os.FileMode
	User     string
	Group    string
	Required bool
}

func (t *DataRecv) InstallFromDatastore(ctx context.Context, from object.DataStore) (bool, error) {
	if len(t.Install) == 0 {
		return false, nil
	}

	changed := false
	head := t.to.Head()
	path := t.to.GetObject().(object.Core).Path()
	dirs, files := t.getInstallMetadata(head)

	if len(dirs) == 0 && len(files) == 0 {
		return false, nil
	}

	if head == "" {
		return false, fmt.Errorf("refuse to install in empty (ie /) head")
	}

	for _, dir := range dirs {
		if err := t.installDir(dir.Path, head, *dir.Perm, dir.User, dir.Group); err != nil && dir.Required {
			return false, err
		}
	}

	signals := volsignal.New()

	for _, md := range files {
		if md.FromStore != from.Path() {
			continue
		}
		if !md.FromStore.Exists() {
			err := fmt.Errorf("store %s does not exist: key %s data can not be installed in the volume", md.FromStore, md.FromPattern)
			if md.Required {
				return false, err
			} else {
				t.to.Log().Warnf("%s", err)
				continue
			}
		}
		shares := from.Shares()
		if !slices.Contains(shares, "*") && !slices.Contains(shares, path.Namespace) {
			path := strings.TrimPrefix(md.ToPath, md.ToHead)
			return false, fmt.Errorf("unauthorized install ...%s from %s key %s", path, md.FromStore, md.FromPattern)
		}
		md.ToLog = t.to.Log()
		if err := from.InstallKeyTo(md); err != nil && md.Required {
			return false, err
		}
		signals.Merge(md.Signals)
		changed = true
	}

	t.SendSignals(ctx, signals)

	return changed, nil
}

func (t *DataRecv) install(ctx context.Context) (bool, error) {
	if len(t.Install) == 0 {
		return false, nil
	}

	changed := false
	head := t.to.Head()
	path := t.to.GetObject().(object.Core).Path()
	dirs, files := t.getInstallMetadata(head)

	if len(dirs) == 0 && len(files) == 0 {
		return false, nil
	}

	if head == "" {
		return false, fmt.Errorf("refuse to install in empty (ie /) head")
	}

	for _, dir := range dirs {
		if err := t.installDir(dir.Path, head, *dir.Perm, dir.User, dir.Group); err != nil && dir.Required {
			return false, err
		}
	}

	signals := volsignal.New()

	for _, md := range files {
		if !md.FromStore.Exists() {
			err := fmt.Errorf("store %s does not exist: key %s data can not be installed in the volume", md.FromStore, md.FromPattern)
			if md.Required {
				return false, err
			} else {
				t.to.Log().Warnf("%s", err)
				continue
			}
		}
		dataStore, err := object.NewDataStore(md.FromStore, object.WithVolatile(true))
		if err != nil {
			t.to.Log().Warnf("store %s init error: %s", md.FromStore, err)
		}
		shares := dataStore.Shares()
		if !slices.Contains(shares, "*") && !slices.Contains(shares, path.Namespace) {
			path := strings.TrimPrefix(md.ToPath, md.ToHead)
			return false, fmt.Errorf("unauthorized install ...%s from %s key %s", path, md.FromStore, md.FromPattern)
		}
		md.ToLog = t.to.Log()
		if err = dataStore.InstallKeyTo(md); err != nil && md.Required {
			return false, err
		}
		signals.Merge(md.Signals)
		changed = true
	}

	t.SendSignals(ctx, signals)

	return changed, nil
}

func (t *DataRecv) getInstallMetadata(head string) ([]dirDefinition, []object.KVInstall) {
	path := t.to.GetObject().(object.Core).Path()
	files := make([]object.KVInstall, 0)
	dirs := make([]dirDefinition, 0)
	if head == "" {
		return dirs, files
	}

	isSep := func(word string, i int) bool {
		if !strings.HasPrefix(word, "/") {
			return false
		}
		return i < 1 || t.Install[i-1] != "key"
	}

	split := func() [][]string {
		var line []string
		lines := make([][]string, 0)
		in := false
		for i, word := range t.Install {
			if isSep(word, i) {
				if in {
					lines = append(lines, line)
					line = []string{word}
				} else {
					line = []string{word}
					in = true
				}
			} else {
				if in {
					line = append(line, word)
				} else {
					// ignore heading garbage
				}
			}
		}
		if len(line) >= 1 {
			lines = append(lines, line)
		}
		return lines
	}

	parseFileMode := func(s string) (*os.FileMode, error) {
		mode, err := strconv.ParseUint(s, 8, 32)
		if err != nil {
			return nil, err
		}
		fileMode := os.FileMode(mode)
		return &fileMode, nil
	}

	parseDir := func(line []string) {
		item := dirDefinition{
			User:  t.User,
			Group: t.Group,
			Perm:  t.getDirPerm(),
		}
		var word string

		word, line = pop(line)
		item.Path = word

		for {
			word, line = pop(line)
			if word == "" {
				break
			}
			switch word {
			case "user":
				word, line = pop(line)
				item.User = word
			case "group":
				word, line = pop(line)
				item.Group = word
			case "mode", "perm":
				word, line = pop(line)
				perm, _ := parseFileMode(word)
				if perm != nil {
					item.Perm = perm
				}
			case "required":
				item.Required = true
			}
		}
		dirs = append(dirs, item)
	}

	parseFile := func(line []string) {
		item := object.KVInstall{
			Required:    false,
			ToHead:      head,
			ToPath:      head,
			FromPattern: "",
			AccessControl: object.KVInstallAccessControl{
				User:         t.User,
				Group:        t.Group,
				MakedirUser:  t.User,
				MakedirGroup: t.Group,
				MakedirPerm:  t.getDirPerm(),
			},
			Signals: volsignal.New(),
		}

		var word string

		word, line = pop(line)
		item.ToPath = filepath.Join(head, word)
		if strings.HasSuffix(word, "/") {
			item.ToPath += "/"
		}

		word, line = pop(line)
		if word != "from" {
			return
		}

		word, line = pop(line)
		fromStore, err := naming.ParsePathRel(word, path.Namespace)
		if err != nil {
			return
		}

		switch fromStore.Kind {
		case naming.KindSec:
			item.AccessControl.Perm = &defaultSecPerm
		case naming.KindCfg:
			item.AccessControl.Perm = &defaultCfgPerm
		default:
			return
		}

		item.FromStore = fromStore

		for {
			word, line = pop(line)
			if word == "" {
				break
			}
			switch word {
			case "key":
				word, line = pop(line)
				item.FromPattern = word
			case "user":
				word, line = pop(line)
				item.AccessControl.User = word
			case "group":
				word, line = pop(line)
				item.AccessControl.Group = word
			case "mode", "perm":
				word, line = pop(line)
				perm, _ := parseFileMode(word)
				if perm != nil {
					item.AccessControl.Perm = perm
				}
			case "required":
				item.Required = true
			case "signal":
				word, line = pop(line)
				item.Signals.Parse(word)
			}
		}

		files = append(files, item)
	}

	for _, line := range split() {
		if slices.Contains(line, "from") {
			parseFile(line)
		} else {
			parseDir(line)
		}
	}
	return dirs, files
}

func (t *DataRecv) Do(ctx context.Context) error {
	changed := false

	if v, err := t.install(ctx); err != nil {
		return err
	} else {
		changed = v || changed
	}
	if err := t.installDirs(); err != nil {
		return err
	}
	if v, err := t.InstallDataByKind(naming.KindSec); err != nil {
		return err
	} else {
		changed = v || changed
	}
	if v, err := t.InstallDataByKind(naming.KindCfg); err != nil {
		return err
	} else {
		changed = v || changed
	}
	if changed {
		// Compat with old `signal` keyword.
		// Having signal=HUP:container#1,container#2 inside the `install`
		// keyword is now preferred.
		return t.OldSendSignals(ctx)
	}
	return nil
}

func (t *DataRecv) OldSendSignals(ctx context.Context) error {
	return t.SendSignals(ctx, volsignal.New(t.Signal))
}

func (t *DataRecv) SendSignals(ctx context.Context, signals *volsignal.T) error {
	if signals == nil {
		return nil
	}
	o, ok := t.to.GetObject().(signaler)
	if !ok {
		return fmt.Errorf("%s does not implement SignalResource()", o.Path())
	}
	for _, sd := range signals.Routes() {
		if err := o.SignalResource(ctx, sd.RID, sd.Signum); err != nil {
			return err
		}
		t.to.Log().Tracef("resource %s has been sent a signal %s", sd.RID, unix.SignalName(sd.Signum))
	}
	return nil
}

func (t *DataRecv) InstallDataByKind(kind naming.Kind) (bool, error) {
	var changed bool
	head := t.to.Head()
	path := t.to.GetObject().(object.Core).Path()

	for _, md := range t.getMetadataByKind(head, kind) {
		if !md.FromStore.Exists() {
			t.to.Log().Warnf("store %s does not exist: key %s data can not be installed in the volume", md.FromStore, md.FromPattern)
			continue
		}
		dataStore, err := object.NewDataStore(md.FromStore, object.WithVolatile(true))
		if err != nil {
			t.to.Log().Warnf("store %s init error: %s", md.FromStore, err)
		}
		shares := dataStore.Shares()
		if !slices.Contains(shares, "*") && !slices.Contains(shares, path.Namespace) {
			path := strings.TrimPrefix(md.ToPath, md.ToHead)
			return false, fmt.Errorf("unauthorized install ...%s from %s key %s", path, md.FromStore, md.FromPattern)
		}
		md.ToLog = t.to.Log()
		if err = dataStore.InstallKeyTo(md); err != nil {
			return changed, err
		}
		changed = true
	}
	return changed, nil
}

func (t *DataRecv) chmod(p string, mode *os.FileMode) error {
	if mode == nil {
		return nil
	}
	return os.Chmod(p, *mode)
}

func (t *DataRecv) chown(p string, usr, grp string, info os.FileInfo) error {
	var uid, gid int
	if usr != "" {
		if i, err := strconv.Atoi(usr); err == nil {
			uid = i
		} else if u, err := user.Lookup(usr); err == nil {
			uid, _ = strconv.Atoi(u.Uid)
		} else {
			return fmt.Errorf("user %s is not numeric and not resolved", usr)
		}
	}
	if grp != "" {
		if i, err := strconv.Atoi(grp); err == nil {
			gid = i
		} else if g, err := user.LookupGroup(grp); err == nil {
			gid, _ = strconv.Atoi(g.Gid)
		} else {
			return fmt.Errorf("group %s is not numeric and not resolved", grp)
		}
	}

	if info != nil {
		if stat, ok := info.Sys().(*syscall.Stat_t); ok {
			currentUID := int(stat.Uid)
			currentGID := int(stat.Gid)

			if uid != currentUID || gid != currentGID {
				t.to.Log().Infof("change %s owner from %d:%d to %d:%d", p, currentUID, currentGID, uid, gid)
				return os.Chown(p, uid, gid)
			} else {
				return nil
			}
		}
	}

	return os.Chown(p, uid, gid)
}

func (t *DataRecv) installDir(path string, head string, perm os.FileMode, user, group string) error {
	if head == "" {
		return fmt.Errorf("refuse to install dir %s in /", path)
	}
	p := filepath.Join(head, path)
	info, err := os.Stat(p)
	switch {
	case os.IsNotExist(err):
		t.to.Log().Infof("install directory %s with ower %s:%s and perm %s", p, user, group, perm)
		if err := os.MkdirAll(p, perm); err != nil {
			return err
		}
		if err := t.chown(p, user, group, nil); err != nil {
			return err
		}
	case err != nil:
		return err
	default:
		if !info.IsDir() {
			return fmt.Errorf("directory path %s is already occupied by a non-directory", p)
		}
		if info.Mode().Perm() != perm {
			t.to.Log().Infof("change directory %s permissions from %s to %s", p, info.Mode().Perm(), perm)
			if err := t.chmod(p, &perm); err != nil {
				return err
			}
		}
		if err := t.chown(p, user, group, info); err != nil {
			return err
		}
	}
	return nil
}

func (t *DataRecv) installDirs() error {
	if len(t.Directories) == 0 {
		return nil
	}
	head := t.to.Head()
	if head == "" {
		return fmt.Errorf("refuse to install dirs in empty (ie /) head")
	}
	dirPerm := t.getDirPerm()
	for _, dir := range t.Directories {
		if err := t.installDir(dir, head, *dirPerm, t.User, t.Group); err != nil {
			return err
		}
	}
	return nil
}
