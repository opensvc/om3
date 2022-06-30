package create

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/iancoleman/orderedmap"
	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/core/clientcontext"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/core/rawconfig"
	"opensvc.com/opensvc/core/xconfig"
	"opensvc.com/opensvc/util/file"
	"opensvc.com/opensvc/util/funcopt"
	"opensvc.com/opensvc/util/uri"
)

type (
	T struct {
		client    *client.T
		path      path.T
		namespace string
		config    string
		template  string
		keywords  []string
		restore   bool
	}
	Pivot map[string]rawconfig.T
)

//
// WithPath sets the path string representation of the single object to create.
// If multiple objects are to be created, use WithNamespace() instead.
//
func WithPath(p path.T) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.path = p
		return nil
	})
}

//
// WithConfig sets the location of the configuration file of the single object to create.
// The value can be a URL or a local file path, or /dev/stdin.
// If multiple objects are to be created, set to /dev/stdin and feed a json map indexed
// by object path.
//
func WithConfig(s string) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.config = s
		return nil
	})
}

//
// WithNamespace sets the name of the namespace where to create the new objects.
// If a path is given via WithPath(), the namespace part of the path is overridden
// by this namespace parameter.
//
func WithNamespace(s string) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.namespace = s
		return nil
	})
}

func WithTemplate(s string) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.template = s
		return nil
	})
}

func WithKeywords(s []string) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.keywords = s
		return nil
	})
}

func WithClient(c *client.T) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.client = c
		return nil
	})
}

func WithRestore(v bool) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.restore = v
		return nil
	})
}

func New(opts ...funcopt.O) (*T, error) {
	t := &T{}
	if err := funcopt.Apply(t, opts...); err != nil {
		return nil, err
	}
	return t, nil
}

func (t T) Do() error {
	switch {
	case t.template != "" && t.config != "":
		return fmt.Errorf("--config and --template are conflicting")
	case t.template != "":
		return t.fromTemplate()
	case t.config == "":
		return t.fromScratch()
	case t.config == "-" || t.config == "/dev/stdin" || t.config == "stdin":
		return t.fromStdin()
	case t.config != "":
		return t.fromConfig()
	default:
		return fmt.Errorf("don't know what to do")
	}
}

func (t *T) submit(pivot Pivot) error {
	data := make(map[string]interface{})
	for opath, c := range pivot {
		data[opath] = c
	}
	req := t.client.NewPostObjectCreate()
	req.Restore = t.restore
	req.Data = data
	if _, err := req.Do(); err != nil {
		return err
	}
	return nil
}

func (t T) fromTemplate() error {
	if pivot, err := t.rawFromTemplate(); err != nil {
		return err
	} else {
		return t.fromData(pivot)
	}
}

func (t T) fromConfig() error {
	if pivot, err := t.rawFromConfig(); err != nil {
		return err
	} else {
		return t.fromData(pivot)
	}
}

func (t T) fromScratch() error {
	if pivot, err := rawFromScratch(t.path); err != nil {
		return err
	} else {
		return t.fromData(pivot)
	}
}

func (t T) fromStdin() error {
	var (
		pivot Pivot
		err   error
	)
	if t.path.IsZero() {
		pivot, err = rawFromStdinNested(t.namespace)
	} else {
		pivot, err = rawFromStdinFlat(t.path)
	}
	if err != nil {
		return err
	} else {
		return t.fromData(pivot)
	}
}

func (t T) fromData(pivot Pivot) error {
	// TODO: kws
	if clientcontext.IsSet() {
		return t.submit(pivot)
	}
	return localFromData(pivot)
}

func (t T) rawFromTemplate() (Pivot, error) {
	return nil, fmt.Errorf("TODO: collector requester")
}

func (t T) rawFromConfig() (Pivot, error) {
	u := uri.New(t.config)
	switch {
	case file.Exists(t.config):
		return rawFromConfigFile(t.path, t.config)
	case u.IsValid():
		return rawFromConfigURI(t.path, u)
	default:
		return nil, fmt.Errorf("invalid configuration: %s is not a file, nor an uri", t.config)
	}
}

func rawFromConfigURI(p path.T, u uri.T) (Pivot, error) {
	fpath, err := u.Fetch()
	if err != nil {
		return make(Pivot), nil
	}
	defer os.Remove(fpath)
	fmt.Print("fetched... ")
	return rawFromConfigFile(p, fpath)
}

func rawFromConfigFile(p path.T, fpath string) (Pivot, error) {
	pivot := make(Pivot)
	c, err := xconfig.NewObject(fpath)
	if err != nil {
		return pivot, err
	}
	pivot[p.String()] = c.Raw()
	fmt.Print("parsed... ")
	return pivot, nil
}

func rawFromScratch(p path.T) (Pivot, error) {
	pivot := make(Pivot)
	pivot[p.String()] = rawconfig.T{}
	return pivot, nil
}

func rawFromStdinNested(namespace string) (Pivot, error) {
	pivot := make(Pivot)
	b, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		return pivot, err
	}
	if err = json.Unmarshal(b, &pivot); err != nil {
		return pivot, err
	}
	if md, ok := pivot["metadata"]; ok {
		p, err := pathFromMetadata(md.Data)
		if err != nil {
			return pivot, err
		}
		if namespace != "" {
			p.Namespace = namespace
		}
		return rawFromBytesFlat(p, b)
	}
	return pivot, nil
}

func pathFromMetadata(data *orderedmap.OrderedMap) (path.T, error) {
	var name, namespace, kind string
	if s, ok := data.Get("name"); ok {
		if name, ok = s.(string); !ok {
			return path.T{}, fmt.Errorf("metadata format error: name")
		}
	}
	if s, ok := data.Get("kind"); ok {
		if kind, ok = s.(string); !ok {
			return path.T{}, fmt.Errorf("metadata format error: kind")
		}
	}
	if s, ok := data.Get("namespace"); ok {
		switch k := s.(type) {
		case nil:
			namespace = ""
		case string:
			namespace = k
		default:
			return path.T{}, fmt.Errorf("metadata format error: namespace")
		}
	}
	return path.New(name, namespace, kind)
}

func rawFromStdinFlat(p path.T) (Pivot, error) {
	b, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		return nil, err
	}
	return rawFromBytesFlat(p, b)
}

func rawFromBytesFlat(p path.T, b []byte) (Pivot, error) {
	pivot := make(Pivot)
	c := &rawconfig.T{}
	if err := json.Unmarshal(b, c); err != nil {
		return pivot, err
	}
	pivot[p.String()] = *c
	return pivot, nil
}

func localFromData(pivot Pivot) error {
	for opath, c := range pivot {
		p, err := path.Parse(opath)
		if err != nil {
			return err
		}
		if err = localFromRaw(p, c); err != nil {
			return err
		}
		fmt.Println(opath, "commited")
	}
	return nil
}

func localFromRaw(p path.T, c rawconfig.T) error {
	o, err := object.New(p)
	if err != nil {
		return err
	}
	oc := o.(object.Configurer)
	return oc.Config().CommitData(c)
}

func LocalEmpty(p path.T) error {
	o, err := object.New(p)
	if err != nil {
		return err
	}
	oc := o.(object.Configurer)
	return oc.Config().Commit()
}

func setKeywords(oc object.Configurer, kws []string) error {
	return oc.Set(object.OptsSet{
		KeywordOps: kws,
	})
}
