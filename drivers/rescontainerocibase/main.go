package rescontainerocibase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/provisioned"
	"github.com/opensvc/om3/core/resource"
	"github.com/opensvc/om3/core/resourceid"
	"github.com/opensvc/om3/util/pg"
)

type (
	BT struct {
		resource.T
		resource.SCSIPersistentReservation
		ObjectDomain    string         `json:"object_domain"`
		PG              pg.Config      `json:"pg"`
		Path            naming.Path    `json:"path"`
		ObjectID        uuid.UUID      `json:"object_id"`
		SCSIReserv      bool           `json:"scsireserv"`
		PromoteRW       bool           `json:"promote_rw"`
		NoPreemptAbort  bool           `json:"no_preempt_abort"`
		OsvcRootPath    string         `json:"osvc_root_path"`
		GuestOS         string         `json:"guest_os"`
		Name            string         `json:"name"`
		Hostname        string         `json:"hostname"`
		Image           string         `json:"image"`
		ImagePullPolicy string         `json:"image_pull_policy"`
		CWD             string         `json:"cwd"`
		User            string         `json:"user"`
		Command         []string       `json:"command"`
		DNS             []string       `json:"dns"`
		DNSSearch       []string       `json:"dns_search"`
		RunArgs         []string       `json:"run_args"`
		Entrypoint      []string       `json:"entrypoint"`
		Detach          bool           `json:"detach"`
		Remove          bool           `json:"remove"`
		Privileged      bool           `json:"privileged"`
		Init            bool           `json:"init"`
		Interactive     bool           `json:"interactive"`
		TTY             bool           `json:"tty"`
		VolumeMounts    []string       `json:"volume_mounts"`
		Env             []string       `json:"environment"`
		SecretsEnv      []string       `json:"secrets_environment"`
		ConfigsEnv      []string       `json:"configs_environment"`
		Devices         []string       `json:"devices"`
		NetNS           string         `json:"netns"`
		UserNS          string         `json:"userns"`
		PIDNS           string         `json:"pidns"`
		IPCNS           string         `json:"ipcns"`
		UTSNS           string         `json:"utsns"`
		RegistryCreds   string         `json:"registry_creds"`
		PullTimeout     *time.Duration `json:"pull_timeout"`
		StartTimeout    *time.Duration `json:"start_timeout"`
		StopTimeout     *time.Duration `json:"stop_timeout"`
	}

	Arg struct {
		Short     string
		Long      string
		Default   string
		Obfuscate bool
		Multi     bool
	}

	Argser interface {
		Args() []Arg
	}

	ImagePullOptions struct {
		Name string
	}

	CreateOptions struct {
		Name  string
		Image string
	}

	containerNamer interface {
		ContainerName() string
	}
)

const (
	imagePullPolicyAlways = "always"
	imagePullPolicyOnce   = "once"
)

// ContainerName formats a docker container name
func (t *BT) ContainerName() string {
	if t.Name != "" {
		return t.Name
	}
	var s string
	switch t.Path.Namespace {
	case "root", "":
		s = ""
	default:
		s = t.Path.Namespace + ".."
	}
	s = s + t.Path.Name + "." + strings.ReplaceAll(t.ResourceID.String(), "#", ".")
	return s
}

func (t *BT) IsAlwaysImagePullPolicy() bool {
	return t.ImagePullPolicy == imagePullPolicyAlways
}

func (t *BT) NeedPreStartRemove() bool {
	return t.Remove || !t.Detach
}

var (
	ErrNotFound = errors.New("not found")
)

func (t *BT) FormatNS(s string) (string, error) {
	switch s {
	case "", "none", "host":
		return s, nil
	}
	rid, err := resourceid.Parse(s)
	if err != nil {
		return "", fmt.Errorf("invalid value %s (must be none, host or container#<n>)", s)
	}
	r := t.GetObjectDriver().ResourceByID(rid.String())
	if r == nil {
		return "", fmt.Errorf("resource %s not found", s)
	}
	if i, ok := r.(containerNamer); ok {
		name := i.ContainerName()
		return "container:" + name, nil
	}
	return "", fmt.Errorf("resource %s has no ns", s)
}

func (t *BT) Label() string {
	return t.Image
}

func (t *BT) Labels() map[string]string {
	data := make(map[string]string)
	data["com.opensvc.id"] = t.containerLabelID()
	data["com.opensvc.path"] = t.Path.String()
	data["com.opensvc.namespace"] = t.Path.Namespace
	data["com.opensvc.kind"] = t.Path.Kind.String()
	data["com.opensvc.name"] = t.Path.Name
	data["com.opensvc.rid"] = t.ResourceID.String()
	return data
}

func (t *BT) LinkNames() []string {
	return []string{t.RID()}
}

func (t *BT) Provision(_ context.Context) error {
	return nil
}

func (t *BT) Provisioned() (provisioned.T, error) {
	return provisioned.NotApplicable, nil
}

func (t *BT) Unprovision(_ context.Context) error {
	return nil
}

func (t *BT) containerLabelID() string {
	return fmt.Sprintf("%s.%s", t.ObjectID, t.ResourceID.String())
}
