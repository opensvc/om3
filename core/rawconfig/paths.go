package rawconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/opensvc/om3/v3/util/confined"
)

type (
	// AgentPaths abstracts all paths of the agent file organisation
	AgentPaths struct {
		Python       string `mapstructure:"python"`
		Root         string `mapstructure:"root"`
		Bin          string `mapstructure:"bin"`
		Var          string `mapstructure:"var"`
		VarNs        string `mapstructure:"varns"`
		Lock         string `mapstructure:"lock"`
		Lsnr         string `mapstructure:"lsnr"`
		Cache        string `mapstructure:"cache"`
		Certs        string `mapstructure:"certs"`
		CACRL        string `mapstructure:"cacrl"`
		Log          string `mapstructure:"log"`
		LogNs        string `mapstructure:"logns"`
		Etc          string `mapstructure:"etc"`
		EtcNs        string `mapstructure:"etcns"`
		LastShutdown string `mapstructure:"last_shutdown"`
		Capabilities string `mapstructure:"capabilities"`
		Run          string `mapstructure:"run"`
		Tmp          string `mapstructure:"tmp"`
		TmpNs        string `mapstructure:"tmpns"`
		Doc          string `mapstructure:"doc"`
		HTML         string `mapstructure:"html"`
		Drivers      string `mapstructure:"drivers"`
		Compliance   string `mapstructure:"compliance"`
		Backup       string `mapstructure:"varbck"`
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

func CollectorResInfoSentDir() string {
	return filepath.Join(Paths.Var, "node", "collector", "resinfo_sent")
}

// CollectorActionPendingDir is the directory holding the begin and end of
// the local instance actions not yet acknowledged by the collector.
func CollectorActionPendingDir() string {
	return filepath.Join(Paths.Var, "node", "collector", "action_pending")
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
		CollectorResInfoSentDir(),
		CollectorActionPendingDir(),
		DNSUDSDir(),
		Paths.Certs,
		Paths.Etc,
		Paths.EtcNs,
		Paths.Lsnr,
		Paths.Backup,
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
	return MkdirTmp(Paths.Tmp)
}

// MkdirTmp makes the directory p, the temporary directory of the agent or
// a directory under it, owned by the user of the process and writable by it
// alone, every directory from the temporary directory of the agent down.
//
// The temporary directory of the agent is under /var/tmp, where every user
// can write: a user making it first, as after a cleaner removed it, could
// have the agent write its files through links of theirs. A directory of
// another user is refused, never taken over: an admin removes it. A
// directory of the user writable by others is made 0700.
func MkdirTmp(p string) error {
	anchor := filepath.Dir(Paths.Tmp)
	rel, err := filepath.Rel(anchor, p)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("%s is not under %s", p, Paths.Tmp)
	}
	return confined.MkdirTrusted(anchor, rel, 0o700, nil)
}
