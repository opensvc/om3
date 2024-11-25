package rawconfig

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/msoap/byline"
	"github.com/subosito/gotenv"

	"github.com/opensvc/om3/util/capabilities"
	"github.com/opensvc/om3/util/render/palette"
)

const (
	// Program is the name of the project and module
	Program = "opensvc"
)

const (
	basenameCapabilities = "capabilities-v3.json"
)

var (
	Colorize *palette.ColorPaletteFunc
	Color    *palette.ColorPalette
	Paths    AgentPaths
)

func init() {
	Load(nil)
}

func Load(env map[string]string) {
	if env == nil {
		if m, err := readEnv(); err != nil {
			panic(err)
		} else {
			env = m
		}
	}

	var root string
	if s, ok := os.LookupEnv("OSVC_ROOT_PATH"); ok {
		root = s
	} else if env != nil {
		root = env["OSVC_ROOT_PATH"]
	}
	setPaths(root)

	var colors string
	if s, ok := os.LookupEnv("OSVC_COLORS"); ok {
		root = s
	} else if env != nil {
		root = env["OSVC_COLORS"]
	}
	setColors(colors)

	capabilities.SetCacheFile(Paths.Capabilities)
}

func readEnv() (map[string]string, error) {
	candidates := []string{
		filepath.FromSlash("/etc/sysconfig/" + Program),
		filepath.FromSlash("/etc/default/" + Program),
		filepath.FromSlash("/etc/defaults/" + Program),
	}
	for _, candidate := range candidates {
		reader, err := os.Open(candidate)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("%s: %s", candidate, err)
		}
		defer reader.Close()
		lr := byline.NewReader(reader)
		lr.GrepByRegexp(regexp.MustCompile(`^\s*[A-Z][A-Z_]*\s*=`))
		env, err := gotenv.StrictParse(lr)
		if err != nil {
			return nil, fmt.Errorf("%s: %s", candidate, err)
		}
		return map[string]string(env), nil
	}
	m := make(map[string]string)
	return m, nil
}

func setColors(colors string) {
	p := palette.DefaultPalette()
	for _, w := range strings.Split(colors, ":") {
		l := strings.SplitN(w, "=", 2)
		if len(l) != 2 {
			continue
		}
		switch l[0] {
		case "primary":
			p.Primary = l[1]
		case "secondary":
			p.Secondary = l[1]
		case "optimal":
			p.Optimal = l[1]
		case "error":
			p.Error = l[1]
		case "warning":
			p.Warning = l[1]
		case "frozen":
			p.Frozen = l[1]
		}
	}
	Colorize = palette.NewFunc(*p)
	Color = palette.New(*p)
}

func setPaths(root string) {
	if root == "" {
		Paths = AgentPaths{
			Root:         root,
			Bin:          filepath.FromSlash("/usr/bin"),
			Var:          filepath.FromSlash(fmt.Sprintf("/var/lib/%s", Program)),
			LastShutdown: filepath.FromSlash(fmt.Sprintf("/var/lib/%s/last_shutdown", Program)),
			Capabilities: filepath.FromSlash(fmt.Sprintf("/var/lib/%s/%s", Program, basenameCapabilities)),
			Lock:         filepath.FromSlash(fmt.Sprintf("/var/lib/%s/lock", Program)),
			Cache:        filepath.FromSlash(fmt.Sprintf("/var/lib/%s/cache", Program)),
			Certs:        filepath.FromSlash(fmt.Sprintf("/var/lib/%s/certs", Program)),
			CACRL:        filepath.FromSlash(fmt.Sprintf("/var/lib/%s/certs/ca_crl", Program)),
			Lsnr:         filepath.FromSlash(fmt.Sprintf("/var/lib/%s/lsnr", Program)),
			Log:          filepath.FromSlash(fmt.Sprintf("/var/log/%s", Program)),
			Etc:          filepath.FromSlash(fmt.Sprintf("/etc/%s", Program)),
			EtcNs:        filepath.FromSlash(fmt.Sprintf("/etc/%s/namespaces", Program)),
			Backup:       filepath.FromSlash(fmt.Sprintf("/var/lib/%s/backup", Program)),
			Tmp:          filepath.FromSlash(fmt.Sprintf("/var/tmp/%s", Program)),
			Doc:          filepath.FromSlash(fmt.Sprintf("/usr/share/doc/%s", Program)),
			HTML:         filepath.FromSlash(fmt.Sprintf("/usr/share/%s/html", Program)),
			Drivers:      filepath.FromSlash(fmt.Sprintf("/usr/libexec/%s", Program)),
			Compliance:   filepath.FromSlash(fmt.Sprintf("/usr/share/%s/compliance", Program)),
		}
	} else {
		Paths = AgentPaths{
			Root:         root,
			Bin:          filepath.Join(root, "bin"),
			Var:          filepath.Join(root, "var"),
			LastShutdown: filepath.Join(root, "var", "last_shutdown"),
			Capabilities: filepath.Join(root, "var", basenameCapabilities),
			Lock:         filepath.Join(root, "var", "lock"),
			Lsnr:         filepath.Join(root, "var", "lsnr"),
			Cache:        filepath.Join(root, "var", "cache"),
			Certs:        filepath.Join(root, "var", "certs"),
			CACRL:        filepath.Join(root, "var", "certs", "ca_crl"),
			Backup:       filepath.Join(root, "var", "backup"),
			Log:          filepath.Join(root, "log"),
			Etc:          filepath.Join(root, "etc"),
			EtcNs:        filepath.Join(root, "etc", "namespaces"),
			Tmp:          filepath.Join(root, "tmp"),
			Doc:          filepath.Join(root, "share", "doc"),
			Compliance:   filepath.Join(root, "share", "compliance"),
			HTML:         filepath.Join(root, "share", "html"),
			Drivers:      filepath.Join(root, "drivers"),
		}
	}
}

// ReloadForTest can be used during tests to force a reload of config after root path populated
// cleanup function is returned to reset rawconfig from default.
//
// Usage example:
//
//	func TestSomething(t *testing.T) {
//	  env := testhelper.Setup(t)
//	  env.InstallFile("../../testdata/cluster.conf", "etc/cluster.conf")
//	  defer rawconfig.ReloadForTest(env.Root)()
//	  ...
func ReloadForTest(rootPath string) func() {
	Load(map[string]string{"OSVC_ROOT_PATH": rootPath})
	return func() {
		Load(map[string]string{})
	}
}
