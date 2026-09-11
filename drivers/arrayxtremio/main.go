// Package arrayxtremio drives a Dell EMC XtremIO array through its rest api.
//
// It is a port of the v2 agent driver, and answers to the same commands with
// the same options, because the collector drives an array by running them.
package arrayxtremio

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/opensvc/om3/v3/core/array"
	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/util/san"
	"github.com/opensvc/om3/v3/util/sizeconv"
)

type (
	// Array is one XtremIO array, as the node or cluster configuration
	// declares it.
	Array struct {
		array.Array
		client *http.Client
	}

	// OptAddDisk is what "add disk" was asked for.
	OptAddDisk struct {
		Name              string
		Size              string
		Blocksize         int
		Tags              []string
		AlignmentOffset   int
		SmallIOAlerts     string
		UnalignedIOAlerts string
		VAAITPAlerts      string
		Access            string
		Mappings          []string
		LUN               int
	}

	// OptAddMap is what "add map" was asked for.
	OptAddMap struct {
		Volume         string
		Mappings       []string
		InitiatorGroup string
		TargetGroup    string
		LUN            int
	}

	// OptDelMap is what "del map" was asked for.
	OptDelMap struct {
		Mapping        string
		Volume         string
		InitiatorGroup string
		TargetGroup    string
	}
)

// defaultTimeout is how long the array is given to answer.
const defaultTimeout = 2 * time.Minute

func init() {
	driver.Register(driver.NewID(driver.GroupArray, "xtremio"), NewDriver)
}

// NewDriver returns an XtremIO array driver.
func NewDriver() array.Driver {
	t := New()
	var i any = t
	return i.(array.Driver)
}

// New returns an XtremIO array.
func New() *Array {
	return &Array{}
}

// Run builds the command tree of this array and runs the arguments through it.
// What the tree holds is declared in Actions.
func (t *Array) Run(args []string) error {
	return array.RunActions(context.Background(), t.Actions(), args, os.Stdout)
}

// clusterName is the name the array answers to on its api, which is the "name"
// keyword when it is set, and the name of the section otherwise.
//
// A section is named "array#<something>", and the something is not always what
// the array calls itself: the keyword is how the two are told apart.
func (t Array) clusterName() string {
	if s := t.Config().GetString(t.Key("name")); s != "" {
		return s
	}
	return strings.TrimPrefix(t.Name(), "array#")
}

func (t Array) api() string {
	return strings.TrimSuffix(t.Config().GetString(t.Key("api")), "/")
}

func (t Array) username() string {
	return t.Config().GetString(t.Key("username"))
}

func (t Array) password() (string, error) {
	return t.Config().GetStringStrict(t.Key("password"))
}

// httpClient returns the client the array is talked to through.
//
// The certificate is not verified, as v2 does not verify it: an XtremIO is
// installed with a self signed certificate and the agent has no way of being
// told which one to trust.
func (t *Array) httpClient() *http.Client {
	if t.client == nil {
		t.client = &http.Client{
			Timeout: defaultTimeout,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		}
	}
	return t.client
}

