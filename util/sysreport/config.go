package sysreport

import (
	"archive/tar"
	"bufio"
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/anmitsu/go-shlex"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"golang.org/x/sys/unix"

	"github.com/opensvc/om3/core/collector"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/util/device"
	"github.com/opensvc/om3/util/file"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/timestamp"
	"github.com/opensvc/om3/util/xmap"
)

type (
	T struct {
		includes        []string
		excludes        []string
		commands        []string
		varDir          string
		etcDir          string
		configReader    io.Reader
		collectorClient *collector.Client
		force           bool

		// variable
		changed      map[string]interface{}
		full         map[string]interface{}
		deleted      []string
		statsChanged bool
		stats        statsMap
		statsStat    Stat

		// expanded is a map keyed by file path and value is the inc
		// pattern that lead to the entry.
		expanded map[string]string
	}
)

var (
	srLog   zerolog.Logger
	rootUID = 0
	rootGID = 0
)

func New() *T {
	srLog = log.With().Str("c", "sysreport").Logger()
	t := &T{
		etcDir:   filepath.Join(rawconfig.Paths.Etc, "sysreport.conf.d"),
		varDir:   filepath.Join(rawconfig.Paths.Var),
		excludes: []string{},
		commands: []string{},
		includes: []string{
			filepath.Join(rawconfig.Paths.Etc, "*.conf"),
			filepath.Join(rawconfig.Paths.Etc, "namespaces", "*", "*", "*.conf"),
			filepath.Join(rawconfig.Paths.Etc, "sysreport.conf.d"),
		},
	}
	return t
}

func (t *T) SetForce(v bool) {
	t.force = v
}

func (t *T) SetConfigReader(r io.Reader) {
	t.configReader = r
}

func (t *T) SetConfigDir(path string) {
	t.etcDir = path
}

func (t *T) SetCollectorClient(c *collector.Client) {
	t.collectorClient = c
}

func (t T) sysreportDir() string {
	return filepath.Join(t.varDir, "sysreport")
}

func (t T) collectDir() string {
	return filepath.Join(t.varDir, "sysreport", hostname.Hostname())
}

func (t T) collectCmdDir() string {
	return filepath.Join(t.collectDir(), "cmd")
}

func (t T) collectFileDir() string {
	return filepath.Join(t.collectDir(), "file")
}

func (t T) collectStatFile() string {
	return filepath.Join(t.collectDir(), "file", "stat")
}

func (t *T) init() error {
	t.changed = make(map[string]interface{})
	t.full = map[string]interface{}{
		t.collectStatFile(): nil,
	}
	if err := t.initDir(t.collectDir()); err != nil {
		return err
	}
	if err := t.initDir(t.collectCmdDir()); err != nil {
		return err
	}
	if err := t.initDir(t.collectFileDir()); err != nil {
		return err
	}
	if err := t.loadStat(); err != nil {
		return err
	}
	if err := t.loadConfigs(); err != nil {
		return err
	}
	if err := t.expand(); err != nil {
		return err
	}
	if err := t.findDeleted(); err != nil {
		return err
	}
	return nil
}

func (t *T) initDir(s string) error {
	if err := os.MkdirAll(s, 0700); err != nil {
		return fmt.Errorf("%s: %w", s, err)
	}
	if err := os.Chown(s, rootUID, rootGID); err != nil {
		return fmt.Errorf("%s: %w", s, err)
	}
	if err := os.Chmod(s, 0700); err != nil {
		return fmt.Errorf("%s: %w", s, err)
	}
	return nil
}

func (t *T) loadStat() error {
	path := t.collectStatFile()
	if !file.Exists(path) {
		t.stats = make(statsMap)
		return nil
	}
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("loading files stat cache: %w", err)
	}
	defer f.Close()
	if err := t.stats.Load(f); err != nil {
		return err
	}
	srLog.Debug().Str("path", path).Int("len", len(t.stats)).Msg("Load stat")
	return nil
}

func (t *T) writeStat() error {
	if !t.force && !t.statsChanged {
		return nil
	}
	path := t.collectStatFile()
	srLog.Debug().Str("path", path).Msg("Rewrite")
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("write stat: %w", err)
	}
	defer f.Close()
	if err := t.stats.Write(f); err != nil {
		return err
	}
	t.changed[path] = nil
	return nil
}

