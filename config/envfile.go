package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/msoap/byline"
	"github.com/spf13/viper"
)

func readEnvFile() map[string]string {
	envVip := viper.New()
	envCfg := make(map[string]string)
	envVip.SetConfigType("env")
	candidates := []string{
		filepath.FromSlash("/etc/sysconfig"),
		filepath.FromSlash("/etc/default"),
		filepath.FromSlash("/etc/defaults"),
	}
	for _, p := range candidates {
		reader, err := os.Open(filepath.Join(p, Program))
		if err != nil {
			continue
		}
		defer reader.Close()
		lr := byline.NewReader(reader)
		lr.GrepByRegexp(regexp.MustCompile(`^\s*[A-Z][A-Z_]*\s*=`))
		if err := envVip.ReadConfig(lr); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
		envVip.Unmarshal(&envCfg)
		return envCfg
	}
	return envCfg
}
