package moncmd

import (
	"context"
	"time"
)

func DropPendingCmd(c <-chan *T, duration time.Duration) {
	dropping := make(chan bool)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), duration)
		defer cancel()
		dropping <- true
		for {
			select {
			case <-c:
			case <-ctx.Done():
				return
			}
		}
	}()
	<-dropping
}
