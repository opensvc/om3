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
		AccessTokenExpire   time.Time     `json:"access_expired_at"`
		AccessToken         string        `json:"access_token"`
		AccessTokenDuration time.Duration `json:"access_token_duration,omitempty"`
		RefreshTokenExpire  time.Time     `json:"refresh_expired_at"`
		RefreshToken        string        `json:"refresh_token"`
	}
)

func Save(contextName string, token Entry) error {
	filename, _ := homedir.Expand(FmtFilename(contextName))
	b, err := json.MarshalIndent(token, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filename, b, 0o600); err != nil {
		return err
	}
	return nil
}

func Load(contextName string) (*Entry, error) {
	filename, _ := homedir.Expand(FmtFilename(contextName))
	b, err := os.ReadFile(filename)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	var token = &Entry{}
	if err := json.Unmarshal(b, token); err != nil {
		return nil, err
	}
	return token, nil
}

func Exists(contextName string) bool {
	filename, _ := homedir.Expand(FmtFilename(contextName))
	_, err := os.Stat(filename)
	return !errors.Is(err, os.ErrNotExist)
}

func Delete(contextName string) error {
	if !Exists(contextName) {
		return fmt.Errorf("no token found for context %s", contextName)
	}
	filename, _ := homedir.Expand(FmtFilename(contextName))
	err := os.Remove(filename)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func getAllFiles() ([]os.DirEntry, error) {
	dirpath, err := homedir.Expand(clientcontext.ConfigFolder)
	if err != nil {
		return nil, err
	}
	files, err := os.ReadDir(dirpath)
	if err != nil {
		return nil, err
	}
	return files, nil
}

func GetAll() map[string]Entry {
	tokens := make(map[string]Entry)
	files, err := getAllFiles()
	if err != nil {
		return tokens
	}
	for _, file := range files {
		name := file.Name()
		if !file.IsDir() && strings.HasPrefix(name, "token-") && strings.HasSuffix(name, ".json") {
			contextName := strings.TrimSuffix(strings.TrimPrefix(name, "token-"), ".json")
			token, err := Load(contextName)
			if err == nil && token != nil {
				tokens[contextName] = *token
			}
		}
	}
	return tokens
}

func GetLast() (string, Entry) {
	files, err := getAllFiles()
	if err != nil {
		return "", Entry{}
	}
	var lastContext string
	var lastModTime time.Time
	for _, file := range files {
		name := file.Name()
		if !file.IsDir() && strings.HasPrefix(name, "token-") && strings.HasSuffix(name, ".json") {
			info, err := file.Info()
			if err != nil {
				continue
			}
			if info.ModTime().After(lastModTime) {
				lastModTime = info.ModTime()
				lastContext = strings.TrimSuffix(strings.TrimPrefix(name, "token-"), ".json")
			}
		}
	}
	if lastContext != "" {
		token, err := Load(lastContext)
		if err == nil && token != nil {
			return lastContext, *token
		}
	}
	return "", Entry{
		AccessTokenExpire:  time.Time{},
		AccessToken:        "",
		RefreshTokenExpire: time.Time{},
		RefreshToken:       "",
	}
}

func FmtFilename(contextName string) string {
	return clientcontext.ConfigFolder + "token-" + contextName + ".json"
}

func ReconnectError(srcErr error, contextName string) error {
	fullPath, err2 := homedir.Expand(FmtFilename(contextName))
	if err2 != nil {
		return err2
	}
	return fmt.Errorf("%w at %s: use `om context login` to authenticate", srcErr, fullPath)
}

func ModTime(contextName string) (time.Time, error) {
	filename := FmtFilename(contextName)
	filename, err := homedir.Expand(filename)
	if err != nil {
		return time.Time{}, err
	}
	info, err := os.Stat(filename)
	if err != nil {
		return time.Time{}, err
	}
	return info.ModTime(), nil
}
