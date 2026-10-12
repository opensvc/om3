package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/key"
)

type (
	SafeFileMetaResponse struct {
		Data []SafeFileMeta `json:"data"`
	}
	SafeFileMeta struct {
		Name         string `json:"name"`
		UploadedFrom string `json:"uploaded_from"`
		UUID         string `json:"uuid"`
		//UploadedAt   time.Time `json:"uploaded_date"`
		Uploader int    `json:"uploader"`
		MD5      string `json:"md5"`
		ID       int64  `json:"id"`
		Size     int64  `json:"size"`
	}
)

// regexpSafeID is the id of a file of the collector safe: its number, or its
// uuid, with the extension of the file uploaded.
var regexpSafeID = regexp.MustCompile(`^([0-9]+|safe\.uuid\.[0-9a-fA-F-]+(\.[A-Za-z0-9_-]+)*)$`)

// collectorSafeURIToUUID returns the id of the safe file a safe uri names.
//
// The id is a component of the path of the collector api the file is
// fetched from, with the credentials of the node: one holding a '/', a '..'
// or a query is refused, so it names a safe file and nothing else.
func collectorSafeURIToUUID(uri string) (string, error) {
	uuid := strings.Replace(uri, "safe://", "", 1)
	if !regexpSafeID.MatchString(uuid) {
		return "", fmt.Errorf("invalid safe uri. use safe://<int> or safe.uuid.<hex>.<ext>")
	}
	return uuid, nil
}

func collectorSafeGetFile(uuid string) ([]byte, error) {
	uuid, err := collectorSafeURIToUUID(uuid)
	if err != nil {
		return nil, err
	}
	return collectorRestGetFile("/safe/" + uuid + "/download")
}

func collectorRestGet(uri string) (*http.Response, error) {
	node, err := object.NewNode()
	if err != nil {
		return nil, err
	}
	user := hostname.Hostname()
	password := node.Config().GetString(key.Parse("node.uuid"))
	client := node.CollectorRestAPIClient()
	url, err := node.CollectorRestAPIURL()
	if err != nil {
		return nil, err
	}
	// The request carries the credentials of the node, as v2 refuses to
	// send them in clear.
	if url.Scheme != "https" {
		return nil, fmt.Errorf("refuse to submit the node credentials through a non-encrypted transport: %s", url.Redacted())
	}
	url.Path += uri
	req, err := http.NewRequest(http.MethodGet, url.String(), nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(user, password)
	req.Header.Add("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		// An error page is no content of the file asked for.
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		_ = resp.Body.Close()
		return nil, fmt.Errorf("collector %s: %s: %s", uri, resp.Status, strings.TrimSpace(string(b)))
	}
	return resp, nil
}

func collectorRestGetFile(uri string) ([]byte, error) {
	resp, err := collectorRestGet(uri)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	return b, err
}

func collectorSafeGetMeta(uuid string) (SafeFileMeta, error) {
	uuid, err := collectorSafeURIToUUID(uuid)
	if err != nil {
		return SafeFileMeta{}, err
	}
	resp, err := collectorRestGet("/safe/" + uuid)
	if err != nil {
		return SafeFileMeta{}, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return SafeFileMeta{}, err
	}
	r := SafeFileMetaResponse{}
	if err := json.Unmarshal(b, &r); err != nil {
		return SafeFileMeta{}, err
	}
	if len(r.Data) == 0 {
		return SafeFileMeta{}, fmt.Errorf("safe file %s not found", uuid)
	}
	return r.Data[0], nil
}
