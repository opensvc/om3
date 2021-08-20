package object

import "runtime"

type (
	// OptsNodePushAsset is the options of the PushAsset function.
	OptsNodePushAsset struct {
		Global OptsGlobal
	}

	AssetValue struct {
		Source string      `json:"source"`
		Title  string      `json:"title"`
		Value  interface{} `json:"value"`
	}

	AssetData []AssetValue
)

const (
	AssetSrcProbe   string = "probe"
	AssetSrcDefault string = "default"
	AssetSrcConfig  string = "config"
)

// PushAsset find and runs the check drivers.
func (t Node) PushAsset() AssetData {
	data := make(AssetData, 0)
	data = append(data, AssetValue{
		Source: AssetSrcProbe,
		Title:  "cpu_threads",
		Value:  runtime.NumCPU(),
	})
	return data
}
