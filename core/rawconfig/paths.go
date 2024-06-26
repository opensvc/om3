package rawconfig

import (
	"fmt"
	"os"
	"path/filepath"
)

var (
	defPathRoot         = ""
	defPathBin          = filepath.FromSlash("/usr/bin")
	defPathVar          = filepath.FromSlash(fmt.Sprintf("/var/lib/%s", Program))
	defPathLastShutdown = filepath.FromSlash(fmt.Sprintf("/var/lib/%s/last_shutdown", Program))
	defPathCapabilities = filepath.FromSlash(fmt.Sprintf("/var/lib/%s/%s", Program, basenameCapabilities))
	defPathLock         = filepath.FromSlash(fmt.Sprintf("/var/lib/%s/lock", Program))
	defPathCache        = filepath.FromSlash(fmt.Sprintf("/var/lib/%s/cache", Program))
	defPathCerts        = filepath.FromSlash(fmt.Sprintf("/var/lib/%s/certs", Program))
	defPathCACRL        = filepath.FromSlash(fmt.Sprintf("/var/lib/%s/certs/ca_crl", Program))
	defPathLsnr         = filepath.FromSlash(fmt.Sprintf("/var/lib/%s/lsnr", Program))
	defPathLog          = filepath.FromSlash(fmt.Sprintf("/var/log/%s", Program))
	defPathEtc          = filepath.FromSlash(fmt.Sprintf("/etc/%s", Program))
	defPathEtcNs        = filepath.FromSlash(fmt.Sprintf("/etc/%s/namespaces", Program))
	defPathTmp          = filepath.FromSlash(fmt.Sprintf("/var/tmp/%s", Program))
	defPathDoc          = filepath.FromSlash(fmt.Sprintf("/usr/share/doc/%s", Program))
	defPathHTML         = filepath.FromSlash(fmt.Sprintf("/usr/share/%s/html", Program))
	defPathDrivers      = filepath.FromSlash(fmt.Sprintf("/usr/libexec/%s", Program))
	defPathCompliance   = filepath.FromSlash(fmt.Sprintf("/usr/share/%s/compliance", Program))
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

func NodeConfigFile() string {
	return filepath.Join(Paths.Etc, "node.conf")
}

func ClusterConfigFile() string {
	return filepath.Join(Paths.Etc, "cluster.conf")
}

func CreateMandatoryDirectories() error {
	mandatoryDirs := []string{
		NodeVarDir(),
		DNSUDSDir(),
		Paths.Certs,
		Paths.Etc,
		Paths.Lsnr,
		filepath.Join(Paths.Etc, "namespaces"),
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
