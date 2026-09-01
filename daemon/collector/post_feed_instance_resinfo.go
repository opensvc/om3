package collector

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/oc3path"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/daemon/daemonauth"
	"github.com/opensvc/om3/v3/daemon/daemonenv"
	"github.com/opensvc/om3/v3/daemon/daemonsubsystem"
	"github.com/opensvc/om3/v3/daemon/msgbus"
)

type (
	// resInfoPost is the POST feed instance resinfo payload.
	resInfoPost struct {
		Info     []resource.Info `json:"info"`
		Path     string          `json:"path"`
		Topology *string         `json:"topology,omitempty"`
	}

	// resInfoSent describes the resource info sent to the collector db. It
	// prevents re-sending unchanged info and is dumped to
	// <var>/collector/resinfo_sent/<fqdn>@<node>.json so the cache survives a
	// daemon restart.
	resInfoSent struct {
		SentAt   time.Time `json:"sent_at"`
		Checksum string    `json:"csum"`

		path     naming.Path
		nodename string

		// cacheFile is the file path to store the resInfoSent struct
		cacheFile string
	}
)

// resInfoKinds are the object kinds that have resources, and so resource
// info to report. The others are datastores and configurations: asking
// the local instance for info it has no way to hold answers "object does
// not support resource info", and asking a peer's api for it answers 400.
var resInfoKinds = naming.NewKinds(naming.KindSvc, naming.KindVol)

// seedResInfoToSend queues the instances we have no sent trace for, so a node
// that becomes speaker still reports resource info refreshed while it was not
// speaking.
//
// Instances with a sent trace are skipped: they wait for the next
// InstanceResourceInfoUpdated signal or the next scheduled refresh. This bounds
// the fetch burst on speaker change.
func (t *T) seedResInfoToSend() {
	for _, v := range instance.StatusData.GetAll() {
		if !resInfoKinds.Has(v.Path.Kind) {
			continue
		}
		i := instance.InstanceString(v.Path, v.Node)
		if _, ok := t.resInfoToSend[i]; ok {
			continue
		}
		if _, ok := t.resInfoSent[i]; ok {
			continue
		}
		sent := resInfoSent{path: v.Path, nodename: v.Node}
		if err := sent.read(); err == nil {
			t.resInfoSent[i] = sent
			continue
		} else if !errors.Is(err, fs.ErrNotExist) {
			t.log.Warnf("can't read sent resource info flag for %s: %s", i, err)
		}
		t.log.Tracef("seed resource info to send for %s", i)
		t.resInfoToSend[i] = &msgbus.InstanceResourceInfoUpdated{Path: v.Path, Node: v.Node}
	}
}

func (t *T) sendResInfoChange() {
	t.log.Tracef("sendResInfoChange")
	for i, v := range t.resInfoToSend {
		infos, err := t.getResInfo(v.Path, v.Node)
		if err != nil {
			// skip os.ErrNotExist: the instance may never have refreshed its
			// cache, or may have been deleted.
			if !errors.Is(err, os.ErrNotExist) {
				t.log.Warnf("skip send resource info %s: %s", i, err)
			}
			delete(t.resInfoToSend, i)
			continue
		}
		if err := t.doPostResInfo(v, infos); err != nil {
			// keep it queued, retried on a later tick
			t.log.Warnf("post resource info %s: %s", i, err)
			continue
		}
		delete(t.resInfoToSend, i)
	}
}

// getResInfo returns the resource info key-values of the p instance on nodename,
// read from the local cache when nodename is localhost, else fetched from the
// nodename daemon api.
func (t *T) getResInfo(p naming.Path, nodename string) (resource.Infos, error) {
	if nodename == t.localhost {
		return t.getLocalResInfo(p)
	}
	return t.getPeerResInfo(p, nodename)
}

func (t *T) getLocalResInfo(p naming.Path) (resource.Infos, error) {
	type loadResInfoer interface {
		LoadResInfo() (resource.Infos, error)
	}
	infos := resource.NewInfos(p)
	o, err := object.New(p)
	if err != nil {
		return infos, err
	}
	i, ok := o.(loadResInfoer)
	if !ok {
		return infos, fmt.Errorf("object does not support resource info")
	}
	return i.LoadResInfo()
}

