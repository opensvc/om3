// +build !linux

package network

import "github.com/rs/zerolog"

type (
	logger interface {
		Log() *zerolog.Logger
	}
)

func setupFW(_ logger, _ []Networker) error {
	return nil
}
