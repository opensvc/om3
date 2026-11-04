package pkgupdate

import (
	"bufio"
	"compress/gzip"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strings"

	"github.com/klauspost/compress/zstd"
)

type (
	// Package is a package a repository offers: its name, version and
	// architecture, where to download it, relative to the repository, and the
	// sha256 the repository metadata give for it.
	Package struct {
		Name    string
		Version string
		Arch    string
		Path    string
		SHA256  string
	}

	// Distro is what /etc/os-release says of the system, the variables
	// naming the repository of its packages.
	Distro struct {
		ID              string
		IDLike          []string
		VersionID       string
		VersionCodename string
	}
)

// ReadDistro reads the os-release file at path.
func ReadDistro(path string) (Distro, error) {
	f, err := os.Open(path)
	if err != nil {
		return Distro{}, err
	}
	defer func() { _ = f.Close() }()
	vars := make(map[string]string)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		k, v, ok := strings.Cut(strings.TrimSpace(scanner.Text()), "=")
		if !ok || strings.HasPrefix(k, "#") {
			continue
		}
		vars[k] = strings.Trim(v, `"'`)
	}
	if err := scanner.Err(); err != nil {
		return Distro{}, err
	}
	codename := vars["VERSION_CODENAME"]
	if s := vars["UBUNTU_CODENAME"]; s != "" {
		// A derivative of ubuntu takes the packages of the ubuntu it is made of.
		codename = s
	}
	return Distro{
		ID:              vars["ID"],
		IDLike:          strings.Fields(vars["ID_LIKE"]),
		VersionID:       vars["VERSION_ID"],
		VersionCodename: codename,
	}, nil
}

func (t Distro) is(ids ...string) bool {
	for _, id := range ids {
		if t.ID == id {
			return true
		}
		for _, like := range t.IDLike {
			if like == id {
				return true
			}
		}
	}
	return false
}

// AptRepository returns the os directory and the suite of the apt
// repository of the opensvc packages of branch: ubuntu, prod-opensvc-v3-noble.
func (t Distro) AptRepository(branch string) (string, string, error) {
	var id string
	switch {
	case t.ID == "debian" || t.ID == "ubuntu":
		id = t.ID
	case t.is("ubuntu"):
		id = "ubuntu"
	case t.is("debian"):
		id = "debian"
	default:
		return "", "", fmt.Errorf("no opensvc apt repository for the distribution %s", t.ID)
	}
	if t.VersionCodename == "" {
		return "", "", fmt.Errorf("no VERSION_CODENAME in the os-release of %s", t.ID)
	}
	return id, fmt.Sprintf("%s-opensvc-v3-%s", branch, t.VersionCodename), nil
}

// RPMRepository returns the directory of the rpm repository of the opensvc
// packages of branch: prod-opensvc-v3-rhel9. The Red Hat rebuilds take the
// rhel packages, the SuSE distributions the sles ones, by major version.
func (t Distro) RPMRepository(branch string) (string, error) {
	major, _, _ := strings.Cut(t.VersionID, ".")
	if major == "" || len(major) > 2 {
		return "", fmt.Errorf("no opensvc rpm repository for the version %q of %s", t.VersionID, t.ID)
	}
	switch {
	case t.is("rhel", "centos", "fedora", "rocky", "almalinux", "ol"):
		return fmt.Sprintf("%s-opensvc-v3-rhel%s", branch, major), nil
	case t.is("sles", "sled", "suse") || strings.HasPrefix(t.ID, "opensuse"):
		return fmt.Sprintf("%s-opensvc-v3-sles%s", branch, major), nil
	}
	return "", fmt.Errorf("no opensvc rpm repository for the distribution %s", t.ID)
}

// DebArch is the debian name of the architecture the agent runs on.
func DebArch() string {
	switch runtime.GOARCH {
	case "386":
		return "i386"
	case "arm":
		return "armhf"
	}
	return runtime.GOARCH
}

// RPMArch is the rpm name of the architecture the agent runs on.
func RPMArch() string {
	switch runtime.GOARCH {
	case "amd64":
		return "x86_64"
	case "arm64":
		return "aarch64"
	case "386":
		return "i686"
	}
	return runtime.GOARCH
}

func fetch(ctx context.Context, client *http.Client, u string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("get %s: %s", u, resp.Status)
	}
	return resp.Body, nil
}

// decompress reads r as the file name says it is compressed.
func decompress(name string, r io.Reader) (io.Reader, func(), error) {
	switch {
	case strings.HasSuffix(name, ".gz"):
		gz, err := gzip.NewReader(r)
		if err != nil {
			return nil, nil, err
		}
		return gz, func() { _ = gz.Close() }, nil
	case strings.HasSuffix(name, ".zst"):
		zr, err := zstd.NewReader(r)
		if err != nil {
			return nil, nil, err
		}
		return zr, zr.Close, nil
	case strings.HasSuffix(name, ".xml"), !strings.Contains(name[strings.LastIndex(name, "/")+1:], "."):
		return r, func() {}, nil
	}
	return nil, nil, fmt.Errorf("%s: unsupported compression", name)
}

