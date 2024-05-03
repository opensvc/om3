package arrayfreenas

// SystemInfo defines model for GetSystemInfo
type SystemInfo struct {
	Version              string     `json:"version"`                // "TrueNAS-13.0-U2"
	Hostname             string     `json:"hostname"`               //  "truenas.vdc.opensvc.com"
	PhysMem              uint64     `json:"physmem"`                // 4241022976
	Model                string     `json:"model"`                  // "Intel(R) Core(TM) i7-10710U CPU @ 1.10GHz"
	Cores                uint       `json:"cores"`                  // 2
	Uptime               string     `json:"uptime"`                 // "4 days, 4:59:17.134670"
	UptimeSeconds        float64    `json:"uptime_seconds"`         // 363557.134669586
	SystemSerial         string     `json:"system_serial"`          // "0",
	SystemProduct        string     `json:"system_product"`         // "VirtualBox"
	SystemProductVersion string     `json:"system_product_version"` // "1.2"
	Timezone             string     `json:"timezone"`               // "Europe/Paris"
	SystemManufacturer   string     `json:"system_manufacturer"`    // "innotek GmbH"
	LoadAvg              [3]float64 `json:"loadavg"`                // [0.32470703125, 0.39111328125, 0.3564453125]
	//  "buildtime": {
	//   "$date": 1661831610000
	//  },
	//  "license": null,
	//  "boottime": {
	//   "$date": 1663040814000
	//  },
	//  "datetime": {
	//   "$date": 1663404371500
	//  },
	//  "birthday": {
	//   "$date": 1658217925615
	//  },
	//  "ecc_memory": null

}
