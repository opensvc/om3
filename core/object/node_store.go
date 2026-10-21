package object

import (
	"fmt"
	"strings"

	"github.com/opensvc/om3/v3/core/backup"
	"github.com/opensvc/om3/v3/util/key"
)

// StoreItem is a store declared in the cluster configuration.
type StoreItem struct {
	Ref  string `json:"ref"`
	Type string `json:"type"`
	URL  string `json:"url"`
}

// Store returns the driver of the store ref names, configured from its
// section.
//
// A bare name is a store#<name> section of the cluster configuration. The
// driver is chosen by the type the section declares.
func (t *Node) Store(ref string) (backup.Driver, error) {
	if ref == "" {
		return nil, fmt.Errorf("no store named")
	}
	if strings.ContainsAny(ref, " /") {
		return nil, fmt.Errorf("store %q: only the stores of the cluster configuration, named bare, are supported", ref)
	}
	section := backup.SectionName(ref)
	config := t.MergedConfig()
	if !config.HasSectionString(section) {
		return nil, fmt.Errorf("store %q: no %s section in the cluster configuration", ref, section)
	}
	storeType := config.GetString(key.New(section, "type"))
	drv := backup.GetDriver(storeType)
	if drv == nil {
		return nil, fmt.Errorf("store %q: no driver for the store type %q", ref, storeType)
	}
	drv.SetName(section)
	drv.SetConfig(config)
	return drv, nil
}

// ListStores returns the stores declared in the cluster configuration.
func (t *Node) ListStores() []StoreItem {
	l := make([]StoreItem, 0)
	config := t.MergedConfig()
	for _, s := range config.SectionStrings() {
		if !strings.HasPrefix(s, backup.SectionPrefix) {
			continue
		}
		l = append(l, StoreItem{
			Ref:  strings.TrimPrefix(s, backup.SectionPrefix),
			Type: config.GetString(key.New(s, "type")),
			URL:  config.GetString(key.New(s, "url")),
		})
	}
	return l
}
