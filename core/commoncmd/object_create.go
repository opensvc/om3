package commoncmd

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/keyop"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/util/uri"
)

func DataFromConfigHttp(u uri.T) ([]byte, error) {
	fpath, err := u.CallerFetch()
	if err != nil {
		return nil, err
	}
	defer os.Remove(fpath)
	return DataFromConfigFile(fpath)
}

func DataFromConfigFile(fpath string) ([]byte, error) {
	b, err := os.ReadFile(fpath)
	if err != nil {
		return nil, err
	}
	return b, err
}

func DataFromStdin() ([]byte, error) {
	b, err := io.ReadAll(os.Stdin)
	if err != nil {
		return nil, err
	}
	return b, err
}

func DataFromTemplate(template string) ([]byte, error) {
	return nil, fmt.Errorf("todo: collector requester")
}

// CreateObjectConfig creates the object at p from the configuration b,
// through the api, which checks it as any configuration it writes, and
// answers once the daemon knows the object, so the command run next finds
// it.
//
// The daemon leaves behind what a configuration copied from another object
// recorded of itself, unless restore, and applies the keyword operations
// after, so a keyword they name is kept. With force, an object that exists
// already has its configuration replaced, with the same treatment.
//
// A restore replaces an object that exists too, as the create does with no
// daemon running: what a backup or a hypervisor restores is the object as it
// was saved, whatever the node holds of it.
//
// With dryRun, the daemon runs every check of the create and writes nothing,
// and the alerts it answers with are printed: the problems the create would
// be refused for, and the warnings of the validation.
//
// With wait, the daemon holds the answer that long at most for every node of
// the object to have the configuration, and for itself to know the object,
// which lasts as long as the first status of its instance: without, it holds
// it CreateKnownHold at most for itself to know the object.
func CreateObjectConfig(ctx context.Context, c *client.T, p naming.Path, b []byte, keywords []string, restore, force bool, wait time.Duration, dryRun *DryRun) error {
	return postObjectConfig(ctx, c, p, b, createParams(keywords, restore, wait, dryRun != nil), force, restore, dryRun)
}

// CreateObjectConfigFromStore creates the object at p from the snapshot of
// the store, through the api: the daemon reads the configuration of the
// object the snapshot is of, creates the keys of the sec and cfg it
// references that the namespace of p lacks, and names the snapshot the
// provision restores the heads from.
func CreateObjectConfigFromStore(ctx context.Context, c *client.T, p naming.Path, store, snapshot string, keywords []string, restore, force bool, wait time.Duration, dryRun *DryRun) error {
	params := createParams(keywords, restore, wait, dryRun != nil)
	params.Store = &store
	params.Snapshot = &snapshot
	return postObjectConfig(ctx, c, p, nil, params, force, restore, dryRun)
}

// CreateKnownHold is how long the daemon holds the answer of a create asked
// with no wait for it to know the object, its objectKnownTimeout.
const CreateKnownHold = 30 * time.Second

// createAnswerMargin is how much longer than the daemon holds a create the
// client waits for its answer: the 408 the daemon answers past its hold says
// the object is created, and a client giving up first says nothing of it.
const createAnswerMargin = 10 * time.Second

// CreateClientTimeout is the timeout of the client posting a create asked
// with wait, zero for none, for the answer of the daemon to reach it.
func CreateClientTimeout(wait time.Duration) time.Duration {
	return max(wait, CreateKnownHold) + createAnswerMargin
}

func createParams(keywords []string, restore bool, wait time.Duration, dryRun bool) api.PostObjectConfigFileParams {
	params := api.PostObjectConfigFileParams{}
	if dryRun {
		// Nothing is written for the daemon to learn of.
		params.DryRun = &dryRun
	} else {
		waitLocal := true
		params.WaitLocal = &waitLocal
		if wait > 0 {
			// The daemon holds the create as long as the caller
			// waits: the first status of the instance, which it knows
			// the object after, may last longer than CreateKnownHold.
			w := api.Wait(wait.String())
			params.Wait = &w
		}
	}
	if restore {
		params.Restore = &restore
	}
	if len(keywords) > 0 {
		params.Set = &keywords
	}
	return params
}

