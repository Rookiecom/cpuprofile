package cpuprofile

import (
	"io"
	"time"

	"github.com/google/pprof/profile"
)

type Collector struct {
	consumer  *Consumer
	writer    io.Writer
	err       error // fields uses to store the result data of collected.
	firstRead chan struct{}
	result    *profile.Profile // fields uses to store the result data of collected.
}

func NewCollector(writer io.Writer) *Collector {
	pc := &Collector{
		writer:    writer,
		firstRead: make(chan struct{}),
	}
	pc.consumer = NewConsumer(pc.handleProfileData)
	return pc
}

// StartCPUProfile is a substitute for the `pprof.StartCPUProfile` function.
// You should use this function instead of `pprof.StartCPUProfile`.
// Otherwise you may fail, or affect the TopSQL feature and pprof profile HTTP API .
// WARN: this function is not thread-safe.
func (pc *Collector) StartCPUProfile() error {
	return pc.consumer.StartConsume()
}

// StopCPUProfile is a substitute for the `pprof.StopCPUProfile` function.
// WARN: this function is not thread-safe.
func (pc *Collector) StopCPUProfile() error {
	if !pc.consumer.started {
		return nil
	}

	// wait for reading least 1 profile data.
	select {
	case <-pc.firstRead:
	case <-time.After((profileWindow + profileInterval) * 2):
	}
	pc.consumer.StopConsume()
	data, err := pc.buildProfileData()
	if err != nil || data == nil {
		return err
	}

	return data.Write(pc.writer)
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
		pc.firstRead <- struct{}{}
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
