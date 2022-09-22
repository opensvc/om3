package rawconfig

import (
	"fmt"
	"path/filepath"
)

var (
	defPathRoot    = ""
	defPathBin     = filepath.FromSlash("/usr/bin")
	defPathVar     = filepath.FromSlash(fmt.Sprintf("/var/lib/%s", Program))
	defPathLock    = filepath.FromSlash(fmt.Sprintf("/var/lib/%s/lock", Program))
	defPathCache   = filepath.FromSlash(fmt.Sprintf("/var/lib/%s/cache", Program))
	defPathCerts   = filepath.FromSlash(fmt.Sprintf("/var/lib/%s/certs", Program))
	defPathCACRL   = filepath.FromSlash(fmt.Sprintf("/var/lib/%s/certs/ca_crl", Program))
	defPathLog     = filepath.FromSlash(fmt.Sprintf("/var/log/%s", Program))
	defPathEtc     = filepath.FromSlash(fmt.Sprintf("/etc/%s", Program))
	defPathEtcNs   = filepath.FromSlash(fmt.Sprintf("/etc/%s/namespaces", Program))
	defPathTmp     = filepath.FromSlash(fmt.Sprintf("/var/tmp/%s", Program))
	defPathDoc     = filepath.FromSlash(fmt.Sprintf("/usr/share/doc/%s", Program))
	defPathHTML    = filepath.FromSlash(fmt.Sprintf("/usr/share/%s/html", Program))
	defPathDrivers = filepath.FromSlash(fmt.Sprintf("/usr/libexec/%s", Program))
)

type (
	// AgentPaths abstracts all paths of the agent file organisation
	AgentPaths struct {
		Python  string `mapstructure:"python"`
		Root    string `mapstructure:"root"`
		Bin     string `mapstructure:"bin"`
		Var     string `mapstructure:"var"`
		Lock    string `mapstructure:"lock"`
		Cache   string `mapstructure:"cache"`
		Certs   string `mapstructure:"certs"`
		CACRL   string
		Log     string `mapstructure:"log"`
		Etc     string `mapstructure:"etc"`
		EtcNs   string
		Tmp     string `mapstructure:"tmp"`
		Doc     string `mapstructure:"doc"`
		HTML    string `mapstructure:"html"`
		Drivers string `mapstructure:"drivers"`
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
