package tokencache

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mitchellh/go-homedir"

	"github.com/opensvc/om3/core/clientcontext"
)

type (
	Entry struct {
		AccessTokenExpire  time.Time `json:"access_expired_at"`
		AccessToken        string    `json:"access_token"`
		RefreshTokenExpire time.Time `json:"refresh_expired_at"`
		RefreshToken       string    `json:"refresh_token"`
	}
)

func Save(contextName string, token Entry) error {
	filename, _ := homedir.Expand(fmtFilename(contextName))
	b, err := json.MarshalIndent(token, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filename, b, 0o600); err != nil {
		return err
	}
	return nil
}

func Load(contextName string, token *Entry) error {
	filename, _ := homedir.Expand(fmtFilename(contextName))
	b, err := os.ReadFile(filename)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	if err := json.Unmarshal(b, token); err != nil {
		return err
	}
	return nil
}

func Exists(contextName string) bool {
	filename, _ := homedir.Expand(fmtFilename(contextName))
	_, err := os.Stat(filename)
	return !errors.Is(err, os.ErrNotExist)
}

func Delete(contextName string) error {
	if !Exists(contextName) {
		return fmt.Errorf("no token found for context %s", contextName)
	}
	filename, _ := homedir.Expand(fmtFilename(contextName))
	err := os.Remove(filename)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func GetAll() map[string]Entry {
	tokens := make(map[string]Entry)
	dirpath, err := homedir.Expand(clientcontext.ConfigFolder)
	if err != nil {
		return tokens
	}
	files, err := os.ReadDir(dirpath)
	if err != nil {
		return tokens
	}
	for _, file := range files {
		name := file.Name()
		if !file.IsDir() && strings.HasPrefix(name, "token-") && strings.HasSuffix(name, ".json") {
			contextName := strings.TrimSuffix(strings.TrimPrefix(name, "token-"), ".json")
			var token Entry
			if err := Load(contextName, &token); err == nil {
				tokens[contextName] = token
			}
		}
	}
	return tokens
}

func GetLast() (string, Entry) {
	var recentContext string
	var recentToken Entry
	tokens := GetAll()
	var recentTime time.Time
	for contextName, token := range tokens {
		if token.AccessTokenExpire.After(recentTime) {
			recentTime = token.AccessTokenExpire
			recentContext = contextName
			recentToken = token
		}
	}
	return recentContext, recentToken
}

func fmtFilename(contextName string) string {
	return clientcontext.ConfigFolder + "token-" + contextName + ".json"
}
