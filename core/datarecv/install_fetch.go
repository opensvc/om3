package datarecv

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/volsignal"
	"github.com/opensvc/om3/v3/util/confined"
	"github.com/opensvc/om3/v3/util/converters"
	"github.com/opensvc/om3/v3/util/file"
	"github.com/opensvc/om3/v3/util/key"
	"github.com/opensvc/om3/v3/util/sizeconv"
	"github.com/opensvc/om3/v3/util/uri"
	"github.com/opensvc/om3/v3/util/volfile"
)

type (
	// fetchDefinition is an install line naming a source and no datastore:
	// the file at Path, relative to the head, is fetched from the https url
	// straight into the volume, as a file too large for a key, the web ui
	// of an application say, is.
	fetchDefinition struct {
		Path     string
		URL      string
		SHA256   string
		Perm     os.FileMode
		User     string
		Group    string
		Signals  *volsignal.T
		Required bool
	}

	// cappedReader reads what a fetch sends, max bytes at most: a server
	// announcing no length, or one smaller than what it sends, is cut there
	// rather than fill the volume.
	cappedReader struct {
		r    io.Reader
		left int64
		max  int64
	}

	// checksumReader reads what a fetch sends, and at its end says whether
	// it is of the sha256 the line names: a body of another is an error, so
	// the file is not renamed in place.
	checksumReader struct {
		r    io.Reader
		h    hash.Hash
		want string
	}
)

var (
	// fetchHTTP fetches the url of an install line, held to the
	// node.allowed_urls and SSRF policies, as long as the action fetching
	// it lasts. The tests replace it.
	fetchHTTP = func(ctx context.Context, u string) (*http.Response, error) {
		client, err := uri.SafeHttpClient(u)
		if err != nil {
			return nil, err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, err
		}
		return client.Do(req)
	}

	// installSizeMax returns the most a file fetched holds,
	// node.max_install_size. The tests replace it.
	installSizeMax = func() (int64, error) {
		n, err := object.NewNode(object.WithVolatile(true))
		if err != nil {
			return 0, err
		}
		v, err := n.MergedConfig().GetSizeStrict(key.New("node", "max_install_size"))
		if err != nil {
			return 0, err
		}
		return *v, nil
	}
)

// lineID names the line as the record of the files the install lines hold
// does: the url, and where. A line whose url changes is another line, whose
// file is fetched, and the line of the former url is no longer configured.
func (t fetchDefinition) lineID() string {
	return "url:" + t.URL + ":" + t.Path
}

// isFetchLine says whether an install line fetches its file from a url: it
// names a source, and no datastore to seed a key of from it.
func isFetchLine(line []string) bool {
	return !slices.Contains(line, "from") && slices.Contains(line, "source")
}

// fetchLineErr refuses a line fetching a file it can not fetch:
//
//   - a destination naming a directory: the file fetched is named, its
//     name not being the one of the url, as the latest download of a
//     release has.
//
//   - a source other than an https url: an http fetch is anyone's on the
//     way to alter, and a path or random are the sources of a key, which a
//     line names with from.
//
//   - a key, or a template: the file is no key, and is installed as it is
//     fetched.
//
//   - a sha256 other than 64 hexadecimal digits.
func fetchLineErr(line []string) error {
	dest := line[0]
	var source, sum string
	var words []string
	for i := 1; i < len(line); i++ {
		switch line[i] {
		case "source":
			if i+1 < len(line) {
				source = line[i+1]
				i++
			}
		case "sha256":
			if i+1 < len(line) {
				sum = line[i+1]
				i++
			}
		case "user", "group", "mode", "perm", "signal":
			i++
		default:
			words = append(words, line[i])
		}
	}
	prefix := fmt.Sprintf("install %s source %s", dest, source)
	switch {
	case strings.HasSuffix(dest, "/") || filepath.Clean("/"+dest) == "/":
		return fmt.Errorf("%s: a line naming a source and no datastore fetches a file, named by its destination: not a directory", prefix)
	case !isHttpsURL(source):
		return fmt.Errorf("%s: a line naming no datastore fetches its file from an https:// url, the source of a key being named with from <datastore>", prefix)
	case slices.Contains(words, "key"):
		return fmt.Errorf("%s: a file fetched is no key: name a datastore with from <datastore> to install a key", prefix)
	case slices.Contains(words, "template"):
		return fmt.Errorf("%s: a file fetched is installed as it is: a template is a key, rendered from a datastore named with from <datastore>", prefix)
	case sum != "" && !isSHA256(sum):
		return fmt.Errorf("%s: sha256 %s: 64 hexadecimal digits expected", prefix, sum)
	}
	return nil
}