func (t *T) collectFile(path string) error {
	if !file.Exists(path) {
		return nil
	}
	if v, err := file.ExistsAndSymlink(path); err != nil {
		return err
	} else if v {
		return nil
	}
	dest := filepath.Join(t.collectFileDir(), path)
	destDir := filepath.Dir(dest)
	if v, err := file.ExistsAndDir(dest); err != nil {
		return err
	} else if v {
		// change from regular to dir ... clean up cache
		if err := t.unlinkAll(dest); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(destDir, 0700); err != nil {
		return err
	}
	buff, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("collect file: %w", err)
	}
	buff = obfuscateClusterSecret(dest, buff)
	if err := t.write(dest, buff); err != nil {
		return err
	}
	if err := file.CopyMeta(path, dest); err != nil {
		return err
	}
	if err := t.pushStat(dest); err != nil {
		return err
	}

	srLog.Debug().Str("path", path).Msg("Collected")
	t.full[path] = nil
	return nil
}

func obfuscateClusterSecret(path string, buff []byte) []byte {
	if filepath.Base(path) != "cluster.conf" {
		return buff
	}
	r := regexp.MustCompile(`secret.*=\s*([0-9A-Fa-f]+)`)
	matches := r.FindAllSubmatch(buff, -1)
	if matches == nil {
		return buff
	}
	for _, match := range matches {
		secret := match[1]
		sum := md5.Sum(secret)
		obfuscated := []byte("hexdigest-" + hex.EncodeToString(sum[:]))
		r := regexp.MustCompile(string(secret))
		buff = r.ReplaceAll(buff, obfuscated)
		srLog.Debug().Bytes("obfuscated", obfuscated).Msg("obfuscated cluster secret")
	}
	return buff
}

func (t *T) collectCommand(s string) error {
	argv, err := shlex.Split(s, true)
	if err != nil {
		return err
	}
	cmd := exec.Command(argv[0], argv[1:]...)
	b, _ := cmd.CombinedOutput()
	path := stupidCommandToPath(t.collectCmdDir(), argv)
	if err := t.write(path, b); err != nil {
		srLog.Error().Err(err).Str("cmd", s).Str("path", path).Msg("collect")
		return nil
	}
	srLog.Debug().Str("cmd", s).Str("path", path).Msg("collected")
	return nil
}

func (t *T) write(path string, buff []byte) error {
	var cached []byte
	if file.Exists(path) {
		if cached, _ = os.ReadFile(path); bytes.Compare(cached, buff) == 0 {
			//srLog.Debug().Str("path", path).Msg("mark full, unchanged")
			t.full[path] = nil
			return nil
		}
	}
	if err := os.WriteFile(path, buff, 0600); err != nil {
		return fmt.Errorf("%w", err)
	}
	//srLog.Debug().Str("path", path).Msg("changed")
	t.full[path] = nil
	t.changed[path] = nil
	return nil
}

func (t *T) loadConfigs() error {
	if t.configReader != nil {
		return t.loadConfigReader(t.configReader)
	}
	err := filepath.Walk(t.etcDir, func(path string, info fs.FileInfo, err error) error {
		if info == nil {
			srLog.Debug().Str("path", path).Msg("Ignore non existing config file")
			return nil
		}
		if info.Mode().IsDir() {
			return nil
		}
		if !info.Mode().IsRegular() {
			srLog.Debug().Str("path", path).Msg("Ignore non regular config file")
			return nil
		}
		if err := isConfigFileSecure(path); err != nil {
			srLog.Warn().Str("path", path).Msgf("Ignore non secure config file: %s", err)
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			srLog.Warn().Err(err).Str("path", path).Msg("Open config file")
			return nil
		}
		defer f.Close()
		if err := t.loadConfigReader(f); err != nil {
			srLog.Warn().Err(err).Str("cf", path).Msg("Load config file")
			return fmt.Errorf("%s: %w", path, err)
		}
		return nil
	})
	return err
}

