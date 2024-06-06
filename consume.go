package cpuprofile

import (
	"context"
	"errors"
	"sync"
)

type Consumer struct {
	ctx         context.Context
	cancel      context.CancelFunc
	dataCh      ProfileConsumer
	wg          sync.WaitGroup
	started     bool
	dataHandler func(*ProfileData) error
}

func NewConsumer(handler func(*ProfileData) error) *Consumer {
	ctx, cancel := context.WithCancel(context.Background())
	return &Consumer{
		ctx:         ctx,
		cancel:      cancel,
		dataCh:      make(ProfileConsumer, 1),
		dataHandler: handler,
	}
}

func (pc *Consumer) StartConsume() error {
	if pc.started {
		return errors.New("Collector already started")
	}
	pc.started = true
	pc.wg.Add(1)
	go pc.consumeProfileData()
	return nil
}

func (pc *Consumer) StopConsume() {
	if !pc.started {
		return
	}
	pc.cancel()
	pc.wg.Wait()
}

func (pc *Consumer) consumeProfileData() {
	// register cpu profile consumer.
	globalCPUProfiler.register(pc.dataCh)
	defer func() {
		globalCPUProfiler.unregister(pc.dataCh)
		pc.wg.Done()
	}()

	for {
		select {
		case <-pc.ctx.Done():
			return
		case data := <-pc.dataCh:
			pc.dataHandler(data)
		}
	}
}