func isHttpsURL(s string) bool {
	if !uri.IsValidHttp(s) {
		return false
	}
	u, err := url.Parse(s)
	return err == nil && u.Scheme == "https"
}

func isSHA256(s string) bool {
	b, err := hex.DecodeString(s)
	return err == nil && len(b) == sha256.Size
}

// parseFetchLine reads a line fetching a file, its mode, user and group
// defaulting to the perm, user and group keywords, as the ones of a cfg key.
func (t *DataRecv) parseFetchLine(line []string) (fetchDefinition, error) {
	if err := fetchLineErr(line); err != nil {
		return fetchDefinition{}, err
	}
	item := fetchDefinition{
		Path:    filepath.Clean("/" + line[0]),
		Perm:    t.getPerm(naming.KindCfg),
		User:    t.User,
		Group:   t.Group,
		Signals: volsignal.New(),
	}
	var word string
	words := line[1:]
	for {
		word, words = Pop(words)
		if word == "" {
			break
		}
		switch word {
		case "user":
			item.User, words = Pop(words)
		case "group":
			item.Group, words = Pop(words)
		case "mode", "perm":
			word, words = Pop(words)
			if v, err := converters.FileMode.Convert(word); err == nil {
				if mode, ok := v.(*os.FileMode); ok && mode != nil {
					item.Perm = *mode
				}
			}
		case "signal":
			word, words = Pop(words)
			item.Signals.Parse(word)
		case "source":
			item.URL, words = Pop(words)
		case "sha256":
			word, words = Pop(words)
			item.SHA256 = strings.ToLower(word)
		case "required":
			item.Required = true
		}
	}
	return item, nil
}

// fetchOwner returns the uid and gid of the user and group of the line, -1 for
// one not set, which the install leaves as it is.
func (t *DataRecv) fetchOwner(fd fetchDefinition) (uid, gid int, err error) {
	uid, gid = -1, -1
	if fd.User != "" {
		if uid, err = t.uid(fd.User); err != nil {
			return
		}
	}
	if fd.Group != "" {
		if gid, err = t.gid(fd.Group); err != nil {
			return
		}
	}
	return
}

// installFetch installs the file of a line fetching it, and says whether it
// fetched it.
//
// The file is fetched when it is missing, or not the one of the url: its
// mark names another, or, on a filesystem keeping no mark, the record of
// the files the lines hold does not say this line fetched it. A file of the
// url is kept as it is, its mode and owner set: a start fetches nothing, and
// a url naming the latest release is fetched again when the file is deleted,
// or the url changed for the one of a version.
func (t *DataRecv) installFetch(ctx context.Context, head string, fd fetchDefinition) (bool, error) {
	dst := filepath.Join(head, fd.Path)
	tree, err := confined.Open(head)
	if err != nil {
		return false, fmt.Errorf("install %s from %s: %w", dst, fd.URL, err)
	}
	defer func() { _ = tree.Close() }()
	info, err := tree.Lstat(dst)
	switch {
	case errors.Is(err, os.ErrNotExist):
	case err != nil:
		return false, fmt.Errorf("install %s from %s: %w", dst, fd.URL, err)
	case info.IsDir():
		// The directory the line made, before it fetched: an empty
		// one is replaced by the file.
		if err := tree.Remove(dst); err != nil {
			return false, fmt.Errorf("install %s from %s: a directory is there: %w", dst, fd.URL, err)
		}
		t.to.Log().Infof("remove the empty directory %s to install the file fetched from %s", dst, fd.URL)
	case t.isFetched(tree, dst, info, fd):
		if reason := t.changedSinceFetched(tree, dst, fd); reason != "" {
			t.to.Log().Warnf("%s: fetch it again from %s", reason, fd.URL)
			break
		}
		return false, t.keepFetched(tree, dst, info, fd)
	}
	if err := t.makeFetchDir(tree, filepath.Dir(dst)); err != nil {
		return false, fmt.Errorf("install %s from %s: %w", dst, fd.URL, err)
	}
	if err := t.fetch(ctx, head, fd); err != nil {
		return false, fmt.Errorf("install %s from %s: %w", dst, fd.URL, err)
	}
	return true, nil
}

// isFetched says whether the file at dst is the one the line fetched from
// its url.
func (t *DataRecv) isFetched(tree *confined.Tree, dst string, info os.FileInfo, fd fetchDefinition) bool {
	if !info.Mode().IsRegular() {
		return false
	}
	if b, err := tree.GetXattr(dst, volfile.InstallAttr); err == nil {
		var m volfile.InstallMark
		if json.Unmarshal(b, &m) == nil {
			return m.From == fd.URL && m.Key == ""
		}
	}
	return slices.Contains(readInstallFiles(t.installFilesFile())[fd.lineID()], fd.Path)
}