// do runs one api call and returns what the array answered.
func (t *Array) do(ctx context.Context, method, path string, params map[string]string, data map[string]any) ([]byte, int, error) {
	api := t.api()
	if api == "" {
		return nil, 0, fmt.Errorf("%s: the api keyword is required", t.Name())
	}
	password, err := t.password()
	if err != nil {
		return nil, 0, fmt.Errorf("%s: %w", t.Name(), err)
	}

	u := path
	if !strings.HasPrefix(u, "http") {
		u = api + path
	}
	parsed, err := url.Parse(u)
	if err != nil {
		return nil, 0, err
	}
	query := parsed.Query()
	for k, v := range params {
		query.Set(k, v)
	}
	parsed.RawQuery = query.Encode()

	var body io.Reader
	if data != nil {
		b, err := json.Marshal(convertIDs(data))
		if err != nil {
			return nil, 0, err
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, parsed.String(), body)
	if err != nil {
		return nil, 0, err
	}
	req.SetBasicAuth(t.username(), password)
	req.Header.Set("Cache-Control", "no-cache")
	if data != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := t.httpClient().Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return b, resp.StatusCode, nil
}

// get reads a resource of the array.
//
// Every read is scoped to the cluster this array is, as v2 scopes it: an api
// endpoint may serve several clusters, and a read that named none would answer
// about whichever one it chose.
func (t *Array) get(ctx context.Context, path string, params map[string]string, out any) error {
	if params == nil {
		params = map[string]string{}
	}
	params["cluster-name"] = t.clusterName()
	b, code, err := t.do(ctx, http.MethodGet, path, params, nil)
	if err != nil {
		return err
	}
	if code >= 400 {
		return fmt.Errorf("GET %s: %s: %s", path, http.StatusText(code), string(b))
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(b, out)
}

// post creates a resource and returns what the array made of it.
//
// The array answers a creation with the location of what it created, and v2
// reads it back from there, so the caller is handed the object rather than the
// link to it.
func (t *Array) post(ctx context.Context, path string, data map[string]any, out any) error {
	if data == nil {
		data = map[string]any{}
	}
	data["cluster-id"] = t.clusterName()
	b, code, err := t.do(ctx, http.MethodPost, path, nil, data)
	if err != nil {
		return err
	}
	if code != http.StatusCreated {
		return fmt.Errorf("POST %s: %s: %s", path, http.StatusText(code), string(b))
	}
	if out == nil {
		return nil
	}
	var created struct {
		Links []struct {
			Href string `json:"href"`
		} `json:"links"`
	}
	if err := json.Unmarshal(b, &created); err != nil {
		return err
	}
	if len(created.Links) == 0 {
		return fmt.Errorf("POST %s: the array named nothing it created", path)
	}
	return t.get(ctx, created.Links[0].Href, nil, out)
}

// put changes a resource of the array.
func (t *Array) put(ctx context.Context, path string, params map[string]string, data map[string]any) error {
	if data == nil {
		data = map[string]any{}
	}
	data["cluster-id"] = t.clusterName()
	b, code, err := t.do(ctx, http.MethodPut, path, params, data)
	if err != nil {
		return err
	}
	if code != http.StatusOK {
		return fmt.Errorf("PUT %s: %s: %s", path, http.StatusText(code), string(b))
	}
	return nil
}

// del removes a resource of the array.
func (t *Array) del(ctx context.Context, path string, params map[string]string) error {
	if params == nil {
		params = map[string]string{}
	}
	params["cluster-name"] = t.clusterName()
	b, code, err := t.do(ctx, http.MethodDelete, path, params, nil)
	if err != nil {
		return err
	}
	if code != http.StatusOK {
		return fmt.Errorf("DELETE %s: %s: %s", path, http.StatusText(code), string(b))
	}
	return nil
}

// convertIDs turns the id fields of a request body into the numbers the array
// expects, leaving the ones that name rather than number alone.
//
// The array takes "vol-id" and "ig-id" as either a number or a name, and
// answers a name sent as a string with an error, so v2 converts what converts
// and this does the same.
func convertIDs(data map[string]any) map[string]any {
	out := make(map[string]any, len(data))
	for k, v := range data {
		if s, ok := v.(string); ok && strings.HasSuffix(k, "-id") {
			if i, err := strconv.Atoi(s); err == nil {
				out[k] = i
				continue
			}
		}
		out[k] = v
	}
	return out
}

// volumePath returns the path and parameters naming one volume.
//
// A volume is named by its index or by its name, and the array reads the two
// differently: an index is a path element, a name is a parameter.
func volumePath(volume string) (string, map[string]string) {
	if _, err := strconv.Atoi(volume); err == nil {
		return "/volumes/" + volume, map[string]string{}
	}
	return "/volumes", map[string]string{"name": volume}
}

// sizeMB renders a size expression the way the array reads one.
func sizeMB(size string) (string, error) {
	b, err := sizeconv.FromSize(size)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%dM", b/(1024*1024)), nil
}

// initiatorGroupOf returns the initiator group an hba belongs to.
func (t *Array) initiatorGroupOf(ctx context.Context, hbaID string) (string, error) {
	var data struct {
		Initiators []struct {
			IGID []any `json:"ig-id"`
		} `json:"initiators"`
	}
	params := map[string]string{"full": "1", "filter": "port-address:eq:" + convertHBAID(hbaID)}
	if err := t.get(ctx, "/initiators", params, &data); err != nil {
		return "", err
	}
	if len(data.Initiators) == 0 {
		return "", fmt.Errorf("no initiator found with port-address=%s", hbaID)
	}
	ig := data.Initiators[0].IGID
	if len(ig) == 0 {
		return "", fmt.Errorf("initiator %s found in no initiatorgroup", hbaID)
	}
	return fmt.Sprint(ig[len(ig)-1]), nil
}

// targetGroupOf returns the target group a target port belongs to.
func (t *Array) targetGroupOf(ctx context.Context, target string) (string, error) {
	var data struct {
		Targets []struct {
			TGID []any `json:"tg-id"`
		} `json:"targets"`
	}
	params := map[string]string{"full": "1", "filter": "port-address:eq:" + convertHBAID(target)}
	if err := t.get(ctx, "/targets", params, &data); err != nil {
		return "", err
	}
	if len(data.Targets) == 0 {
		return "", fmt.Errorf("no target found with port-address=%s", target)
	}
	tg := data.Targets[0].TGID
	if len(tg) == 0 {
		return "", fmt.Errorf("target %s found in no targetgroup", target)
	}
	return fmt.Sprint(tg[len(tg)-1]), nil
}

// convertHBAID renders a port name the way the array stores one: a wwn is
// stored with a colon between each byte.
//
// An iqn is left alone. v2 puts every port name through this, iqn included,
// which turns an iqn into something no initiator answers to, so an iSCSI
// mapping never matched there either.
func convertHBAID(s string) string {
	if strings.HasPrefix(s, "iqn.") {
		return s
	}
	if len(s) != 16 {
		return s
	}
	l := make([]string, 0, 8)
	for i := 0; i < 16; i += 2 {
		l = append(l, s[i:i+2])
	}
	return strings.Join(l, ":")
}

// sizeBytes reads a size expression.
func sizeBytes(size string) (int64, error) {
	return sizeconv.FromSize(size)
}

// parseMappings reads the mappings of a command line, in the grammar the
// collector writes them.
func parseMappings(l []string) (san.Paths, error) {
	return san.ParseMappings(l)
}

// sortedPathKeys returns the keys of a map, in a stable order, so two runs of
// one command make the same mappings in the same order.
func sortedPathKeys(m map[string][]san.Path) []string {
	l := make([]string, 0, len(m))
	for k := range m {
		l = append(l, k)
	}
	sort.Strings(l)
	return l
}

// errMappingRequired is what "del map" says when it is told nothing to remove.
var errMappingRequired = errors.New("--mapping is mandatory, or --volume to remove every mapping of a volume")
