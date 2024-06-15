package cpuprofile

import (
	"bytes"
	"context"
	"log"
	"runtime/pprof"
	"sync"
	"time"
)

var globalCPUProfiler = newCPUProfiler()
var profileWindow = time.Second

type ProfileData struct {
	Data  *bytes.Buffer
	Error error
}

type ProfileConsumer = chan *ProfileData

type cpuProfiler struct {
	consumers    map[ProfileConsumer]struct{}
	profileData  *ProfileData
	lastDataSize int
	sync.Mutex
}

func newCPUProfiler() *cpuProfiler {
	return &cpuProfiler{
		consumers: make(map[ProfileConsumer]struct{}),
	}
}

func (p *cpuProfiler) register(ch ProfileConsumer) {
	if ch == nil {
		return
	}
	p.Lock()
	p.consumers[ch] = struct{}{}
	p.Unlock()
}

func (p *cpuProfiler) unregister(ch ProfileConsumer) {
	if ch == nil {
		return
	}
	p.Lock()
	delete(p.consumers, ch)
	p.Unlock()
}

// StartCPUProfiler uses to start to run the global cpuProfiler.
func StartCPUProfiler(ctx context.Context, window time.Duration) error {
	profileWindow = window
	return globalCPUProfiler.start(ctx)
}

func (p *cpuProfiler) start(ctx context.Context) error {
	go p.profilingLoop(ctx)

	log.Println("cpu profiler started")

	return nil
}


func (p *cpuProfiler) profilingLoop(ctx context.Context) {
	checkTicker := time.NewTicker(profileWindow)
	defer func() {
		checkTicker.Stop()
		pprof.StopCPUProfile()
	}()
	for {
		select {
		case <-ctx.Done():
			log.Println("cpu profiler stopped")
			return
		case <-checkTicker.C:
			p.doProfiling()
		}
	}
}

func (p *cpuProfiler) doProfiling() {
	if p.profileData != nil {
		pprof.StopCPUProfile()
		p.lastDataSize = p.profileData.Data.Len()
		p.sendToConsumers()
	}

	if len(p.consumers) == 0 {
		return
	}

	capacity := (p.lastDataSize/4096 + 1) * 4096
	p.profileData = &ProfileData{Data: bytes.NewBuffer(make([]byte, 0, capacity))}
	err := pprof.StartCPUProfile(p.profileData.Data)
	if err != nil {
		p.profileData.Error = err
		// notify error as soon as possible
		p.sendToConsumers()
		return
	}
}

func (p *cpuProfiler) sendToConsumers() {
	p.Lock()
	defer func() {
		p.Unlock()
		if r := recover(); r != nil {
			log.Printf("cpu profiler panic: %v", r)
		}
	}()

	for c := range p.consumers {
		select {
		case c <- p.profileData:
		default:
			// ignore
		}
	}
	p.profileData = nil
}
