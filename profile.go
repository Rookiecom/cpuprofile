package cpuprofile

import (
	"bytes"
	"context"
	"errors"
	"log"
	"runtime/pprof"
	"sync"
	"time"
)

var errProfilerAlreadyStarted = errors.New("cpuProfiler is already started")
var globalCPUProfiler = newCPUProfiler()
var profileWindow = time.Second

type ProfileData struct {
	Data  *bytes.Buffer
	Error error
}

type ProfileConsumer = chan *ProfileData

type cpuProfiler struct {
	ctx          context.Context
	consumers    map[ProfileConsumer]struct{}
	profileData  *ProfileData
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	lastDataSize int
	sync.Mutex
	started bool
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
func StartCPUProfiler(window time.Duration) error {
	profileWindow = window
	return globalCPUProfiler.start()
}

func (p *cpuProfiler) start() error {
	p.Lock()
	if p.started {
		p.Unlock()
		return errProfilerAlreadyStarted
	}

	p.started = true
	p.ctx, p.cancel = context.WithCancel(context.Background())
	p.Unlock()
	p.wg.Add(1)
	go p.profilingLoop()

	log.Println("cpu profiler started")
	return nil
}

// StopCPUProfiler uses to stop the global cpuProfiler.
func StopCPUProfiler() {
	globalCPUProfiler.stop()
}

func (p *cpuProfiler) stop() {
	p.Lock()
	if !p.started {
		p.Unlock()
		return
	}
	p.started = false
	if p.cancel != nil {
		p.cancel()
	}
	p.Unlock()

	p.wg.Wait()
	log.Println("cpu profiler stopped")
}

func (p *cpuProfiler) profilingLoop() {
	checkTicker := time.NewTicker(profileWindow)
	defer func() {
		checkTicker.Stop()
		pprof.StopCPUProfile()
		p.wg.Done()
	}()
	for {
		select {
		case <-p.ctx.Done():
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
