package object

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/opensvc/om3/v3/core/oc3path"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/util/sysreport"
)

var (
	// sysreportPostTimeout bounds the post of a sysreport, whose first
	// archive holds every tracked file.
	sysreportPostTimeout = 5 * time.Minute
)

// Sysreport sends an archive of modified files the agent is configured
// to track, and the list of files deleted since the last call.
//
// The collector is in charge of versioning this information and of
// reporting on changes.
func (t Node) Sysreport() error {
	return t.newSysreport().Do()
}

// ForceSysreport sends every file the agent tracks, as a full report the
// collector replaces what it holds of the node with.
func (t Node) ForceSysreport() error {
	sr := t.newSysreport()
	sr.SetForce(true)
	return sr.Do()
}

func (t Node) newSysreport() *sysreport.T {
	sr := sysreport.New(rawconfig.Paths.Etc, rawconfig.Paths.Var)
	sr.SetSender(t)
	return sr
}

// SendSysreport posts a sysreport to the oc3 feeder, as a multipart form: the
// archive in the "file" part, when there is one, each deleted file in a
// "deleted" part, and the "full" flag.
func (t Node) SendSysreport(archive string, deleted []string, full bool) error {
	oc3, err := t.CollectorFeeder()
	if err != nil {
		return err
	}
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	if archive != "" {
		if err := addSysreportArchivePart(w, archive); err != nil {
			return err
		}
	}
	for _, path := range deleted {
		if err := w.WriteField("deleted", path); err != nil {
			return err
		}
	}
	if err := w.WriteField("full", strconv.FormatBool(full)); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), sysreportPostTimeout)
	defer cancel()
	method, path := http.MethodPost, oc3path.FeedNodeSysreport
	req, err := oc3.NewRequestWithContext(ctx, method, path, &body)
	if err != nil {
		return fmt.Errorf("create collector request %s %s: %w", method, path, err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	resp, err := oc3.Do(req)
	if err != nil {
		return fmt.Errorf("collector %s %s: %w", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusAccepted {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("unexpected collector response status code for %s %s: wanted %d got %d: %s",
			method, path, http.StatusAccepted, resp.StatusCode, bytes.TrimSpace(b))
	}
	return nil
}

func addSysreportArchivePart(w *multipart.Writer, archive string) error {
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer f.Close()
	part, err := w.CreateFormFile("file", filepath.Base(archive))
	if err != nil {
		return err
	}
	_, err = io.Copy(part, f)
	return err
}
