package config

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/viper"
)

// LoadObject configures and returns a Viper instance
func LoadObject(p string) (*viper.Viper, error) {
	v := viper.New()
	v.SetConfigType("ini")
	v.SetConfigFile(filepath.FromSlash(p))
	v.ReadInConfig()

	data := make(map[string]interface{})

	if err := v.Unmarshal(&data); err != nil {
		return nil, err
	}
	defaults, ok := data["DEFAULT"]
	if !ok {
		defaults = map[string]string{
			"nodes": Node.Hostname,
		}
	}
	fmt.Println(defaults)
	return v, nil
}
