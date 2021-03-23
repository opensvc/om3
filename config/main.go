package config

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

type (
	// T exposes methods to read and write configurations.
	T struct {
		Path string
		v    *viper.Viper
		raw  Raw
	}

	Raw map[string]interface{}
	Key string
)

func (t Key) section() string {
	l := strings.Split(string(t), ".")
	switch len(l) {
	case 2:
		return l[0]
	default:
		return "DEFAULT"
	}
}

func (t Key) option() string {
	l := strings.Split(string(t), ".")
	switch len(l) {
	case 2:
		return l[1]
	default:
		return l[0]
	}
}

//
// Get returns a key value,
// * contextualized for a node (by default the local node, customized by the
//   impersonate option)
// * dereferenced
// * evaluated
//
func (t *T) Get(key string) interface{} {
	val := t.v.GetString(key)
	log.Debug().Msgf("config %s get %s => %s", t.Path, key, val)
	return val
}

func (t *T) Raw() Raw {
	return t.raw
}

var (
	RegexpReference = regexp.MustCompile(`({.+})`)
)

func (t Raw) Render() string {
	s := ""
	for section, data := range t {
		s += Node.Colorize.Primary(fmt.Sprintf("[%s]\n", section))
		for k, v := range data.(map[string]interface{}) {
			coloredValue := RegexpReference.ReplaceAllString(v.(string), Node.Colorize.Optimal("$1"))
			s += fmt.Sprintf("%s = %s\n", Node.Colorize.Secondary(k), coloredValue)
		}
		s += "\n"
	}
	return s
}
