package resfszfs

import (
	"reflect"
	"testing"
)

const (
	g10 = int64(10) << 30
	g20 = int64(20) << 30
	g30 = int64(30) << 30
	g60 = int64(60) << 30
)

// The size of a dataset is its refquota, or its quota when it has none, and
// a property configured as a multiplier of the size says the size divided by
// it. A dataset with neither has no size.
func TestDatasetSize(t *testing.T) {
	for _, tc := range []struct {
		name   string
		cur    map[string]int64
		exprs  map[string]string
		size   int64
		anchor string
	}{
		{"refquota", map[string]int64{"refquota": g10}, map[string]string{"refquota": "x1"}, g10, "refquota"},
		{"refquota over quota", map[string]int64{"refquota": g10, "quota": g20}, map[string]string{"refquota": "x1", "quota": "x2"}, g10, "refquota"},
		{"quota only", map[string]int64{"quota": g10}, map[string]string{"refquota": "none", "quota": "x1"}, g10, "quota"},
		{"quota only, doubled", map[string]int64{"quota": g20}, map[string]string{"refquota": "none", "quota": "x2"}, g10, "quota"},
		{"quota only, a size of its own", map[string]int64{"quota": g20}, map[string]string{"quota": "20g"}, g20, "quota"},
	} {
		size, anchor, err := datasetSize(tc.cur, tc.exprs)
		if err != nil || size != tc.size || anchor != tc.anchor {
			t.Errorf("%s: size %d, anchor %s, err %v", tc.name, size, anchor, err)
		}
	}
	if _, _, err := datasetSize(map[string]int64{"reservation": g10}, nil); err == nil {
		t.Error("a dataset with no refquota and no quota has a size")
	}
}

// A resize moves the properties bounding the dataset together: a multiplier
// is set again from the new size, a property that was the size moves with
// it, a property with a size of its own is left alone, and one not set is
// not set.
func TestResizedProperties(t *testing.T) {
	for _, tc := range []struct {
		name  string
		cur   map[string]int64
		exprs map[string]string
		to    int64
		want  map[string]int64
	}{
		{
			"the quota capping the dataset and its descendants at the size moves with it",
			map[string]int64{"refquota": g10, "quota": g10},
			map[string]string{"refquota": "x1", "quota": "x1"},
			g30,
			map[string]int64{"refquota": g30, "quota": g30},
		},
		{
			"a quota twice the size stays twice the size",
			map[string]int64{"refquota": g10, "quota": g20},
			map[string]string{"refquota": "x1", "quota": "x2"},
			g30,
			map[string]int64{"refquota": g30, "quota": g60},
		},
		{
			"a quota equal to the size moves with it, with no keyword saying so",
			map[string]int64{"refquota": g10, "quota": g10},
			map[string]string{},
			g30,
			map[string]int64{"refquota": g30, "quota": g30},
		},
		{
			"a quota with a size of its own, over the size asked, is left alone",
			map[string]int64{"refquota": g10, "quota": g60},
			map[string]string{"refquota": "x1", "quota": "60g"},
			g30,
			map[string]int64{"refquota": g30},
		},
		{
			"a dataset with a quota only is resized by its quota",
			map[string]int64{"quota": g10},
			map[string]string{"refquota": "none", "quota": "x1"},
			g30,
			map[string]int64{"quota": g30},
		},
		{
			"a refreservation guaranteeing the whole size moves with it, one guaranteeing part of it does not",
			map[string]int64{"refquota": g20, "refreservation": g20, "reservation": g10},
			map[string]string{"refquota": "x1"},
			g30,
			map[string]int64{"refquota": g30, "refreservation": g30},
		},
		{
			"a property not set on the dataset is not set by a resize",
			map[string]int64{"refquota": g10},
			map[string]string{"refquota": "x1", "quota": "x2", "reservation": "x1"},
			g30,
			map[string]int64{"refquota": g30},
		},
		{
			"a dataset already at the size has nothing to set",
			map[string]int64{"refquota": g30, "quota": g30},
			map[string]string{"refquota": "x1", "quota": "x1"},
			g30,
			map[string]int64{},
		},
	} {
		got, err := resizedProperties(tc.cur, tc.exprs, tc.to)
		if err != nil {
			t.Errorf("%s: %s", tc.name, err)
			continue
		}
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}

// A quota with a size of its own under the size asked is refused: the
// dataset would be given a size its quota does not let it reach.
func TestResizedPropertiesRefusesAQuotaUnderTheSize(t *testing.T) {
	_, err := resizedProperties(
		map[string]int64{"refquota": g10, "quota": g20},
		map[string]string{"refquota": "x1", "quota": "20g"},
		g30,
	)
	if err == nil {
		t.Fatal("a refquota over the quota is accepted")
	}
	if _, err := resizedProperties(map[string]int64{}, nil, g30); err == nil {
		t.Error("a dataset with no size is resized")
	}
}

// A grow raises the caps before what they cap, and a shrink lowers them in
// the reverse order.
func TestSizePropertiesOrder(t *testing.T) {
	grow := sizePropertiesOrder(true)
	shrink := sizePropertiesOrder(false)
	if grow[0] != "quota" || grow[1] != "refquota" {
		t.Errorf("grow order: %v", grow)
	}
	if shrink[len(shrink)-1] != "quota" || shrink[len(shrink)-2] != "refquota" {
		t.Errorf("shrink order: %v", shrink)
	}
	if !reflect.DeepEqual(sizeProperties, grow) {
		t.Errorf("the order of a grow changed the properties: %v", sizeProperties)
	}
}

// A dataset given a size and no refquota is bounded by the size, as in v2:
// the refquota is x1 unless the keyword says otherwise, and nothing when
// there is no size to bound it by.
func TestRefQuotaDefault(t *testing.T) {
	size := g10
	for _, tc := range []struct {
		name     string
		t        T
		want     int64
		wantNone bool
	}{
		{"size only", T{Size: &size}, g10, false},
		{"size, refquota x2", T{Size: &size, RefQuota: "x2"}, g20, false},
		{"size, refquota none", T{Size: &size, RefQuota: "none"}, 0, true},
		{"no size", T{}, 0, true},
	} {
		v, err := tc.t.refquota()
		switch {
		case err != nil:
			t.Errorf("%s: %s", tc.name, err)
		case tc.wantNone && v != nil:
			t.Errorf("%s: got %d, want none", tc.name, *v)
		case !tc.wantNone && (v == nil || *v != tc.want):
			t.Errorf("%s: got %v, want %d", tc.name, v, tc.want)
		}
	}
}
