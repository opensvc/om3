package main

import (
	"math/rand"
	"sync"
	"time"

	"github.com/fatih/color"
	"github.com/opensvc/om3/util/progress"
)

func randSleep() {
	duration := time.Millisecond * time.Duration(rand.Int63n(1000))
	time.Sleep(duration)
}

func main() {
	pv := progress.NewView()
	pv.Start()
	defer pv.Stop()

	wg := sync.WaitGroup{}
	wg.Add(2)
	go func() {
		pv.Info("svc1 ip#1", "pre start")
		randSleep()
		pv.Info("svc1 ip#1", "start")
		randSleep()
		pv.Info("svc1 ip#1", "post start")
		randSleep()
		pv.Info("svc1 ip#1", color.GreenString("up"))
		randSleep()
		pv.Info("svc1 fs#1", "pre start")
		randSleep()
		pv.Info("svc1 fs#1", "start")
		randSleep()
		pv.Info("svc1 fs#1", "post start")
		randSleep()
		pv.Info("svc1 fs#1", color.GreenString("up"))
		wg.Done()
	}()
	go func() {
		pv.Info("svc2 ip#1", "pre start")
		randSleep()
		pv.Info("svc2 ip#1", "start")
		randSleep()
		pv.Info("svc2 ip#1", color.RedString("start failed: a useful error"))
		wg.Done()
	}()
	wg.Wait()
}
