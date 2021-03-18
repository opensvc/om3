package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"opensvc.com/opensvc/core/resource"
	"opensvc.com/opensvc/core/status"
	"opensvc.com/opensvc/util/file"
)

// Start the Resource
func (t T) Start() error {
	if t.exists() {
		return nil
	}
	if t.file() == "" {
		t.Log.Error("empty file path")
		return errors.New("empty file path")
	}
	if err := os.MkdirAll(t.dir(), os.ModePerm); err != nil {
		t.Log.Error("%s", err)
		return err
	}
	//t.log.Info().Msgf("create flag %s", t.file())
	if _, err := os.Create(t.file()); err != nil {
		t.Log.Error("%s", err)
		return err
	}
	return nil
}

// Stop the Resource
func (t T) Stop() error {
	if !t.exists() {
		return nil
	}
	if t.file() == "" {
		t.Log.Error("empty file path")
		return errors.New("empty file path")
	}
	//t.log.Info().Msgf("remove flag %s", t.file())
	if err := os.Remove(t.file()); err != nil {
		t.Log.Error("%s", err)
		return err
	}
	return nil
}

// Label returns a formatted short description of the Resource
func (t T) Label() string {
	return t.file()
}

// Status evaluates and display the Resource status and logs
func (t *T) Status() status.T {
	if t.file() == "" {
		t.Log.Error("empty file path")
		return status.NotApplicable
	}
	if t.exists() {
		return status.Up
	}
	return status.Down
}

func (t T) exists() bool {
	return file.Exists(t.file())
}

func (t *T) file() string {
	if t.lazyFile != "" {
		return t.lazyFile
	}
	if t.dir() == "" {
		return ""
	}
	p := fmt.Sprintf("%s/%s.flag", t.dir(), t.ResourceID)
	t.lazyFile = filepath.FromSlash(p)
	return t.lazyFile
}

func (t T) dir() string {
	var p string
	if t.lazyDir != "" {
		return t.lazyDir
	}
	if t.Path.Namespace != "root" {
		p = fmt.Sprintf("%s/%s/%s/%s", t.baseDir(), t.Path.Namespace, t.Path.Kind, t.Path.Name)
	} else {
		p = fmt.Sprintf("%s/%s/%s", t.baseDir(), t.Path.Kind, t.Path.Name)
	}
	t.lazyDir = filepath.FromSlash(p)
	return t.lazyDir
}

func main() {
	r := &T{}
	if err := resource.NewLoader(os.Stdin).Load(r); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	resource.Action(r)
}
