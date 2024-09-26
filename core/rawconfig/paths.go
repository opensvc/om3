package rawconfig

import (
	"fmt"
	"os"
	"path/filepath"
)

type (
	// AgentPaths abstracts all paths of the agent file organisation
	AgentPaths struct {
		Python       string `mapstructure:"python"`
		Root         string `mapstructure:"root"`
		Bin          string `mapstructure:"bin"`
		Var          string `mapstructure:"var"`
		Lock         string `mapstructure:"lock"`
		Lsnr         string `mapstructure:"lsnr"`
		Cache        string `mapstructure:"cache"`
		Certs        string `mapstructure:"certs"`
		CACRL        string `mapstructure:"cacrl"`
		Log          string `mapstructure:"log"`
		Etc          string `mapstructure:"etc"`
		EtcNs        string `mapstructure:"etcns"`
		LastShutdown string `mapstructure:"last_shutdown"`
		Capabilities string `mapstructure:"capabilities"`
		Tmp          string `mapstructure:"tmp"`
		Doc          string `mapstructure:"doc"`
		HTML         string `mapstructure:"html"`
		Drivers      string `mapstructure:"drivers"`
		Compliance   string `mapstructure:"compliance"`
	}
)

func DNSUDSDir() string {
	return filepath.Join(Paths.Var, "dns")
}

func DNSUDSFile() string {
	return filepath.Join(Paths.Var, "dns", "pdns.sock")
}

func NodeVarDir() string {
	return filepath.Join(Paths.Var, "node")
}

func CollectorSentDir() string {
	return filepath.Join(Paths.Var, "node", "collector", "config_sent")
}

func NodeConfigFile() string {
	return filepath.Join(Paths.Etc, "node.conf")
}

func ClusterConfigFile() string {
	return filepath.Join(Paths.Etc, "cluster.conf")
}

func CreateMandatoryDirectories() error {
	mandatoryDirs := []string{
		NodeVarDir(),
		CollectorSentDir(),
		DNSUDSDir(),
		Paths.Certs,
		Paths.Etc,
		filepath.Join(Paths.Etc, "namespaces"),
		Paths.Lsnr,
		Paths.Tmp,
	}
	for _, d := range mandatoryDirs {
		info, err := os.Stat(d)
		if os.IsNotExist(err) {
			if err := os.MkdirAll(d, 0700); err != nil {
				return fmt.Errorf("can't create mandatory dir '%s'", d)
			}
		} else if err != nil {
			return fmt.Errorf("mandatory dir '%s' stat unexpected error: %s", d, err)
		} else if !info.IsDir() {
			return fmt.Errorf("mandatory dir '%s' is not a directory", d)
		}
	}
	return nil
}
