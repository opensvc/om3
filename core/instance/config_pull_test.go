package instance

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/resource"
)

// Two configurations of the same file differ when the image a resource
// pulls does: it changes with the engine and the prefetch default of the
// node, while the file stays the same.
func TestConfigEqualComparesThePulls(t *testing.T) {
	config := func(resources ResourceConfigs) *Config {
		return &Config{Checksum: "c1", Scope: []string{"n1"}, ActorConfig: &ActorConfig{Resources: resources}}
	}
	docker := &resource.Pull{Engine: "docker", Image: "docker.io/library/nginx:latest"}
	sameDocker := &resource.Pull{Engine: "docker", Image: "docker.io/library/nginx:latest"}
	podman := &resource.Pull{Engine: "podman", Image: "docker.io/library/nginx:latest"}
	base := config(ResourceConfigs{"container#1": {Pull: docker}, "fs#1": {}})

	assert.True(t, ConfigEqual(base, config(ResourceConfigs{"container#1": {Pull: sameDocker}, "fs#1": {}})))
	assert.False(t, ConfigEqual(base, config(ResourceConfigs{"container#1": {Pull: podman}, "fs#1": {}})),
		"the node changed its engine")
	assert.False(t, ConfigEqual(base, config(ResourceConfigs{"container#1": {}, "fs#1": {}})),
		"the node disabled the prefetch")
	assert.False(t, ConfigEqual(config(ResourceConfigs{"container#1": {}, "fs#1": {}}), base),
		"the node enabled the prefetch")
	assert.False(t, ConfigEqual(base, config(ResourceConfigs{"container#1": {Pull: docker}, "container#2": {Pull: docker}, "fs#1": {}})),
		"another resource pulls the same image")
	assert.True(t, ConfigEqual(base, config(ResourceConfigs{"container#1": {Pull: docker}})),
		"a resource pulling nothing")
}

// The Pull of a resource is published with the instance configuration, and
// a copy of the configuration does not share it.
func TestResourceConfigPull(t *testing.T) {
	pull := &resource.Pull{Engine: "podman", Store: "app", Image: "ghcr.io/opensvc/pause:latest", Creds: "ns1/sec/creds"}
	cfg := Config{ActorConfig: &ActorConfig{Resources: ResourceConfigs{"container#1": {Pull: pull}, "fs#1": {}}}}

	b, err := json.Marshal(cfg)
	require.NoError(t, err)
	assert.Contains(t, string(b), `"pull":{"engine":"podman","store":"app","image":"ghcr.io/opensvc/pause:latest","creds":"ns1/sec/creds"}`)
	var back Config
	require.NoError(t, json.Unmarshal(b, &back))
	assert.Equal(t, pull, back.Resources["container#1"].Pull)
	assert.Nil(t, back.Resources["fs#1"].Pull)

	clone := cfg.DeepCopy()
	require.Equal(t, pull, clone.Resources["container#1"].Pull)
	clone.Resources["container#1"].Pull.Engine = "docker"
	assert.Equal(t, "podman", pull.Engine, "the copy has a Pull of its own")

	assert.Equal(t, map[string]any{"engine": "podman", "store": "app", "image": "ghcr.io/opensvc/pause:latest", "creds": "ns1/sec/creds"},
		cfg.Resources["container#1"].Unstructured()["pull"])
	_, ok := cfg.Resources["fs#1"].Unstructured()["pull"]
	assert.False(t, ok)
}