// keepFetched gives the file of the url kept the mode and the owner of the
// line, and its mark when it has none, as a file the record alone says the
// line fetched: an upload looks for the mark, not the record.
func (t *DataRecv) keepFetched(tree *confined.Tree, dst string, info os.FileInfo, fd fetchDefinition) error {
	if t.report {
		t.to.Log().Infof("%s from %s is up to date", dst, fd.URL)
	}
	uid, gid, err := t.fetchOwner(fd)
	if err != nil {
		return fmt.Errorf("install %s from %s: %w", dst, fd.URL, err)
	}
	chowned := false
	if currentUID, currentGID, differs := ownerDiffers(info, uid, gid); differs {
		t.to.Log().Infof("change %s owner from %d:%d to %d:%d", dst, currentUID, currentGID, uid, gid)
		if err := tree.Lchown(dst, uid, gid); err != nil {
			return fmt.Errorf("install %s from %s: %w", dst, fd.URL, err)
		}
		chowned = true
	}
	// Set again after a chown, which drops the setuid and setgid bits.
	if current := info.Mode() & file.ModeBits; current != fd.Perm || chowned {
		if current != fd.Perm {
			t.to.Log().Infof("change %s permissions from %s to %s", dst, current, fd.Perm)
		}
		if err := tree.Chmod(dst, fd.Perm); err != nil {
			return fmt.Errorf("install %s from %s: %w", dst, fd.URL, err)
		}
	}
	if _, err := tree.GetXattr(dst, volfile.InstallAttr); err != nil {
		if b, err := json.Marshal(volfile.InstallMark{From: fd.URL, At: time.Now()}); err == nil {
			_ = tree.SetXattr(dst, volfile.InstallAttr, b)
		}
	}
	return nil
}

// makeFetchDir makes the directory of the file fetched, as the install of a
// key makes the one of its file: each directory missing on the way with the
// dirperm, user and group keywords.
func (t *DataRecv) makeFetchDir(tree *confined.Tree, dir string) error {
	if _, err := tree.Lstat(dir); err == nil {
		return nil
	}
	perm := t.getDirPerm()
	t.to.Log().Infof("install dir %s with owner %s:%s perm %v", dir, t.User, t.Group, perm)
	return confined.MkdirAllEach(tree, dir, perm, func(p string) error {
		if err := t.chmod(tree, p, &perm); err != nil {
			return err
		}
		return t.chown(tree, p, t.User, t.Group, nil)
	})
}

// fetch writes the body of the url of the line to its file, through
// volfile.Install: the file appears whole or not at all, a fetch failing
// leaving the one fetched before.
func (t *DataRecv) fetch(ctx context.Context, head string, fd fetchDefinition) error {
	maxSize, err := installSizeMax()
	if err != nil {
		return err
	}
	uid, gid, err := t.fetchOwner(fd)
	if err != nil {
		return err
	}
	resp, err := fetchHTTP(ctx, fd.URL)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	if resp.ContentLength > maxSize {
		return fmt.Errorf("%sB announced, over %sB (raise node.max_install_size)", sizeconv.BSize(float64(resp.ContentLength)), sizeconv.BSize(float64(maxSize)))
	}
	var r io.Reader = &cappedReader{r: resp.Body, left: maxSize, max: maxSize}
	if fd.SHA256 != "" {
		r = &checksumReader{r: r, h: sha256.New(), want: fd.SHA256}
	}
	h := sha256.New()
	r = io.TeeReader(r, h)
	t.to.Log().Infof("fetch %s to %s with owner %s:%s perm %v", fd.URL, filepath.Join(head, fd.Path), fd.User, fd.Group, fd.Perm)
	rel := strings.TrimPrefix(fd.Path, "/")
	m := volfile.InstallMark{From: fd.URL, At: time.Now()}
	if err := volfile.Install(head, path.Dir(rel), path.Base(rel), r, m, fd.Perm, uid, gid); err != nil {
		return err
	}
	return t.recordFetched(fd, hex.EncodeToString(h.Sum(nil)))
}

// fetchedRecord is what a node records of a file it fetched: the url, and the
// digest of the content written. A volume holding no mark, as a filesystem
// with no extended attributes, is checked against it.
type fetchedRecord struct {
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
}

func (t *DataRecv) fetchedFile() string {
	if f := t.installFilesFile(); f != "" {
		return f + "_fetched"
	}
	return ""
}

