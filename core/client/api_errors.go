package client

import (
	"encoding/json"
	"strings"

	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

type (
	schema1 struct {
		Status    interface{} `json:"status"`
		Error     interface{} `json:"error"`
		Info      interface{} `json:"info"`
		Traceback string      `json:"traceback"`
		Data      interface{} `json:"data"`
	}
)

// parse tries to abstract the status code, error and info strings
// parsing in the response body, returning Go-friendly errors.
func parse(b []byte, err error) ([]byte, error) {
	//log.Debug().Err(err).Bytes("b", b).Msg("parse response")
	if err != nil {
		return b, err
	}
	data := &schema1{}
	if err := json.Unmarshal(b, &data); (err != nil) || (data == nil) || (data.Status == nil) {
		// not a schema1 response => let it go as-is
		//log.Debug().Bytes("b", b).Msg("parse")
		return b, nil
	}

	switch data.Info.(type) {
	case string:
		log.Debug().Str("info", data.Info.(string)).Msgf("parse response")
	case []string:
		for _, s := range data.Info.([]string) {
			log.Debug().Str("info", s).Msgf("parse response")
		}
	}

	switch data.Error.(type) {
	case string:
		log.Debug().Str("error", data.Error.(string)).Msgf("parse response")
		err = errors.New(data.Error.(string))
	case []string:
		for _, s := range data.Error.([]string) {
			log.Debug().Str("error", s).Msgf("parse response")
		}
		msg := strings.Join(data.Error.([]string), "\n")
		err = errors.New(msg)
	}

	if data.Traceback != "" {
		log.Error().Str("traceback", data.Traceback).Msg("parse")
		return nil, errors.New("api bug")
	}

	switch data.Status {
	case 0.0, "0":
		switch data.Data {
		case nil:
			log.Debug().Msg("parse: no data")
			return b, nil
		default:
			log.Debug().Msg("parse: schema1")
			return json.Marshal(data.Data)
		}
	default:
		return nil, err
	}
}
