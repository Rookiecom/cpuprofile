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

var errProfilerAlreadyStarted = errors.New("parallelCPUProfiler is already started")
var globalCPUProfiler = newParallelCPUProfiler()
var profileWindow = time.Second
var profileInterval = time.Second * 0

type ProfileData struct {
	Data  *bytes.Buffer
	Error error
}

type ProfileConsumer = chan *ProfileData

type parallelCPUProfiler struct {
	ctx            context.Context
	consumers      map[ProfileConsumer]struct{}
	notifyRegister chan struct{}
	profileData    *ProfileData
	cancel         context.CancelFunc
	wg             sync.WaitGroup
	lastDataSize   int
	sync.Mutex
	started bool
}

func newParallelCPUProfiler() *parallelCPUProfiler {
	return &parallelCPUProfiler{
		consumers:      make(map[ProfileConsumer]struct{}),
		notifyRegister: make(chan struct{}),
	}
}

func (p *parallelCPUProfiler) register(ch ProfileConsumer) {
	if ch == nil {
		return
	}
	p.Lock()
	p.consumers[ch] = struct{}{}
	p.Unlock()

	select {
	case p.notifyRegister <- struct{}{}:
	default:
	}
}

func (p *parallelCPUProfiler) unregister(ch ProfileConsumer) {
	if ch == nil {
		return
	}
	p.Lock()
	delete(p.consumers, ch)
	p.Unlock()
}

// StartCPUProfiler uses to start to run the global parallelCPUProfiler.
func StartCPUProfiler(window time.Duration, interval time.Duration) error {
	profileWindow = window
	profileInterval = interval
	return globalCPUProfiler.start()
}

func (p *parallelCPUProfiler) start() error {
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

	log.Println("parallel cpu profiler started")
	return nil
}

// StopCPUProfiler uses to stop the global parallelCPUProfiler.
func StopCPUProfiler() {
	globalCPUProfiler.stop()
}

func (p *parallelCPUProfiler) stop() {
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
	log.Println("parallel cpu profiler stopped")
}

func (p *parallelCPUProfiler) profilingLoop() {
	checkTicker := time.NewTicker(profileWindow + profileInterval)
	timer := time.NewTimer(profileInterval)
	defer func() {
		checkTicker.Stop()
		pprof.StopCPUProfile()
		p.wg.Done()
	}()
	for {
		select {
		case <-p.ctx.Done():
			return
		case <-p.notifyRegister:
			// If already in profiling, don't do anything.
			if p.profileData != nil {
				continue
			}
		case <-checkTicker.C:
			p.doProfiling()
			if profileInterval != 0 {
				timer.Reset(profileInterval)
				<-timer.C
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
		}
	}
}

func (p *parallelCPUProfiler) doProfiling() {
	if p.profileData != nil {
		pprof.StopCPUProfile()
		p.lastDataSize = p.profileData.Data.Len()
		p.sendToConsumers()
	}

	if len(p.consumers) == 0 {
		return
	}
}

func (p *parallelCPUProfiler) sendToConsumers() {
	p.Lock()
	defer func() {
		p.Unlock()
		if r := recover(); r != nil {
			log.Printf("parallel cpu profiler panic: %v", r)
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
