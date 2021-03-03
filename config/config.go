package config

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

const (
	// Program is the name of the project and module
	Program = "opensvc"
)

var (
	// Config is the global containing the program configuration
	Config Type

	// Viper is the global accessor to the viper instance handling configuration
	Viper *viper.Viper
)

type (
	// Type is the top level configuration structure
	Type struct {
		Paths AgentPaths `mapstructure:"paths"`
	}
)

// Load initializes the Viper and Config globals
func Load() {
	Viper = viper.New()
	Viper.SetConfigName("opensvc")
	Viper.SetConfigType("yaml")
	//v.SetEnvPrefix("")
	Viper.AddConfigPath(filepath.Join("etc", Program))
	Viper.AddConfigPath(filepath.Join("$HOME", "."+Program))
	Viper.AddConfigPath(".")
	Viper.AutomaticEnv()
	Viper.SetDefault("paths.root", "")
	Viper.SetDefault("paths.bin", defPathBin)
	Viper.SetDefault("paths.var", defPathVar)
	Viper.SetDefault("paths.log", defPathLog)
	Viper.SetDefault("paths.etc", defPathEtc)
	Viper.SetDefault("paths.etcns", defPathEtcNs)
	Viper.SetDefault("paths.tmp", defPathTmp)
	Viper.SetDefault("paths.doc", defPathDoc)
	Viper.SetDefault("paths.html", defPathHTML)
	Viper.SetDefault("paths.drivers", defPathDrivers)
	Viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	Viper.ReadInConfig()

	root := Viper.GetString("paths.root")
	if root != defPathRoot {
		Viper.SetDefault("paths.bin", filepath.Join(root, "bin"))
		Viper.SetDefault("paths.var", filepath.Join(root, "var"))
		Viper.SetDefault("paths.log", filepath.Join(root, "log"))
		Viper.SetDefault("paths.etc", filepath.Join(root, "etc"))
		Viper.SetDefault("paths.etcns", filepath.Join(root, "etc", "namespaces"))
		Viper.SetDefault("paths.tmp", filepath.Join(root, "tmp"))
		Viper.SetDefault("paths.doc", filepath.Join(root, "share", "doc"))
		Viper.SetDefault("paths.html", filepath.Join(root, "share", "html"))
		Viper.SetDefault("paths.drivers", filepath.Join(root, "drivers"))
	}

	if err := Viper.Unmarshal(&Config); err != nil {
		fmt.Printf("Failed to parse the configuration file: %s\n", err)
		return
	}
}
