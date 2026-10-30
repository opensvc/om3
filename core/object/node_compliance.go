package object

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/opensvc/om3/v3/core/oc3path"
	"github.com/opensvc/om3/v3/util/compliance"
)

// complianceModulesTimeout bounds the download of the compliance modules.
const complianceModulesTimeout = 5 * time.Minute

func (t Node) NewCompliance() (*compliance.T, error) {
	client, err := t.CollectorComplianceClient()
	if err != nil {
		return nil, err
	}
	comp := compliance.New()
	comp.SetCollectorClient(client)
	return comp, nil
}

// UpdateCompliance installs the compliance modules of the collector and
// returns the commit of the modules repository installed.
func (t *Node) UpdateCompliance() (string, error) {
	var (
		method = http.MethodGet
		path   = oc3path.ServerComplianceModules
	)
	oc3, err := t.CollectorServer()
	if err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(context.Background(), complianceModulesTimeout)
	defer cancel()

	req, err := oc3.NewRequestWithContext(ctx, method, path, nil)
	if err != nil {
		return "", fmt.Errorf("create collector request %s %s: %w", method, path, err)
	}
	resp, err := oc3.Do(req)
	if err != nil {
		return "", fmt.Errorf("collector %s %s: %w", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return "", fmt.Errorf("collector %s %s: no compliance module to install", method, path)
	default:
		return "", fmt.Errorf("unexpected collector response status code for %s %s: wanted %d got %d",
			method, path, http.StatusOK, resp.StatusCode)
	}
	if err := compliance.New().Install(resp.Body); err != nil {
		return "", err
	}
	return resp.Header.Get("X-Compliance-Modules-Commit"), nil
}