func (t *T) getPeerResInfo(p naming.Path, nodename string) (resource.Infos, error) {
	infos := resource.NewInfos(p)
	tk, err := daemonauth.CreateNodeToken()
	if err != nil {
		return infos, err
	}
	c, err := client.New(
		client.WithURL(daemonsubsystem.PeerURL(nodename)),
		client.WithBearer(tk),
		client.WithCertificate(daemonenv.CertChainFile()),
	)
	if err != nil {
		return infos, fmt.Errorf("new client: %w", err)
	}
	ctx, cancel := context.WithTimeout(t.ctx, defaultPostMaxDuration)
	defer cancel()
	resp, err := c.GetInstanceResourceInfoWithResponse(ctx, nodename, p.Namespace, p.Kind, p.Name)
	if err != nil {
		return infos, err
	}
	switch {
	case resp.JSON200 != nil:
		return resInfoFromAPI(p, resp.JSON200.Items), nil
	case resp.StatusCode() == http.StatusNotFound:
		return infos, os.ErrNotExist
	default:
		return infos, fmt.Errorf("unexpected response: %s", resp.Status())
	}
}

// resInfoFromAPI rebuilds the resource.Infos from the flat key-value items the
// api serves, preserving the item order so the collector sees a stable rid
// ordering.
func resInfoFromAPI(p naming.Path, items api.ResourceInfoItems) resource.Infos {
	infos := resource.NewInfos(p)
	index := make(map[string]int)
	for _, item := range items {
		i, ok := index[item.Rid]
		if !ok {
			infos.Resources = append(infos.Resources, resource.Info{RID: item.Rid})
			i = len(infos.Resources) - 1
			index[item.Rid] = i
		}
		infos.Resources[i].Keys = append(infos.Resources[i].Keys, resource.InfoKey{
			Key:   item.Key,
			Value: item.Value,
		})
	}
	return infos
}

func (t *T) doPostResInfo(v *msgbus.InstanceResourceInfoUpdated, infos resource.Infos) error {
	if t.client == nil {
		return nil
	}
	var (
		method = http.MethodPost
		path   = oc3path.FeedInstanceResinfo
	)
	i := instance.InstanceString(v.Path, v.Node)

	data := resInfoPost{
		Info: infos.Resources,
		Path: v.Path.String(),
	}
	if cfg := instance.ConfigData.GetByPathAndNode(v.Path, v.Node); cfg != nil {
		topology := cfg.Topology.String()
		data.Topology = &topology
	}

	b, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("encode request body: %w", err)
	}

	ctx, cancel := context.WithTimeout(t.ctx, defaultPostMaxDuration)
	defer cancel()

	req, err := t.client.NewRequestWithContext(ctx, method, path, bytes.NewBuffer(b))
	if err != nil {
		return fmt.Errorf("%s %s create request: %w", method, path, err)
	}

	t.log.Debugf("%s %s %s", method, path, i)
	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("%s %s: %w", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("%s %s unexpected status code: wanted %d got %d",
			method, path, http.StatusAccepted, resp.StatusCode)
	}
	t.log.Tracef("%s %s %s status code %d", method, path, i, resp.StatusCode)
	sent := resInfoSent{
		path:     v.Path,
		nodename: v.Node,
		Checksum: v.Checksum,
		SentAt:   time.Now(),
	}
	if err := sent.write(); err != nil {
		return err
	}
	t.resInfoSent[i] = sent
	return nil
}

func (o *resInfoSent) filename() string {
	if o == nil {
		return ""
	}
	if len(o.cacheFile) == 0 {
		if o.path.IsZero() {
			return ""
		}
		flat := fmt.Sprintf("%s.%s.%s@%s.json", o.path.Namespace, o.path.Kind, o.path.Name, o.nodename)
		o.cacheFile = filepath.FromSlash(filepath.Join(rawconfig.CollectorResInfoSentDir(), flat))
	}
	return o.cacheFile
}

func (o *resInfoSent) write() error {
	if o == nil || o.path.IsZero() {
		return ErrZeroPath
	}
	sentTrace := o.filename()
	f, err := os.Create(sentTrace)
	if err != nil {
		if err1 := os.MkdirAll(filepath.Dir(sentTrace), 0755); err1 != nil {
			return errors.Join(err, err1)
		}
		if f, err = os.Create(sentTrace); err != nil {
			return err
		}
	}
	defer func() { _ = f.Close() }()
	return json.NewEncoder(f).Encode(o)
}

func (o *resInfoSent) read() error {
	if o == nil || o.path.IsZero() {
		return ErrZeroPath
	}
	f, err := os.Open(o.filename())
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	return json.NewDecoder(f).Decode(&o)
}

func (o *resInfoSent) drop() error {
	if o == nil || o.path.IsZero() {
		return ErrZeroPath
	}
	if err := os.Remove(o.filename()); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}
