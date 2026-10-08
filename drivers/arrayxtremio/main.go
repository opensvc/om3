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
	"math"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/opensvc/om3/v3/core/array"
	"github.com/opensvc/om3/v3/core/datarecv"
	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/util/san"
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

// password returns the password the api is logged in with.
//
// The keyword names the sec holding the password rather than holding it, as
// v2 reads it: "from system/sec/xio key password", or the older
// "system/sec/xio" whose key is "password". A value naming no sec is refused
// rather than sent as the password, so a password written in clear in the
// configuration is not used, as v2 does not use it.
func (t Array) password() (string, error) {
	s, err := t.Config().GetStringStrict(t.Key("password"))
	if err != nil {
		return "", err
	}
	// The value is not quoted in the errors: when it names no sec, it may be
	// the password itself, and an error is logged.
	km, err := datarecv.ParseKeyMetaRelWithFallback(s, naming.NsSys, "password")
	if err != nil || km.Path.Kind != naming.KindSec {
		return "", fmt.Errorf("the password keyword does not name a sec key, as \"from system/sec/<name> key password\" does")
	}
	b, err := km.RootDecode()
	if err != nil {
		return "", fmt.Errorf("the password keyword names %s key %s: %w", km.Path, km.Key, err)
	}
	return string(b), nil
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
	if strings.HasPrefix(path, "http") {
		// A link the array answered with is followed only to the array:
		// the request carries the password, which another host, or the
		// array over plain http, must not be sent.
		base, err := url.Parse(api)
		if err != nil {
			return nil, 0, fmt.Errorf("%s: api %s: %w", t.Name(), api, err)
		}
		if parsed.Scheme != base.Scheme || parsed.Host != base.Host {
			return nil, 0, fmt.Errorf("%s: refuse to follow the link %s out of the api %s", t.Name(), path, api)
		}
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
		return nil, 0, fmt.Errorf("%w: %s %s: %w", errNoAnswer, method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("%w: %s %s: %w", errNoAnswer, method, path, err)
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

// create creates a resource and returns the link to what the array made.
//
// The array answers a creation with the location of what it created. The
// caller reads it from there, and knows from an error wrapping errNoAnswer
// that the creation may have been done all the same.
func (t *Array) create(ctx context.Context, path string, data map[string]any) (string, error) {
	if data == nil {
		data = map[string]any{}
	}
	data["cluster-id"] = t.clusterName()
	b, code, err := t.do(ctx, http.MethodPost, path, nil, data)
	if err != nil {
		return "", err
	}
	if code != http.StatusCreated {
		return "", fmt.Errorf("POST %s: %s: %s", path, http.StatusText(code), string(b))
	}
	var created struct {
		Links []struct {
			Href string `json:"href"`
		} `json:"links"`
	}
	if err := json.Unmarshal(b, &created); err != nil {
		return "", fmt.Errorf("POST %s: the array answered the creation with %q: %w", path, string(b), err)
	}
	if len(created.Links) == 0 || created.Links[0].Href == "" {
		return "", fmt.Errorf("POST %s: the array named nothing it created", path)
	}
	return created.Links[0].Href, nil
}

// post creates a resource and returns what the array made of it.
//
// v2 reads what was made back from the link the array answers with, so the
// caller is handed the object rather than the link to it.
func (t *Array) post(ctx context.Context, path string, data map[string]any, out any) error {
	href, err := t.create(ctx, path, data)
	if err != nil {
		return err
	}
	return t.get(ctx, href, nil, out)
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
		if k == "cluster-id" {
			// The cluster is named, as the reads name it with
			// cluster-name: a cluster named "2" sent as the number 2 is the
			// cluster of index 2, another one on an XMS serving several.
			out[k] = v
			continue
		}
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

// mib returns the size the array is told a volume has, in megabytes, as v2
// tells it.
//
// v2 dropped what is below a megabyte, which makes a volume smaller than
// asked. A size that is not a whole number of megabytes is refused rather
// than rounded either way.
func mib(b int64) (string, error) {
	if b <= 0 || b%(1024*1024) != 0 {
		return "", fmt.Errorf("a volume size of %d bytes is not a whole number of megabytes, the unit the array is told a size in", b)
	}
	return fmt.Sprintf("%dM", b/(1024*1024)), nil
}

// parseSize reads the --size of an action, in the units v2 and the collector
// read it in.
func parseSize(s string) (array.Size, error) {
	if s == "" {
		return array.Size{}, fmt.Errorf("--size is mandatory")
	}
	size, err := array.ParseSize(s)
	if err != nil {
		return size, fmt.Errorf("--size: %w", err)
	}
	if size.Bytes%(1024*1024) != 0 {
		return size, fmt.Errorf("--size %s is not a whole number of megabytes, the unit the array is told a size in", s)
	}
	return size, nil
}

// groupOf returns the index of the group the port of an initiator or of a
// target belongs to.
//
// The array is asked for the port by a filter, and the answer is checked
// rather than trusted: a filter the array did not apply would hand back the
// first port it has, and the volume would be exported to a host that did not
// ask for it.
//
// The group is named by the last element of its id, which is the index, as v2
// names it. The index is what the mapping is made with, so a group whose name
// is a number is not mistaken for another one.
func (t *Array) groupOf(ctx context.Context, resource, groupKey, port string) (int, error) {
	address := convertHBAID(port)
	var data map[string]json.RawMessage
	params := map[string]string{"full": "1", "filter": "port-address:eq:" + address}
	if err := t.get(ctx, "/"+resource, params, &data); err != nil {
		return 0, err
	}
	var list []map[string]any
	if err := json.Unmarshal(data[resource], &list); err != nil {
		return 0, fmt.Errorf("GET /%s: the answer holds no %s list: %w", resource, resource, err)
	}
	matching := make([]map[string]any, 0, 1)
	for _, one := range list {
		if s, ok := one["port-address"].(string); ok && strings.EqualFold(s, address) {
			matching = append(matching, one)
		}
	}
	switch len(matching) {
	case 0:
		return 0, fmt.Errorf("no %s found with port-address=%s", strings.TrimSuffix(resource, "s"), address)
	case 1:
	default:
		return 0, fmt.Errorf("%d %s found with port-address=%s", len(matching), resource, address)
	}
	id, ok := matching[0][groupKey].([]any)
	if !ok || len(id) == 0 {
		return 0, fmt.Errorf("%s %s found in no group: its %s is %v", strings.TrimSuffix(resource, "s"), address, groupKey, matching[0][groupKey])
	}
	index, err := indexOf(id[len(id)-1])
	if err != nil {
		return 0, fmt.Errorf("%s %s: %s %v: %w", strings.TrimSuffix(resource, "s"), address, groupKey, id, err)
	}
	return index, nil
}

// initiatorGroupOf returns the index of the initiator group an hba belongs to.
func (t *Array) initiatorGroupOf(ctx context.Context, hbaID string) (int, error) {
	return t.groupOf(ctx, "initiators", "ig-id", hbaID)
}

// targetGroupOf returns the index of the target group a target port belongs
// to.
func (t *Array) targetGroupOf(ctx context.Context, target string) (int, error) {
	return t.groupOf(ctx, "targets", "tg-id", target)
}

// indexOf reads an object index of an answer of the array, which a json
// decoder hands over as a float.
func indexOf(v any) (int, error) {
	f, ok := v.(float64)
	if !ok || f < 0 || f != math.Trunc(f) || f > math.MaxInt32 {
		return 0, fmt.Errorf("%v is not an object index", v)
	}
	return int(f), nil
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

// parseMappings reads the mappings of a command line, in the grammar the
// collector writes them.
func parseMappings(l []string) (san.Paths, error) {
	return san.ParseMappings(l)
}

var (
	// errMappingRequired is what "del map" says when it is told nothing to
	// remove.
	errMappingRequired = errors.New("--mapping is mandatory, or --volume to remove every mapping of a volume")

	// errNoAnswer is a request sent and not answered. A creation or a removal
	// may have been done all the same.
	errNoAnswer = errors.New("no answer from the array")
)
