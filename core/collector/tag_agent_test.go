package collector

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/opensvc/om3/v3/core/naming"
)

func TestTagClient(t *testing.T) {
	type call struct{ method, uri, body string }
	var got []call
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		got = append(got, call{r.Method, r.URL.RequestURI(), strings.TrimSpace(string(b))})
		switch {
		case r.URL.Path == "/api/agent/tags" && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode([]string{"a", "b"})
		case r.URL.Path == "/api/agent/tags/attached":
			_ = json.NewEncoder(w).Encode(TagAttachmentList{{TagName: "a", TagData: "d", TagAttachData: "x"}})
		case r.URL.Path == "/api/agent/tags/missing":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"text":"tag missing does not exist: create it first"}`))
		default:
			_ = json.NewEncoder(w).Encode(map[string]string{"info": "done"})
		}
	}))
	defer srv.Close()

	cli, err := NewRequester(srv.URL, "", false)
	if err != nil {
		t.Fatal(err)
	}
	c := NewTagClient(cli)
	p := naming.Path{Namespace: "ns1", Kind: naming.KindSvc, Name: "s1"}
	data := "x"

	if l, err := c.Names(); err != nil || len(l) != 2 {
		t.Fatalf("Names: %v %v", l, err)
	}
	if l, err := c.Attached(p); err != nil || len(l) != 1 || l[0].TagAttachData != "x" {
		t.Fatalf("Attached: %v %v", l, err)
	}
	if info, err := c.Create("a b", &data, nil); err != nil || info != "done" {
		t.Fatalf("Create: %q %v", info, err)
	}
	if _, err := c.Attach("a b", naming.Path{}, &data); err != nil {
		t.Fatalf("Attach: %v", err)
	}
	if _, err := c.Detach("a", p); err != nil {
		t.Fatalf("Detach: %v", err)
	}
	_, err = c.Attach("missing", p, nil)
	if err == nil || !strings.Contains(err.Error(), "[404] tag missing does not exist: create it first") {
		t.Fatalf("Attach missing: %v", err)
	}

	want := []call{
		{"GET", "/api/agent/tags", ""},
		{"GET", "/api/agent/tags/attached?path=ns1%2Fsvc%2Fs1", ""},
		{"POST", "/api/agent/tags", `{"name":"a b","data":"x"}`},
		{"POST", "/api/agent/tags/a%20b", `{"attach_data":"x"}`},
		{"DELETE", "/api/agent/tags/a?path=ns1%2Fsvc%2Fs1", ""},
		{"POST", "/api/agent/tags/missing?path=ns1%2Fsvc%2Fs1", "{}"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d calls, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("call %d: got %+v, want %+v", i, got[i], want[i])
		}
	}
}