func isConfigFileSecure(s string) error {
	info, err := os.Stat(s)
	if err != nil {
		return err
	}
	mode := info.Mode()
	if mode&0002 != 0 {
		return fmt.Errorf("%s: file mode is insecure ('other' has write permission)", s)
	}
	uid, gid, err := file.Ownership(s)
	if err != nil {
		return err
	}
	if uid != rootUID || gid != rootGID {
		return fmt.Errorf("%s: file ownership is insecure (Must be owned by %d:%d)", s, rootUID, rootGID)
	}
	return nil
}

func (t *T) loadConfigReader(r io.Reader) error {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "CMD"):
			t.commands = append(t.commands, strings.TrimSpace(line[3:]))
		case strings.HasPrefix(line, "EXC"):
			t.excludes = append(t.excludes, strings.TrimSpace(line[3:]))
		case strings.HasPrefix(line, "INC"):
			t.includes = append(t.includes, strings.TrimSpace(line[3:]))
		case strings.HasPrefix(line, "FILE"):
			t.includes = append(t.includes, strings.TrimSpace(line[4:]))
		case strings.HasPrefix(line, "DIR"):
			t.includes = append(t.includes, strings.TrimSpace(line[3:]))
		case strings.HasPrefix(line, "GLOB"):
			t.includes = append(t.includes, strings.TrimSpace(line[4:]))
		case line == "":
			continue
		case strings.HasPrefix(line, "#"):
			continue
		case strings.HasPrefix(line, ";"):
			continue
		default:
			srLog.Warn().Msgf("Unsupported item type: %s", line)
			continue
		}
	}
	return nil
}

func (t *T) findDeleted() error {
	head := t.collectFileDir()
	n := len(head)
	t.deleted = make([]string, 0)
	err := filepath.Walk(head, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			srLog.Error().Err(err).Str("path", path).Msg("findDeleted")
			return nil
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		path = path[n:]
		if path == "/stat" {
			return nil
		}
		if _, ok := t.expanded[path]; !ok {
			srLog.Debug().Str("path", path).Msg("Deleted file")
			t.statsDel(path)
			t.deleted = append(t.deleted, path)
		}
		return nil
	})
	if err != nil {
		return err
	}
	sort.Strings(t.deleted)
	return nil
}

func (t *T) expand() error {
	excludes := expand(t.excludes, nil)
	t.expanded = expand(t.includes, excludes)
	return nil
}

func sortedKeys(m interface{}) []string {
	l := xmap.Keys(m)
	sort.Strings(l)
	return l
}

func expand(in []string, excludes map[string]string) map[string]string {
	out := make(map[string]string, 0)
	if excludes == nil {
		excludes = make(map[string]string)
	}
	for _, s := range in {
		matches, err := filepath.Glob(s)
		if err != nil {
			srLog.Error().Err(err).Str("pattern", s).Msg("glob")
			continue
		}
		for _, match := range matches {
			err := filepath.Walk(match, func(path string, info fs.FileInfo, err error) error {
				if err != nil {
					srLog.Error().Err(err).Str("path", path).Msg("Expand")
					return nil
				}
				if !info.Mode().IsRegular() {
					return nil
				}
				if cf, ok := excludes[path]; ok {
					srLog.Debug().Str("path", path).Msgf("Excluded by %s", cf)
					return nil
				}
				//srLog.Debug().Str("path", path).Msgf("included by %s", s)
				out[path] = s
				return nil
			})
			if err != nil {
				srLog.Error().Err(err).Str("path", match).Msg("Walk")
				continue
			}
		}
	}
	return out
}

func (t T) filterLstree(lstreeData []string) []string {
	filtered := make([]string, 0)
	for _, path := range lstreeData {
		if path == "/stat" {
			continue
		}
		if _, ok := t.full[path]; ok {
			continue
		}
	}
	sort.Strings(filtered)
	return filtered
}

func (t T) getLstree() ([]string, error) {
	response, err := t.collectorClient.Call("sysreport_lstree")
	if err != nil {
		return nil, fmt.Errorf("collector sysreport_lstree call: %w", err)
	}
	if response.Error != nil {
		return nil, fmt.Errorf("collector sysreport_lstree response: %w", response.Error)
	}
	switch l := response.Result.(type) {
	case []interface{}:
		sl := make([]string, 0)
		for _, i := range l {
			if s, ok := i.(string); ok {
				sl = append(sl, s)
			}
		}
		return sl, nil
	case []string:
		return l, nil
	default:
		return nil, fmt.Errorf("unexpected sysreport_lstree rpc result: %+v", response.Result)
	}
}