func postObjectConfig(ctx context.Context, c *client.T, p naming.Path, b []byte, params api.PostObjectConfigFileParams, force, replace bool, dryRun *DryRun) error {
	resp, err := c.PostObjectConfigFileWithBodyWithResponse(ctx, p.Namespace, p.Kind, p.Name, &params, "application/octet-stream", bytes.NewReader(b))
	if err != nil {
		return err
	}
	switch resp.StatusCode() {
	case http.StatusOK:
		// A dry run. Forced, a configuration holding what others hold is
		// written all the same, so the dry run is the one of the forced
		// write, as the create asks it on a conflict.
		if dryRun != nil && force && resp.JSON200 != nil && HasConfigAlertErrorsOf(resp.JSON200.Items, api.ConfigAlertKindHeld) {
			return forceObjectConfig(ctx, c, p, b, params, "created", dryRun)
		}
		return reportDryRun(p, resp.JSON200, dryRun, "created")
	case http.StatusNoContent:
		fmt.Printf("%s: %s\n", p, doneOrWouldBe(params.DryRun, "created"))
		return nil
	case http.StatusConflict:
		// Forced, a configuration holding what others hold is written all
		// the same, and an object that exists has its configuration
		// replaced.
		held := resp.JSON409 != nil && resp.JSON409.Title == api.ProblemTitleHeld
		switch {
		case !force && held:
			return fmt.Errorf("%s: %s", p, resp.JSON409.Detail)
		case !force && !replace:
			return fmt.Errorf("%s already exists", p)
		case held:
			return forceObjectConfig(ctx, c, p, b, params, "created", dryRun)
		default:
			return forceObjectConfig(ctx, c, p, b, params, "replaced", dryRun)
		}
	case http.StatusBadRequest:
		return fmt.Errorf("%s: %s", p, *resp.JSON400)
	case http.StatusUnauthorized:
		return fmt.Errorf("%s: %s", p, *resp.JSON401)
	case http.StatusForbidden:
		return fmt.Errorf("%s: %s", p, *resp.JSON403)
	case http.StatusNotFound:
		return fmt.Errorf("%s: %s", p, *resp.JSON404)
	case http.StatusRequestTimeout:
		// Created, and not known to the daemon yet: the command run next
		// may not find it.
		return fmt.Errorf("%s: %s", p, resp.JSON408.Detail)
	case http.StatusInternalServerError:
		return fmt.Errorf("%s: %s", p, *resp.JSON500)
	default:
		return fmt.Errorf("%s: unexpected response: %s", p, resp.Status())
	}
}

// forceObjectConfig writes the configuration b of the object at p as a
// create with force does: root writes it holding what others hold, and the
// configuration of an object that exists is replaced, the daemon giving it
// the treatment of a creation, and the checks of an update. done says what
// was done, created or replaced.
func forceObjectConfig(ctx context.Context, c *client.T, p naming.Path, b []byte, params api.PostObjectConfigFileParams, done string, dryRun *DryRun) error {
	force := true
	params.Force = &force
	resp, err := c.PostObjectConfigFileWithBodyWithResponse(ctx, p.Namespace, p.Kind, p.Name, &params, "application/octet-stream", bytes.NewReader(b))
	if err != nil {
		return err
	}
	switch resp.StatusCode() {
	case http.StatusOK:
		return reportDryRun(p, resp.JSON200, dryRun, done)
	case http.StatusNoContent:
		fmt.Printf("%s: %s\n", p, doneOrWouldBe(params.DryRun, done))
		return nil
	case http.StatusBadRequest:
		return fmt.Errorf("%s: %s", p, *resp.JSON400)
	case http.StatusUnauthorized:
		return fmt.Errorf("%s: %s", p, *resp.JSON401)
	case http.StatusForbidden:
		return fmt.Errorf("%s: %s", p, *resp.JSON403)
	case http.StatusNotFound:
		return fmt.Errorf("%s: %s", p, *resp.JSON404)
	case http.StatusConflict:
		return fmt.Errorf("%s: %s", p, *resp.JSON409)
	case http.StatusRequestTimeout:
		return fmt.Errorf("%s: %s", p, resp.JSON408.Detail)
	case http.StatusInternalServerError:
		return fmt.Errorf("%s: %s", p, *resp.JSON500)
	default:
		return fmt.Errorf("%s: unexpected response: %s", p, resp.Status())
	}
}

// CreatedConfigData returns the configurer reading the configuration b of
// an object created at p, as the api writes it, to tell what its validation
// finds before it is sent.
func CreatedConfigData(p naming.Path, b []byte, keywords []string, restore bool) (object.Configurer, error) {
	oc, _, err := object.CreatedConfig(p, b, restore, keyop.ParseOps(keywords)...)
	return oc, err
}

// reportDryRun prints the alerts the daemon answered a dry run with. An
// answer with alerts to a write, which a daemon answers with no content, is
// an unexpected one.
func reportDryRun(p naming.Path, alerts *api.ConfigAlertList, dryRun *DryRun, done string) error {
	if dryRun == nil {
		return fmt.Errorf("%s: unexpected response to a write: alerts", p)
	}
	return dryRun.Report(p, alerts, done)
}

// doneOrWouldBe says what a configuration write did, or would have done in a
// dry run.
func doneOrWouldBe(dryRun *bool, done string) string {
	if dryRun != nil && *dryRun {
		return "would be " + done
	}
	return done
}
