package cpuprofile

import (
	"context"
	"errors"
	"io"
	"sync"
	"time"

	"github.com/google/pprof/profile"
)

// DefProfileDuration exports for testing.
var DefProfileDuration = time.Second

// Collector is a cpu profile collector, it collect cpu profile data from globalCPUProfiler.
type Collector struct {
	ctx       context.Context
	writer    io.Writer
	err       error // fields uses to store the result data of collected.
	cancel    context.CancelFunc
	firstRead chan struct{}
	dataCh    ProfileConsumer
	result    *profile.Profile // fields uses to store the result data of collected.
	wg        sync.WaitGroup
	started   bool
}

func NewCollector() *Collector {
	ctx, cancel := context.WithCancel(context.Background())
	return &Collector{
		ctx:       ctx,
		cancel:    cancel,
		firstRead: make(chan struct{}),
		dataCh:    make(ProfileConsumer, 1),
	}
}

// StartCPUProfile is a substitute for the `pprof.StartCPUProfile` function.
// You should use this function instead of `pprof.StartCPUProfile`.
// Otherwise you may fail, or affect the TopSQL feature and pprof profile HTTP API .
// WARN: this function is not thread-safe.
func (pc *Collector) StartCPUProfile(w io.Writer) error {
	if pc.started {
		return errors.New("Collector already started")
	}
	pc.started = true
	pc.writer = w
	pc.wg.Add(1)
	go pc.readProfileData()
	return nil
}

// StopCPUProfile is a substitute for the `pprof.StopCPUProfile` function.
// WARN: this function is not thread-safe.
func (pc *Collector) StopCPUProfile() error {
	if !pc.started {
		return nil
	}

	// wait for reading least 1 profile data.
	select {
	case <-pc.firstRead:
	case <-time.After(DefProfileDuration * 2):
	}

	pc.cancel()
	pc.wg.Wait()
	data, err := pc.buildProfileData()
	if err != nil || data == nil {
		return err
	}
	return data.Write(pc.writer)
}

// WaitProfilingFinish waits for collecting `seconds` profile data finished.
func (pc *Collector) readProfileData() {
	// register cpu profile consumer.
	globalCPUProfiler.register(pc.dataCh)
	defer func() {
		globalCPUProfiler.unregister(pc.dataCh)
		close(pc.dataCh)
		close(pc.firstRead)
		pc.wg.Done()
	}()

	pc.result, pc.err = nil, nil
	firstRead := true
	for {
		select {
		case <-pc.ctx.Done():
			return
		case data := <-pc.dataCh:
			pc.err = pc.handleProfileData(data)
			if pc.err != nil {
				return
			}
			if firstRead {
				firstRead = false
				close(pc.firstRead)
			}
		}
	}
}

func (pc *Collector) handleProfileData(data *ProfileData) error {
	if data.Error != nil {
		return data.Error
	}
	pd, err := profile.ParseData(data.Data.Bytes())
	if err != nil {
		return err
	}
	if pc.result == nil {
		pc.result = pd
		return nil
	}
	pc.result, err = profile.Merge([]*profile.Profile{pc.result, pd})
	return err
}

func (pc *Collector) buildProfileData() (*profile.Profile, error) {
	if pc.err != nil || pc.result == nil {
		return nil, pc.err
	}

	return pc.result, nil
}
