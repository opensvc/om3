package rawconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/viper"

	"opensvc.com/opensvc/util/hostname"
	"opensvc.com/opensvc/util/render/palette"
)

const (
	// Program is the name of the project and module
	Program = "opensvc"
)

var (
	Colorize *palette.ColorPaletteFunc
	Color    *palette.ColorPalette
	Paths    AgentPaths

	// nodeViper is the global accessor to the viper instance handling configuration
	nodeViper *viper.Viper
	fromViper = conf{}

	clusterSectionCmd = make(chan chan<- clusterSection)
	nodeSectionCmd    = make(chan chan<- nodeSection)
	loadConfigCmd     = make(chan loadCmd)

	sectionCluster clusterSection
	sectionNode    nodeSection
)

type (
	// conf is the merged config (defaults, cluster.conf then node.conf)
	conf struct {
		Cluster  clusterSection        `mapstructure:"cluster"`
		Hostname string                `mapstructure:"hostname"`
		Node     nodeSection           `mapstructure:"node"`
		Palette  palette.StringPalette `mapstructure:"palette"`
		Paths    AgentPaths            `mapstructure:"paths"`
	}

	clusterSection struct {
		ID         string `mapstructure:"id"`
		Name       string `mapstructure:"name"`
		Secret     string `mapstructure:"secret"`
		CASecPaths string `mapstructure:"ca"`
		Nodes      string `mapstructure:"nodes"`
	}

	nodeSection struct {
		Env               string         `mapstructure:"env"`
		Collector         string         `mapstructure:"dbopensvc"`
		UUID              string         `mapstructure:"uuid"`
		PRKey             string         `mapstructure:"prkey"`
		RejoinGracePeriod *time.Duration `mapstructure:"rejoin_grace_period"`
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
}

func ClusterSection() clusterSection {
	c := make(chan clusterSection)
	clusterSectionCmd <- c
	return <-c
}

func NodeSection() nodeSection {
	c := make(chan nodeSection)
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
	clusterName, _ := env["osvc_cluster_name"]
	setDefaults(root)
	nodeViper.SetDefault("paths.python", python)
	nodeViper.SetDefault("cluster.name", clusterName)

	if err := nodeViper.Unmarshal(&fromViper); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Failed to extract the configuration %s\n", err)
		return
	}
	Paths = fromViper.Paths

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
		nodeViper.SetDefault("paths.lock", defPathLock)
		nodeViper.SetDefault("paths.cache", defPathCache)
		nodeViper.SetDefault("paths.certs", defPathCerts)
		nodeViper.SetDefault("paths.cacrl", defPathCACRL)
		nodeViper.SetDefault("paths.log", defPathLog)
		nodeViper.SetDefault("paths.etc", defPathEtc)
		nodeViper.SetDefault("paths.etcns", defPathEtcNs)
		nodeViper.SetDefault("paths.tmp", defPathTmp)
		nodeViper.SetDefault("paths.doc", defPathDoc)
		nodeViper.SetDefault("paths.html", defPathHTML)
		nodeViper.SetDefault("paths.drivers", defPathDrivers)
	} else {
		nodeViper.SetDefault("paths.root", root)
		nodeViper.SetDefault("paths.bin", filepath.Join(root, "bin"))
		nodeViper.SetDefault("paths.var", filepath.Join(root, "var"))
		nodeViper.SetDefault("paths.lock", filepath.Join(root, "var", "lock"))
		nodeViper.SetDefault("paths.cache", filepath.Join(root, "var", "cache"))
		nodeViper.SetDefault("paths.certs", filepath.Join(root, "var", "certs"))
		nodeViper.SetDefault("paths.cacrl", filepath.Join(root, "var", "certs", "ca_crl"))
		nodeViper.SetDefault("paths.log", filepath.Join(root, "log"))
		nodeViper.SetDefault("paths.etc", filepath.Join(root, "etc"))
		nodeViper.SetDefault("paths.etcns", filepath.Join(root, "etc", "namespaces"))
		nodeViper.SetDefault("paths.tmp", filepath.Join(root, "tmp"))
		nodeViper.SetDefault("paths.doc", filepath.Join(root, "share", "doc"))
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
	sectionNode = fromViper.Node
}
