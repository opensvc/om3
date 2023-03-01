/*
Package callcount provides call count watcher

Example:

	import "callcount"
	mapping := map[int]string{
		1: "operation 1",
		2: "operation 2",
		3: "operation 3",
	}
	c, cancel := callcount.Start(context.Background(), mapping)
	defer cancel() stop the counter
	c <- 1 // register func with id 1 as been called
	c <- 2 // register func with id 2 as been called
	c <- 1 // register func with id 1 as been called
	c <- 8 // register func with id 8 has been called (undefined in mapping)

	Get(c) // return Counts{1: 2, 2:1, 8:1}
	GetStats(c) // return Stats{"operation 1": 1, "operation 2":1, "unknown":1}
	Reset(c) // resets current counters
*/
package callcount

import (
	"context"
	"time"
)

type (
	Counts map[int]uint64
	Stats  map[string]uint64

	getCount   chan Counts
	getStats   chan Stats
	resetCount struct{}
)

/*
Start launch new go routine that watch call counts

It returns command control channel, and stop function to stop the counter
*/
func Start(parent context.Context, mapping map[int]string, drainDuration time.Duration) (chan<- interface{}, context.CancelFunc) {
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithCancel(parent)
	c := make(chan interface{})
	go func() {
		run(ctx, c, mapping)
		tC := time.After(drainDuration)
		for {
			select {
			case <-tC:
				// drop pending timeout reached
				return
			case <-c:
				// drop pending
			}
		}
	}()
	var cmdC chan<- interface{} = c
	return cmdC, cancel
}

// Get return current Counts
func Get(cmd chan<- interface{}) Counts {
	result := make(getCount)
	cmd <- result
	return <-result
}

// GetStats return current Stats
func GetStats(cmd chan<- interface{}) Stats {
	result := make(getStats)
	cmd <- result
	return <-result
}

// Reset resets current counts
func Reset(cmd chan<- interface{}) {
	cmd <- resetCount{}
}

func run(ctx context.Context, c <-chan interface{}, mapping map[int]string) {
	counts := make(Counts)
	for {
		select {
		case <-ctx.Done():
			return
		case op := <-c:
			switch o := op.(type) {
			case int:
				counts[o]++
			case getCount:
				resCounts := make(Counts)
				for i, v := range counts {
					resCounts[i] = v
				}
				o <- resCounts
			case getStats:
				stats := make(Stats)
				var unknownCount uint64
				for id, count := range counts {
					if name, ok := mapping[id]; ok {
						stats[name] = count
					} else {
						unknownCount += count
					}
				}
				if unknownCount > 0 {
					stats["unknown"] = unknownCount
				}
				o <- stats
			case resetCount:
				counts = make(Counts)
			}
		}
	}
}
