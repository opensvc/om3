package apimodel

type (
	// BaseResponseMux can be used to compose multiplexed response
	// composed multiplex response will have to define Data
	BaseResponseMux struct {
		EntryPoint string `json:"entrypoint" yaml:"entrypoint"`
		Status     int    `json:"status" yaml:"status"`
		Error      string `json:"error,omitempty" yaml:"error,omitempty"`
		//Data []BaseResponseMuxData `json:"data,omitempty" yaml:"data,omitempty"`
	}

	// BaseResponseMuxData can be used to define composed multiplexed response
	// Data member
	BaseResponseMuxData struct {
		Endpoint string `json:"endpoint" yaml:"endpoint"`
		Error    string `json:"error,omitempty" yaml:"error,omitempty"`
		//Data []XXX `json:"data,omitempty" yaml:"data,omitempty"`
	}
)
