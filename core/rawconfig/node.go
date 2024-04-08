package rawconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"

	"github.com/opensvc/om3/core/clusternode"
	"github.com/opensvc/om3/core/omcrypto"
	"github.com/opensvc/om3/util/capabilities"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/render/palette"
)

const (
	// Program is the name of the project and module
	Program = "opensvc"
)

var (
	Colorize *palette.ColorPaletteFunc
	Color    *palette.ColorPalette
	Paths    AgentPaths
	Envs     []string

	// nodeViper is the global accessor to the viper instance handling configuration
	nodeViper *viper.Viper
	fromViper = conf{}

	clusterSectionCmd = make(chan chan<- ClusterSection)
	nodeSectionCmd    = make(chan chan<- NodeSection)
	loadConfigCmd     = make(chan loadCmd)

	sectionCluster ClusterSection
	sectionNode    NodeSection

	defaultEnvs = []string{
		"CERT",
		"DEV",
		"DRP",
		"FOR",
		"INT",
		"PRA",
		"PRD",
		"PRJ",
		"PPRD",
		"QUAL",
		"REC",
		"STG",
		"TMP",
		"TST",
		"UAT",
	}
)

type (
	// conf is the merged config (defaults, cluster.conf then node.conf)
	conf struct {
		Cluster  ClusterSection        `mapstructure:"cluster"`
		Hostname string                `mapstructure:"hostname"`
		Node     NodeSection           `mapstructure:"node"`
		Palette  palette.StringPalette `mapstructure:"palette"`
		Paths    AgentPaths            `mapstructure:"paths"`
		Envs     []string              `mapstructure:"envs"`
	}

	ClusterSection struct {
		ID         string `mapstructure:"id"`
		Name       string `mapstructure:"name"`
		Secret     string `mapstructure:"secret"`
		CASecPaths string `mapstructure:"ca"`
		Nodes      string `mapstructure:"nodes"`
		DNS        string `mapstructure:"dns"`
	}

	NodeSection struct {
		Env       string `mapstructure:"env"`
		Collector string `mapstructure:"dbopensvc"`
		UUID      string `mapstructure:"uuid"`
		PRKey     string `mapstructure:"prkey"`
	}

	loadCmd struct {
		Env  map[string]string
		Done chan struct{}
	}
)

func init() {
	running := make(chan bool)
	go func() {
		running <- true
		for {
			select {
			case respChan := <-clusterSectionCmd:
				respChan <- sectionCluster
			case respChan := <-nodeSectionCmd:
				respChan <- sectionNode
			case cmd := <-loadConfigCmd:
				loadSections()
				cmd.Done <- struct{}{}
			}
		}
	}()
	<-running
	Load(nil)

	// TODO: move this outside, to remove omcrypto deps. keep this here for
	// cmds that needs omcrypto
	omcrypto.SetClusterName(sectionCluster.Name)
	omcrypto.SetClusterSecret(sectionCluster.Secret)
}

func GetClusterSection() ClusterSection {
	c := make(chan ClusterSection)
	clusterSectionCmd <- c
	return <-c
}

func GetNodeSection() NodeSection {
	c := make(chan NodeSection)
	nodeSectionCmd <- c
	return <-c
}

func LoadSections() {
	cmd := loadCmd{
		Done: make(chan struct{}),
	}
	loadConfigCmd <- cmd
	<-cmd.Done
}

// Load initializes the Viper and Config globals.
// Done once in package init(), but can be called again to force env variables or detect changes.
func Load(env map[string]string) {
	nodeViper = viper.New()
	nodeViper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	nodeViper.AutomaticEnv()

	if env == nil {
		env = readEnvFile()
	}
	root := os.Getenv("OSVC_ROOT_PATH")
	if root == "" {
		root, _ = env["osvc_root_path"]
	}
	python, _ := env["osvc_python"]
	getClusterName := func() string {
		if s, ok := env["osvc_cluster_name"]; ok && s != "" {
			return s
		}
		return "default"
	}
	setDefaults(root)
	nodeViper.SetDefault("paths.python", python)
	nodeViper.SetDefault("cluster.name", getClusterName())
	nodeViper.SetDefault("envs", defaultEnvs)

	if err := nodeViper.Unmarshal(&fromViper); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Failed to extract the configuration %s\n", err)
		return
	}
	Paths = fromViper.Paths
	capabilities.SetCacheFile(Paths.Capabilities)

	LoadSections()

	Colorize = palette.NewFunc(fromViper.Palette)
	Color = palette.New(fromViper.Palette)
}

