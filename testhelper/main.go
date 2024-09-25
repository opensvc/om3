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

	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/util/file"
	"github.com/opensvc/om3/util/hostname"
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
	return SetupEnv(Env{
		Root:        t.TempDir(),
		ClusterName: "cluster1",
		TestingT:    t,
	})
}

func SetupEnv(env Env) Env {
	rawconfig.Load(map[string]string{
		"OSVC_ROOT_PATH":    env.Root,
		"OSVC_CLUSTER_NAME": env.ClusterName,
	})
	setupLog()

	// Create mandatory dirs
	if err := rawconfig.CreateMandatoryDirectories(); err != nil {
		panic(err)
	}

	return env
}

func setupLog() {
	zerolog.TimeFieldFormat = time.StampMicro
	out := zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: time.StampMicro,
	}
	switch os.Getenv("TEST_LOG_LEVEL") {
	case "info":
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	case "warn":
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	case "error":
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	case "fatal":
		zerolog.SetGlobalLevel(zerolog.FatalLevel)
	case "panic":
		zerolog.SetGlobalLevel(zerolog.PanicLevel)
	case "nolevel":
		zerolog.SetGlobalLevel(zerolog.NoLevel)
	default:
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}
	log.Logger = log.Logger.Output(out).With().Caller().Logger()
}

func SetupEnvWithoutCreateMandatoryDirectories(env Env) Env {
	rawconfig.Load(map[string]string{
		"OSVC_ROOT_PATH":    env.Root,
		"OSVC_CLUSTER_NAME": env.ClusterName,
	})
	setupLog()

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

func TCPPortAvailable(port string) error {
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
	if _, ok := os.LookupEnv("TEST_HELPER_TRACE"); ok {
		RunCmd(t, "ps", "fax")
		RunCmd(t, "netstat", "-petulan")
		pid := os.Getpid()
		RunCmd(t, "ls", "-l", fmt.Sprintf("/proc/%d/fd", pid))
	}
}
