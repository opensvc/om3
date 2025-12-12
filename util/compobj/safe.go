package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
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

func collectorSafeURIToUUID(uri string) (string, error) {
	uuid := strings.Replace(uri, "safe://", "", 1)
	if strings.HasPrefix(uuid, "safe.uuid") {
		return uuid, nil
	}
	if _, err := strconv.Atoi(uuid); err != nil {
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
	url.Path += uri
	req, err := http.NewRequest(http.MethodGet, url.String(), nil)
	req.SetBasicAuth(user, password)
	req.Header.Add("Content-Type", "application/json")
	resp, err := client.Do(req)
	return resp, err
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
