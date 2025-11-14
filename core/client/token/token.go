package reqtoken

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
	Token struct {
		AccessTokenExpire  time.Time `json:"access_expired_at"`
		AccessToken        string    `json:"access_token"`
		RefreshTokenExpire time.Time `json:"refresh_expired_at"`
		RefreshToken       string    `json:"refresh_token"`
	}
)

func SaveToken(contextName string, token Token) error {
	filename, _ := homedir.Expand(getFilePath(contextName))
	b, err := json.MarshalIndent(token, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filename, b, 0o600); err != nil {
		return err
	}
	return nil
}

func LoadToken(contextName string, token *Token) error {
	filename, _ := homedir.Expand(getFilePath(contextName))
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

func TokenExists(contextName string) bool {
	filename, _ := homedir.Expand(getFilePath(contextName))
	_, err := os.Stat(filename)
	return !errors.Is(err, os.ErrNotExist)
}

func DeleteToken(contextName string) error {
	if !TokenExists(contextName) {
		return fmt.Errorf("no token found for context %s", contextName)
	}
	filename, _ := homedir.Expand(getFilePath(contextName))
	err := os.Remove(filename)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func GetAllToken() map[string]Token {
	tokens := make(map[string]Token)
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
			var token Token
			if err := LoadToken(contextName, &token); err == nil {
				tokens[contextName] = token
			}
		}
	}
	return tokens
}

func GetMostRecent() (string, Token) {
	var recentContext string
	var recentToken Token
	tokens := GetAllToken()
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

func getFilePath(contextName string) string {
	return clientcontext.ConfigFolder + "token-" + contextName + ".json"
}