func (t *DataRecv) readFetched() map[string]fetchedRecord {
	m := make(map[string]fetchedRecord)
	if f := t.fetchedFile(); f != "" {
		if b, err := os.ReadFile(f); err == nil {
			_ = json.Unmarshal(b, &m)
		}
	}
	return m
}

// recordFetched records the digest of the file the line fetched.
func (t *DataRecv) recordFetched(fd fetchDefinition, digest string) error {
	f := t.fetchedFile()
	if f == "" {
		return nil
	}
	m := t.readFetched()
	m[fd.Path] = fetchedRecord{URL: fd.URL, SHA256: digest}
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
		return err
	}
	return os.WriteFile(f, b, 0o600)
}

// changedSinceFetched says why the file at dst is not the content the line
// fetched, empty when it is: the digest the line names, else the one of the
// mark, else the one this node recorded, of the same url. A file written
// over, by a user of the volume or an upload a volume with no mark let in,
// is fetched again rather than served as the url's. A file of no known
// digest, fetched before one was recorded, is kept.
func (t *DataRecv) changedSinceFetched(tree *confined.Tree, dst string, fd fetchDefinition) string {
	want := fd.SHA256
	marked := false
	if want == "" {
		if b, err := tree.GetXattr(dst, volfile.InstallAttr); err == nil {
			var m volfile.InstallMark
			if json.Unmarshal(b, &m) == nil && m.From == fd.URL {
				marked = true
				want = m.SHA256
			}
		}
	}
	if want == "" {
		if r, ok := t.readFetched()[fd.Path]; ok && r.URL == fd.URL {
			want = r.SHA256
		}
	}
	switch {
	case want != "":
	case marked:
		// Fetched before the mark held a digest: kept as it was.
		return ""
	default:
		// No mark, and the record of its digest lost: nothing tells
		// the file is the url's, so it is fetched again.
		return fmt.Sprintf("no digest of %s is known", dst)
	}
	f, err := tree.Open(dst)
	if err != nil {
		return fmt.Sprintf("%s can not be read: %s", dst, err)
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Sprintf("%s can not be read: %s", dst, err)
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != want {
		return fmt.Sprintf("%s changed since it was fetched, sha256 %s expected, got %s", dst, want, got)
	}
	return ""
}

func (t *cappedReader) Read(p []byte) (int, error) {
	if t.left < 0 {
		return 0, fmt.Errorf("more than %sB sent (raise node.max_install_size)", sizeconv.BSize(float64(t.max)))
	}
	// One byte past the max tells a body larger from one of the max.
	if int64(len(p)) > t.left+1 {
		p = p[:t.left+1]
	}
	n, err := t.r.Read(p)
	t.left -= int64(n)
	if t.left < 0 {
		return 0, fmt.Errorf("more than %sB sent (raise node.max_install_size)", sizeconv.BSize(float64(t.max)))
	}
	return n, err
}

func (t *checksumReader) Read(p []byte) (int, error) {
	n, err := t.r.Read(p)
	t.h.Write(p[:n])
	if errors.Is(err, io.EOF) {
		if got := hex.EncodeToString(t.h.Sum(nil)); got != t.want {
			return n, fmt.Errorf("sha256 %s expected, got %s", t.want, got)
		}
	}
	return n, err
}

// linesHolding returns the lines the record says hold the file at p: a
// fetch failing for a url changed leaves the file of the former url, until
// one of the new url replaces it.
func (t *DataRecv) linesHolding(p string) []string {
	l := make([]string, 0)
	for id, files := range readInstallFiles(t.installFilesFile()) {
		if slices.Contains(files, p) {
			l = append(l, id)
		}
	}
	return l
}

// fetchIDs returns the lines fetching a file, and the ones holding the file
// of each, which a refresh of a datastore, fetching nothing, leaves.
func (t *DataRecv) fetchIDs() []string {
	l := make([]string, 0, len(t.fetches))
	for _, fd := range t.fetches {
		l = append(l, fd.lineID())
		l = append(l, t.linesHolding(fd.Path)...)
	}
	return l
}

// statusFetches warns of the file of a required line fetching it missing.
func (t *DataRecv) statusFetches(head string) {
	if head == "" {
		return
	}
	tree, err := confined.Open(head)
	if err != nil {
		return
	}
	defer func() { _ = tree.Close() }()
	for _, fd := range t.fetches {
		if !fd.Required {
			continue
		}
		dst := filepath.Join(head, fd.Path)
		if _, err := tree.Lstat(dst); errors.Is(err, os.ErrNotExist) {
			t.to.StatusLog().Warn("%s does not exist: fetched from %s", dst, fd.URL)
		}
	}
}
