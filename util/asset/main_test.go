package asset

import (
	"encoding/json"
	"testing"
)

func TestPropertiesFeedMap(t *testing.T) {
	var p Properties
	p.Nodename = Property{Source: SrcProbe, Value: "n1"}
	p.ConnectTo = Property{Source: SrcDefault, Value: ""}
	p.SecZone = Property{Source: SrcDefault}
	p.LocCity = Property{Source: SrcConfig, Value: "Paris"}
	p.Enclosure = Property{Source: SrcDefault}

	m, err := p.FeedMap()
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"connect_to", "sec_zone", "asset_env", "team_integ", "team_support", "loc_zip"} {
		if _, ok := m[k]; ok {
			t.Errorf("unset optional property %s must be omitted", k)
		}
	}
	if v := m["loc_city"].Value; v != "Paris" {
		t.Errorf("loc_city: expected Paris, got %v", v)
	}
	if v := m["nodename"].Value; v != "n1" {
		t.Errorf("nodename: expected n1, got %v", v)
	}
	// non optional properties are kept even when defaulted
	if _, ok := m["enclosure"]; !ok {
		t.Errorf("enclosure must be kept")
	}
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	var back map[string]map[string]any
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if back["loc_city"]["source"] != SrcConfig {
		t.Errorf("unexpected encoding: %s", b)
	}
}
