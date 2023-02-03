package testhelper

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/require"

	"opensvc.com/opensvc/core/rawconfig"
	"opensvc.com/opensvc/util/file"
	"opensvc.com/opensvc/util/hostname"
)

type (
	Env struct {
		TestingT    *testing.T
		Root        string
		ClusterName string
	}
)

func InstallFile(t *testing.T, srcFile, dstFile string) {
	require.NoError(t, os.MkdirAll(filepath.Dir(dstFile), os.ModePerm))
	t.Logf("install %s to %s", srcFile, dstFile)
	require.NoError(t, file.Copy(srcFile, dstFile))
}

func (env Env) InstallFile(srcFile, dstFile string) {
	InstallFile(env.TestingT, srcFile, filepath.Join(env.Root, dstFile))
}

func Setup(t *testing.T) Env {
	var env Env
	if env.Root == "" {
		env.Root = t.TempDir()
	}
	if env.ClusterName == "" {
		env.ClusterName = "cluster1"
	}
	env.TestingT = t
	return SetupEnv(env)
}

func SetupEnv(env Env) Env {
	rawconfig.Load(map[string]string{
		"osvc_root_path":    env.Root,
		"osvc_cluster_name": env.ClusterName,
	})
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	log.Logger = log.Logger.Output(zerolog.NewConsoleWriter()).With().Caller().Logger()

	// Create mandatory dirs
	if err := rawconfig.CreateMandatoryDirectories(); err != nil {
		panic(err)
	}
	if err := os.MkdirAll(filepath.Join(rawconfig.Paths.Etc, "namespaces"), os.ModePerm); err != nil {
		panic(err)
	}

	return env
}

func SetupEnvWithoutCreateMandatoryDirectories(env Env) Env {
	rawconfig.Load(map[string]string{
		"osvc_root_path":    env.Root,
		"osvc_cluster_name": env.ClusterName,
	})
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	log.Logger = log.Logger.Output(zerolog.NewConsoleWriter()).With().Caller().Logger()

	return env
}

func Main(m *testing.M, execute func([]string)) {
	defer hostname.Impersonate("node1")()
	switch os.Getenv("GO_TEST_MODE") {
	case "":
		// test mode
		_ = os.Setenv("GO_TEST_MODE", "off")
		os.Exit(m.Run())

	case "off":
		// test bypass mode
		_ = os.Setenv("LANG", "C.UTF-8")
		execute(os.Args[1:])
	}
}

func TcpPortAvailable(port string) error {
	ln, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return err
	}
	return ln.Close()
}

func RunCmd(t *testing.T, name string, args ...string) {
	cmd := exec.Command(name, args...)
	b, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("%s error %s\n%s", cmd, err, b)
	} else {
		t.Logf("%s\n%s", cmd, b)
	}
}

func Trace(t *testing.T) {
	RunCmd(t, "ps", "fax")
	RunCmd(t, "netstat", "-petulan")
	pid := os.Getpid()
	RunCmd(t, "ls", "-l", fmt.Sprintf("/proc/%d/fd", pid))
}

func DaemonPorts(t *testing.T, name string) error {
	t.Logf("Verify daemon ports [%s]", name)
	Trace(t)
	var delay time.Duration
	for _, port := range []string{"1214", "1215"} {
		if err := TcpPortAvailable(port); err != nil {
			t.Logf("Verify daemon ports [%s] failed for port %s '%s' wait delay then check again", name, port, err)
			Trace(t)
			delay = 250 * time.Millisecond
		}
	}
	time.Sleep(delay)
	for _, port := range []string{"1214", "1215"} {
		if err := TcpPortAvailable(port); err != nil {
			t.Logf("Verify daemon ports [%s] failed for port %s '%s'", name, port, err)
			Trace(t)
			return err
		}
	}
	t.Logf("Verify daemon ports [%s] [done]", name)
	return nil
}
