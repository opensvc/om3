package collector

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/xmap"
)

func (t *T) sendCollectorData() error {
	if t.featurePostChange {
		return t.sendCollectorDataFeatureChange()
	} else {
		return t.sendCollectorDataLegacy()
	}
}

func (t *T) sendCollectorDataFeatureChange() error {
	if t.hasChanges() {
		return t.postChanges()
	} else {
		return t.postPing()
	}
}

func (t *T) sendCollectorDataLegacy() error {
	if t.hasDaemonStatusChange() {
		return t.postStatus()
	} else {
		return t.postPing()
	}
}

func (t *T) hasChanges() bool {
	change := t.changes
	if len(change.instanceStatusUpdates) > 0 {
		return true
	}
	if len(change.instanceStatusDeletes) > 0 {
		return true
	}
	return false
}

func (t *T) hasDaemonStatusChange() bool {
	return len(t.daemonStatusChange) > 0
}

func (t *T) postPing() error {
	if t.client == nil {
		t.previousUpdatedAt = time.Time{}
		t.dropChanges()
		return nil
	}
	var (
		req  *http.Request
		resp *http.Response

		err error

		method = http.MethodPost
		path   = "/oc3/feed/daemon/ping"
	)
	instances := make([]string, 0, len(t.instances))
	for k := range t.instances {
		instances = append(instances, k)
	}
	now := time.Now()

	ctx, cancel := context.WithTimeout(t.ctx, defaultPostMaxDuration)
	defer cancel()

	req, err = t.client.NewRequestWithContext(ctx, method, path, nil)
	if err != nil {
		return fmt.Errorf("%s %s create request: %w", method, path, err)
	}

	t.log.Debugf("%s %s", method, path)
	resp, err = t.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	t.log.Debugf("%s %s status code %d", method, path, resp.StatusCode)
	switch resp.StatusCode {
	case http.StatusNoContent:
		// collector detect out of sync
		t.initChanges()
		t.previousUpdatedAt = time.Time{}
		return nil
	case http.StatusAccepted:
		// collector accept changes, we can drop pending change
		t.previousUpdatedAt = now
		t.dropChanges()
		return nil
	default:
		return fmt.Errorf("%s %s unexpected status code %d", method, path, resp.StatusCode)
	}
}

func (t *T) postChanges() error {
	if t.client == nil {
		t.previousUpdatedAt = time.Time{}
		t.dropChanges()
		return nil
	}
	var (
		req  *http.Request
		resp *http.Response

		ioReader io.Reader

		err error

		method = http.MethodPost
		path   = "/oc3/feed/daemon/changes"
	)
	now := time.Now()

	if b, err := t.changes.asPostBody(t.previousUpdatedAt, now); err != nil {
		return fmt.Errorf("post daemon change body: %s", err)
	} else {
		ioReader = bytes.NewBuffer(b)
	}

	ctx, cancel := context.WithTimeout(t.ctx, defaultPostMaxDuration)
	defer cancel()

	req, err = t.client.NewRequestWithContext(ctx, method, path, ioReader)
	if err != nil {
		return fmt.Errorf("%s %s create request: %w", method, path, err)
	}

	t.log.Debugf("%s %s from %s -> %s", method, path, t.previousUpdatedAt, now)
	resp, err = t.client.Do(req)
	if err != nil {
		return fmt.Errorf("post daemon change call: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	t.log.Debugf("post daemon change status code %d", resp.StatusCode)
	switch resp.StatusCode {
	case http.StatusConflict:
		// collector detect out of sync (collector previousUpdatedAt is not t.previousUpdatedAt), recreate full
		t.initChanges()
		t.previousUpdatedAt = time.Time{}
		return nil
	case http.StatusAccepted:
		// collector accept changes, we can drop pending change
		t.previousUpdatedAt = now
		t.dropChanges()
		return nil
	default:
		t.log.Warnf("post daemon change unexpected status code %d", resp.StatusCode)
		return fmt.Errorf("post daemon change unexpected status code %d", resp.StatusCode)
	}
}

func (t *T) postStatus() error {
	if t.client == nil {
		t.previousUpdatedAt = time.Time{}
		t.dropChanges()
		return nil
	}
	var (
		req      *http.Request
		resp     *http.Response
		err      error
		ioReader io.Reader
		method   = http.MethodPost
		path     = "/oc3/feed/daemon/status"
	)
	now := time.Now()
	body := statusPost{
		PreviousUpdatedAt: t.previousUpdatedAt,
		UpdatedAt:         now,
		Data:              t.clusterData.ClusterData(),
		Changes:           xmap.Keys(t.daemonStatusChange),
		Version:           t.version,
	}
	if body.Data == nil {
		return fmt.Errorf("%s %s abort on empty cluster data", method, path)
	}
	if b, err := json.Marshal(body); err != nil {
		return fmt.Errorf("post daemon status body: %s", err)
	} else {
		ioReader = bytes.NewBuffer(b)
	}

	ctx, cancel := context.WithTimeout(t.ctx, defaultPostMaxDuration)
	defer cancel()

	req, err = t.client.NewRequestWithContext(ctx, method, path, ioReader)
	if err != nil {
		return fmt.Errorf("%s %s create request: %w", method, path, err)
	}

	req.Header.Set(headerPreviousUpdatedAt, t.previousUpdatedAt.Format(time.RFC3339Nano))

	t.log.Debugf("%s %s from %s -> %s", method, path, t.previousUpdatedAt, now)
	resp, err = t.client.Do(req)
	if err != nil {
		return fmt.Errorf("%s %s: %w", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	t.log.Infof("%s %s status code %d", method, path, resp.StatusCode)
	switch resp.StatusCode {
	case http.StatusConflict:
		// collector detect out of sync (collector previousUpdatedAt is not t.previousUpdatedAt), recreate full
		t.initChanges()
		t.previousUpdatedAt = time.Time{}
		return nil
	case http.StatusAccepted:
		// collector accept changes, we can drop pending change
		t.previousUpdatedAt = now
		t.dropChanges()
		return nil
	default:
		b := make([]byte, 512)
		l, _ := resp.Body.Read(b)
		t.log.Debugf("%s %s unexpected status code %d, response body extract: '%s'", method, path, resp.StatusCode, b[0:l])
		return fmt.Errorf("%s %s unexpected status code %d", method, path, resp.StatusCode)
	}
}

func (c *changesData) asPostBody(previous, current time.Time) ([]byte, error) {
	iStatusChanges := make([]msgbus.InstanceStatusUpdated, 0, len(c.instanceStatusUpdates))
	for _, v := range c.instanceStatusUpdates {
		iStatusChanges = append(iStatusChanges, *v)
	}
	iStatusDeletes := make([]msgbus.InstanceStatusDeleted, 0, len(c.instanceStatusDeletes))
	for _, v := range c.instanceStatusDeletes {
		iStatusDeletes = append(iStatusDeletes, *v)
	}

	return json.Marshal(changesPost{
		PreviousUpdatedAt:     previous,
		UpdatedAt:             current,
		InstanceStatusUpdates: iStatusChanges,
		InstanceStatusDeletes: iStatusDeletes,
	})
}