func (t T) send() error {
	var toSend []string
	var deleted []string
	if t.force {
		toSend = sortedKeys(t.full)
		lstreeData, err := t.getLstree()
		if err != nil {
			return fmt.Errorf("can not get lstree from collector: %w", err)
		}
		deleted = t.filterLstree(lstreeData)
	} else {
		toSend = sortedKeys(t.changed)
		deleted = t.deleted
	}

	if len(toSend) == 0 && len(deleted) == 0 {
		srLog.Info().Msg("No change to report")
		return nil
	}

	var tmpf string
	var err error
	if len(toSend) > 0 {
		tmpf, err = t.archive(toSend)
		defer t.unlink(tmpf)
		if err != nil {
			return err
		}
	}

	b, err := os.ReadFile(tmpf)
	if err != nil {
		return err
	}
	response, err := t.collectorClient.Call("send_sysreport", filepath.Base(tmpf), b, deleted)
	if err != nil {
		return fmt.Errorf("send_sysreport call: %w", err)
	}
	if response.Error != nil {
		return fmt.Errorf("send_sysreport response: %w", response.Error)
	}
	srLog.Info().Int("size", len(b)).Msg("Report sent")
	return nil
}

// unlink is a paranoid wrapper around os.Remove, verifying the file we are
// asked to delete is under our responsibility.
func (t T) unlink(path string) error {
	base := t.collectDir()
	if !strings.HasPrefix(path, base) {
		return fmt.Errorf("abort unlink %s: not based on %s", path, base)
	}
	return os.Remove(path)
}

func (t T) unlinkAll(path string) error {
	base := t.collectDir()
	if !strings.HasPrefix(path, base) {
		return fmt.Errorf("abort recursive unlink %s: not based on %s", path, base)
	}
	return os.RemoveAll(path)
}

func relPath(base, path string) string {
	if strings.HasPrefix(path, base) {
		return path[len(base):]
	}
	return path
}

func doStat(path string) (Stat, error) {
	info, err := device.New(path).Stat()
	if err != nil {
		return Stat{}, err
	}
	stat := makeStat(path, info)
	return stat, nil
}

func (t *T) pushStat(path string) error {
	stat, err := doStat(path)
	if err != nil {
		return err
	}
	stat.Path = relPath(t.collectFileDir(), stat.Path)
	stat.RealPath = relPath(t.collectFileDir(), stat.RealPath)
	t.statsAdd(stat.Path, stat)
	return nil
}

func (t *T) statsAdd(path string, stat Stat) error {
	cachedStat, ok := t.stats[path]
	if !ok || !stat.IsEqual(cachedStat) {
		t.statsChanged = true
		t.stats[path] = stat
	}
	return nil
}

func (t *T) statsDel(path string) {
	if _, ok := t.stats[path]; !ok {
		return
	}
	delete(t.stats, path)
	t.statsChanged = true
	t.changed[t.collectStatFile()] = nil
}

func makeStat(path string, info unix.Stat_t) Stat {
	stat := Stat{
		Path:       path,
		RealPath:   path,
		Mode:       info.Mode,
		ModeOctStr: "0o" + strings.TrimLeft(fmt.Sprintf("%#o", info.Mode), "0"),
		MTime:      timestamp.New(time.Unix(info.Mtim.Sec, 0)),
		CTime:      timestamp.New(time.Unix(info.Ctim.Sec, 0)),
		Nlink:      info.Nlink,
		Dev:        info.Dev,
		UID:        info.Uid,
		GID:        info.Gid,
		Size:       info.Size,
	}
	if realPath, err := filepath.EvalSymlinks(path); err == nil {
		stat.RealPath = realPath
	}
	return stat
}

func (t T) statsGet(path string) (Stat, error) {
	switch {
	case path == "/stat":
		return t.statsStat, nil
	case strings.HasPrefix(path, t.collectCmdDir()):
		return doStat(path)
	default:
		stat, ok := t.stats[path]
		if !ok {
			return stat, fmt.Errorf("file %s not found in the file stats cache", path)
		}
		return stat, nil
	}
}