// AptIndex returns the packages of the apt repository at base (the url of the
// os directory, as https://packages.opensvc.com/apt/ubuntu) for suite and
// arch, those of all architectures included.
func AptIndex(ctx context.Context, client *http.Client, base, suite, arch string) ([]Package, error) {
	u, err := url.JoinPath(base, "dists", suite, "main", "binary-"+arch, "Packages.gz")
	if err != nil {
		return nil, err
	}
	body, err := fetch(ctx, client, u)
	if err != nil {
		return nil, err
	}
	defer func() { _ = body.Close() }()
	r, done, err := decompress(u, body)
	if err != nil {
		return nil, err
	}
	defer done()
	l, err := parseAptPackages(r)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", u, err)
	}
	return keepArch(l, arch, "all"), nil
}

// parseAptPackages reads the stanzas of an apt Packages file.
func parseAptPackages(r io.Reader) ([]Package, error) {
	var (
		l   []Package
		cur Package
	)
	flush := func() {
		if cur.Name != "" && cur.Version != "" && cur.Path != "" {
			l = append(l, cur)
		}
		cur = Package{}
	}
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			flush()
			continue
		}
		if line[0] == ' ' || line[0] == '\t' {
			// The continuation of a multi-line field, as the description.
			continue
		}
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		v = strings.TrimSpace(v)
		switch k {
		case "Package":
			cur.Name = v
		case "Version":
			cur.Version = v
		case "Architecture":
			cur.Arch = v
		case "Filename":
			cur.Path = v
		case "SHA256":
			cur.SHA256 = v
		}
	}
	flush()
	return l, scanner.Err()
}

// RPMIndex returns the packages of the rpm repository at base (the url of the
// directory holding repodata/) for arch, those of no architecture included.
func RPMIndex(ctx context.Context, client *http.Client, base, arch string) ([]Package, error) {
	u, err := url.JoinPath(base, "repodata", "repomd.xml")
	if err != nil {
		return nil, err
	}
	body, err := fetch(ctx, client, u)
	if err != nil {
		return nil, err
	}
	primary, err := parseRepomd(body)
	_ = body.Close()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", u, err)
	}
	u, err = url.JoinPath(base, primary)
	if err != nil {
		return nil, err
	}
	body, err = fetch(ctx, client, u)
	if err != nil {
		return nil, err
	}
	defer func() { _ = body.Close() }()
	r, done, err := decompress(u, body)
	if err != nil {
		return nil, err
	}
	defer done()
	l, err := parsePrimary(r)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", u, err)
	}
	return keepArch(l, arch, "noarch"), nil
}

// parseRepomd returns the location of the primary metadata the repomd.xml
// of a rpm repository names.
func parseRepomd(r io.Reader) (string, error) {
	var doc struct {
		Data []struct {
			Type     string `xml:"type,attr"`
			Location struct {
				Href string `xml:"href,attr"`
			} `xml:"location"`
		} `xml:"data"`
	}
	if err := xml.NewDecoder(r).Decode(&doc); err != nil {
		return "", err
	}
	for _, d := range doc.Data {
		if d.Type == "primary" && d.Location.Href != "" {
			return d.Location.Href, nil
		}
	}
	return "", fmt.Errorf("no primary metadata")
}

// parsePrimary reads the packages of the primary metadata of a rpm repository.
func parsePrimary(r io.Reader) ([]Package, error) {
	type pkg struct {
		Name    string `xml:"name"`
		Arch    string `xml:"arch"`
		Version struct {
			Epoch string `xml:"epoch,attr"`
			Ver   string `xml:"ver,attr"`
			Rel   string `xml:"rel,attr"`
		} `xml:"version"`
		Checksum struct {
			Type  string `xml:"type,attr"`
			Value string `xml:",chardata"`
		} `xml:"checksum"`
		Location struct {
			Href string `xml:"href,attr"`
		} `xml:"location"`
	}
	var l []Package
	dec := xml.NewDecoder(r)
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return l, nil
		}
		if err != nil {
			return nil, err
		}
		start, ok := tok.(xml.StartElement)
		if !ok || start.Name.Local != "package" {
			continue
		}
		var p pkg
		if err := dec.DecodeElement(&p, &start); err != nil {
			return nil, err
		}
		version := p.Version.Ver
		if p.Version.Rel != "" {
			version += "-" + p.Version.Rel
		}
		if p.Version.Epoch != "" && p.Version.Epoch != "0" {
			version = p.Version.Epoch + ":" + version
		}
		e := Package{Name: p.Name, Version: version, Arch: p.Arch, Path: p.Location.Href}
		if p.Checksum.Type == "sha256" {
			e.SHA256 = strings.TrimSpace(p.Checksum.Value)
		}
		l = append(l, e)
	}
}

func keepArch(l []Package, arches ...string) []Package {
	out := l[:0]
	for _, p := range l {
		for _, arch := range arches {
			if p.Arch == arch {
				out = append(out, p)
				break
			}
		}
	}
	return out
}

// Latest returns the newest package named name in l, by compare.
func Latest(l []Package, name string, compare func(a, b string) int) (Package, bool) {
	var (
		best  Package
		found bool
	)
	for _, p := range l {
		if p.Name != name {
			continue
		}
		if !found || compare(p.Version, best.Version) > 0 {
			best, found = p, true
		}
	}
	return best, found
}
