//go:build !linux

package network

import "github.com/rs/zerolog"

type (
	logger interface {
		Log() *plog.Logger
	}
)

func setupFW(_ logger, _ []Networker) error {
	return nil
}