func (t T) archive(l []string) (string, error) {
	f, err := os.CreateTemp(t.collectDir(), "sysreport.*.tar")
	if err != nil {
		return "", err
	}
	tw := tar.NewWriter(f)
	sysreportDir := t.sysreportDir()
	n := len(sysreportDir) + 1
	for _, path := range l {
		if !strings.HasPrefix(path, sysreportDir) {
			continue
		}
		srLog.Info().Str("path", path).Msg("Add changed file to archive")
		statPath := relPath(t.collectFileDir(), path)
		stat, err := t.statsGet(statPath)
		if err != nil {
			return f.Name(), fmt.Errorf("%w", err)
		}
		r, err := os.Open(path)
		if err != nil {
			return f.Name(), fmt.Errorf("%w", err)
		}
		defer r.Close()
		hdr := &tar.Header{
			Name:    path[n:],
			Mode:    int64(stat.Mode),
			Size:    stat.Size,
			ModTime: stat.MTime.Time(),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return f.Name(), fmt.Errorf("%w", err)
		}
		if _, err = io.Copy(tw, r); err != nil {
			return f.Name(), fmt.Errorf("%w", err)
		}
	}
	if err := tw.Close(); err != nil {
		return f.Name(), fmt.Errorf("%w", err)
	}
	return f.Name(), nil
}

func (t *T) Do() error {
	if err := t.init(); err != nil {
		return err
	}
	if err := t.collectFiles(); err != nil {
		return err
	}
	if err := t.collectCommands(); err != nil {
		return err
	}
	if err := t.deleteCollected(); err != nil {
		return err
	}
	if err := t.writeStat(); err != nil {
		return err
	}
	if err := t.updateStatsStat(); err != nil {
		return err
	}
	if err := t.send(); err != nil {
		return err
	}
	return nil
}

func (t *T) collectFiles() error {
	for _, path := range sortedKeys(t.expanded) {
		if err := t.collectFile(path); err != nil {
			return err
		}
	}
	return nil
}

func (t *T) updateStatsStat() error {
	if stat, err := doStat(t.collectStatFile()); err != nil {
		return err
	} else {
		t.statsStat = stat
	}
	return nil
}

func (t *T) collectCommands() error {
	for _, command := range t.commands {
		if err := t.collectCommand(command); err != nil {
			return err
		}
	}
	return nil
}

func (t *T) deleteCollected() error {
	for _, path := range t.deleted {
		path = filepath.Join(t.collectFileDir(), path)
		if err := t.unlink(path); err != nil {
			return fmt.Errorf("delete cache of deleted file: %w", err)
		}
		srLog.Debug().Str("path", path).Msg("Delete cache of deleted file")
	}
	return nil
}

func stupidCommandToPath(baseDir string, l []string) string {
	s := stupidCommandToFilename(l)
	return filepath.Join(baseDir, s)
}

func stupidCommandToFilename(l []string) string {
	s := strings.Join(l, "(space)")
	s = strings.ReplaceAll(s, "|", "(pipe)")
	s = strings.ReplaceAll(s, "&", "(amp)")
	s = strings.ReplaceAll(s, "$", "(dollar)")
	s = strings.ReplaceAll(s, "^", "(caret)")
	s = strings.ReplaceAll(s, "/", "(slash)")
	s = strings.ReplaceAll(s, ":", "(colon)")
	s = strings.ReplaceAll(s, ";", "(semicolon)")
	s = strings.ReplaceAll(s, "<", "(lt)")
	s = strings.ReplaceAll(s, ">", "(gt)")
	s = strings.ReplaceAll(s, "=", "(eq)")
	s = strings.ReplaceAll(s, "?", "(question)")
	s = strings.ReplaceAll(s, "@", "(at)")
	s = strings.ReplaceAll(s, "!", "(excl)")
	s = strings.ReplaceAll(s, "#", "(num)")
	s = strings.ReplaceAll(s, "%", "(pct)")
	s = strings.ReplaceAll(s, "\"", "(dquote)")
	s = strings.ReplaceAll(s, "'", "(squote)")
	s = strings.ReplaceAll(s, "\\", "(bslash)")
	return s
}
