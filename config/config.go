package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

const (
	// Program is the name of the project and module
	Program = "opensvc"
)

var (
	// Node is the global containing the program configuration
	Node node

	// NodeViper is the global accessor to the viper instance handling configuration
	NodeViper *viper.Viper
)

type (
	// Type is the top level configuration structure
	node struct {
		Hostname string         `mapstructure:"hostname"`
		Paths    AgentPaths     `mapstructure:"paths"`
		Cluster  clusterSection `mapstructure:"cluster"`
	}

	clusterSection struct {
		Name   string `mapstructure:"name"`
		Secret string `mapstructure:"secret"`
	}
)

// Load initializes the Viper and Config globals
func Load() {
	NodeViper = viper.New()

	if s, err := os.Hostname(); err != nil {
		NodeViper.SetDefault("hostname", s)
	}
	NodeViper.SetDefault("paths.root", "")
	NodeViper.SetDefault("paths.bin", defPathBin)
	NodeViper.SetDefault("paths.var", defPathVar)
	NodeViper.SetDefault("paths.log", defPathLog)
	NodeViper.SetDefault("paths.etc", defPathEtc)
	NodeViper.SetDefault("paths.etcns", defPathEtcNs)
	NodeViper.SetDefault("paths.tmp", defPathTmp)
	NodeViper.SetDefault("paths.doc", defPathDoc)
	NodeViper.SetDefault("paths.html", defPathHTML)
	NodeViper.SetDefault("paths.drivers", defPathDrivers)
	NodeViper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	NodeViper.AutomaticEnv()

	envCfg := readEnvFile()
	root, ok := envCfg["osvc_root_path"].(string)
	if ok && root != defPathRoot {
		NodeViper.SetDefault("paths.bin", filepath.Join(root, "bin"))
		NodeViper.SetDefault("paths.var", filepath.Join(root, "var"))
		NodeViper.SetDefault("paths.log", filepath.Join(root, "log"))
		NodeViper.SetDefault("paths.etc", filepath.Join(root, "etc"))
		NodeViper.SetDefault("paths.etcns", filepath.Join(root, "etc", "namespaces"))
		NodeViper.SetDefault("paths.tmp", filepath.Join(root, "tmp"))
		NodeViper.SetDefault("paths.doc", filepath.Join(root, "share", "doc"))
		NodeViper.SetDefault("paths.html", filepath.Join(root, "share", "html"))
		NodeViper.SetDefault("paths.drivers", filepath.Join(root, "drivers"))
	}

	NodeViper.SetConfigType("ini")

	p := fmt.Sprintf("%s/cluster.conf", NodeViper.GetString("paths.etc"))
	NodeViper.SetConfigFile(filepath.FromSlash(p))
	NodeViper.MergeInConfig()

	p = fmt.Sprintf("%s/node.conf", NodeViper.GetString("paths.etc"))
	NodeViper.SetConfigFile(filepath.FromSlash(p))
	NodeViper.MergeInConfig()

	p = fmt.Sprintf("$HOME/.%s", Program)
	NodeViper.SetConfigType("yaml")
	NodeViper.AddConfigPath(filepath.FromSlash(p))
	NodeViper.AddConfigPath(".")
	NodeViper.MergeInConfig()

	if err := NodeViper.Unmarshal(&Node); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to parse the configuration file: %s\n", err)
		return
	}
}
