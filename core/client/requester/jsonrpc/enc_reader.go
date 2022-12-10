package reqjsonrpc

import (
	"bytes"
	"context"
	"io"
)

type (
	// streamRead implement decrypted io.ReadCloser from encrypted io.readCloser
	streamRead struct {
		// rc is src io.readCloser or encrypted data
		rc io.ReadCloser

		ctx    context.Context
		cancel context.CancelFunc

		// r is internal reader for 1 decrypted message
		r chan *bytes.Reader

		// errC is internal error chanel of errors
		errC chan error
	}
)

// NewReader returns decrypted io.ReadCloser from encrypted io.ReaderCloser
// TODO convert to io.Reader instead of io.ReadCloser ?
func NewReader(ctx context.Context, r io.ReadCloser) io.ReadCloser {
	ctx, cancel := context.WithCancel(ctx)
	d := streamRead{
		rc:     r,
		ctx:    ctx,
		cancel: cancel,
		r:      make(chan *bytes.Reader),
		errC:   make(chan error, 2),
	}
	encryptedC := make(chan []byte)
	decryptedC := make(chan []byte)

	go func() {
		if err := GetMessages(encryptedC, d.rc); err != nil {
			d.errC <- err
		}
	}()

	go func() {
		if err := decryptChan(encryptedC, decryptedC); err != nil {
			d.errC <- err
		}
	}()

	go func() {
		var r *bytes.Reader
		for {
			select {
			case <-d.ctx.Done():
				return
			case b := <-decryptedC:
				r = bytes.NewReader(b)
				for {
					d.r <- r
					<-d.r
					if r.Len() == 0 {
						r = nil
						break
					}
				}
			}
		}
	}()
	return &d
}

func (d *streamRead) Read(b []byte) (n int, err error) {
	select {
	case err := <-d.errC:
		// returns internal error
		return 0, err
	case <-d.ctx.Done():
		return 0, d.ctx.Err()
	case r := <-d.r:
		defer func() {
			d.r <- r
		}()
		return r.Read(b)
	}
}

func (d *streamRead) Close() error {
	d.cancel()
	return d.rc.Close()
}