func setDefaults(root string) {
	nodeViper.SetDefault("hostname", hostname.Hostname())
	if root == defPathRoot {
		nodeViper.SetDefault("paths.root", "")
		nodeViper.SetDefault("paths.bin", defPathBin)
		nodeViper.SetDefault("paths.var", defPathVar)
		nodeViper.SetDefault("paths.last_shutdown", defPathLastShutdown)
		nodeViper.SetDefault("paths.capabilities", defPathCapabilities)
		nodeViper.SetDefault("paths.lock", defPathLock)
		nodeViper.SetDefault("paths.lsnr", defPathLsnr)
		nodeViper.SetDefault("paths.cache", defPathCache)
		nodeViper.SetDefault("paths.certs", defPathCerts)
		nodeViper.SetDefault("paths.cacrl", defPathCACRL)
		nodeViper.SetDefault("paths.log", defPathLog)
		nodeViper.SetDefault("paths.etc", defPathEtc)
		nodeViper.SetDefault("paths.etcns", defPathEtcNs)
		nodeViper.SetDefault("paths.tmp", defPathTmp)
		nodeViper.SetDefault("paths.doc", defPathDoc)
		nodeViper.SetDefault("paths.compliance", defPathCompliance)
		nodeViper.SetDefault("paths.html", defPathHTML)
		nodeViper.SetDefault("paths.drivers", defPathDrivers)
	} else {
		nodeViper.SetDefault("paths.root", root)
		nodeViper.SetDefault("paths.bin", filepath.Join(root, "bin"))
		nodeViper.SetDefault("paths.var", filepath.Join(root, "var"))
		nodeViper.SetDefault("paths.last_shutdown", filepath.Join(root, "var", "last_shutdown"))
		nodeViper.SetDefault("paths.capabilities", filepath.Join(root, "var", "capabilities.json"))
		nodeViper.SetDefault("paths.lock", filepath.Join(root, "var", "lock"))
		nodeViper.SetDefault("paths.lsnr", filepath.Join(root, "var", "lsnr"))
		nodeViper.SetDefault("paths.cache", filepath.Join(root, "var", "cache"))
		nodeViper.SetDefault("paths.certs", filepath.Join(root, "var", "certs"))
		nodeViper.SetDefault("paths.cacrl", filepath.Join(root, "var", "certs", "ca_crl"))
		nodeViper.SetDefault("paths.log", filepath.Join(root, "log"))
		nodeViper.SetDefault("paths.etc", filepath.Join(root, "etc"))
		nodeViper.SetDefault("paths.etcns", filepath.Join(root, "etc", "namespaces"))
		nodeViper.SetDefault("paths.tmp", filepath.Join(root, "tmp"))
		nodeViper.SetDefault("paths.doc", filepath.Join(root, "share", "doc"))
		nodeViper.SetDefault("paths.compliance", filepath.Join(root, "share", "compliance"))
		nodeViper.SetDefault("paths.html", filepath.Join(root, "share", "html"))
		nodeViper.SetDefault("paths.drivers", filepath.Join(root, "drivers"))
	}
	nodeViper.SetDefault("palette.primary", palette.DefaultPrimary)
	nodeViper.SetDefault("palette.secondary", palette.DefaultSecondary)
	nodeViper.SetDefault("palette.optimal", palette.DefaultOptimal)
	nodeViper.SetDefault("palette.error", palette.DefaultError)
	nodeViper.SetDefault("palette.warning", palette.DefaultWarning)
	nodeViper.SetDefault("palette.frozen", palette.DefaultFrozen)
}

func loadSections() {
	nodeViper.SetConfigType("ini")

	p := fmt.Sprintf("%s/cluster.conf", Paths.Etc)
	nodeViper.SetConfigFile(filepath.FromSlash(p))
	_ = nodeViper.MergeInConfig()

	p = fmt.Sprintf("%s/node.conf", Paths.Etc)
	nodeViper.SetConfigFile(filepath.FromSlash(p))
	_ = nodeViper.MergeInConfig()

	p = fmt.Sprintf("$HOME/.%s", Program)
	nodeViper.SetConfigType("yaml")
	nodeViper.AddConfigPath(filepath.FromSlash(p))
	nodeViper.AddConfigPath(".")
	nodeViper.MergeInConfig()

	if err := nodeViper.Unmarshal(&fromViper); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Failed to extract the configuration %s\n", err)
		return
	}
	sectionCluster = fromViper.Cluster
	clusterNodes := []string{}
	for _, s := range strings.Fields(sectionCluster.Nodes) {
		clusterNodes = append(clusterNodes, strings.ToLower(s))
	}
	clusternode.Set(clusterNodes)
	sectionNode = fromViper.Node
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
	Load(map[string]string{"osvc_root_path": rootPath})
	return func() {
		Load(map[string]string{})
	}
}
