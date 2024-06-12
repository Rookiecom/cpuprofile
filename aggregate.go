package cpuprofile

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/google/pprof/profile"
)

var globalAggregator = newAggregator()

type DataSetAggregate struct {
	TotalCPUTimeMs int
	Stats          map[string]int // in milliseconds.
}

type Aggregator struct {
	sync.Mutex
	ctx     context.Context
	cancel  context.CancelFunc
	dataCh  ProfileConsumer
	wg      sync.WaitGroup
	started bool
	tags    map[string]chan *DataSetAggregate
}

func newAggregator() *Aggregator {
	ctx, cancel := context.WithCancel(context.Background())
	return &Aggregator{
		ctx:    ctx,
		cancel: cancel,
		dataCh: make(ProfileConsumer, 1),
		tags:   make(map[string]chan *DataSetAggregate),
	}
}

func RegisterTag(tag string, receiveChan chan *DataSetAggregate) {
	globalAggregator.registerTag(tag, receiveChan)
}

func UnRegisterTag(tag string) {
	globalAggregator.unregisterTag(tag)
}

func StartAggregator() error {
	return globalAggregator.start()
}

func StopAggregator() {
	globalAggregator.stop()
}

func (pa *Aggregator) registerTag(tag string, receiveChan chan *DataSetAggregate) {
	pa.Lock()
	pa.tags[tag] = receiveChan
	pa.Unlock()
}

func (pa *Aggregator) unregisterTag(tag string) {
	pa.Lock()
	defer pa.Unlock()
	if _, ok := pa.tags[tag]; !ok {
		return
	}
	delete(pa.tags, tag)
}

func (pa *Aggregator) start() error {
	if pa.started {
		return errors.New("Aggregator already started")
	}
	pa.started = true
	pa.wg.Add(1)
	go pa.aggregateProfileData()
	log.Println("cpu profile data aggregator start")
	return nil
}

func (pa *Aggregator) stop() {
	if !pa.started {
		return
	}
	pa.cancel()
	pa.wg.Wait()
	close(pa.dataCh)
	log.Println("cpu profile data aggregator stop")
}

func (pa *Aggregator) aggregateProfileData() {
	// register cpu profile consumer.
	globalCPUProfiler.register(pa.dataCh)
	defer func() {
		globalCPUProfiler.unregister(pa.dataCh)
		pa.wg.Done()
	}()

	for {
		select {
		case <-pa.ctx.Done():
			return
		case data := <-pa.dataCh:
			if data == nil {
				return
			}
			pa.handleProfileData(data)
		}
	}
}

func (pa *Aggregator) handleProfileData(data *ProfileData) {
	if data.Error != nil {
		log.Println("data error")
		return
	}
	if len(pa.tags) == 0 {
		log.Println("no tag to aggratore")
		return
	}
	dataMap := make(map[string]*DataSetAggregate)
	pf, err := profile.ParseData(data.Data.Bytes())
	if err != nil {
		log.Println("parse data error")
		return
	}
	idx := len(pf.SampleType) - 1
	for _, s := range pf.Sample {
		for label := range s.Label {
			if _, ok := pa.tags[label]; !ok {
				continue
			}
			dataSet, ok := dataMap[label]
			if !ok {
				dataSet = &DataSetAggregate{
					TotalCPUTimeMs: 0,
					Stats:          make(map[string]int),
				}
				dataMap[label] = dataSet
			}
			digists := s.Label[label]
			for _, digist := range digists {
				if _, ok := dataSet.Stats[digist]; !ok {
					dataSet.Stats[digist] = 0
				}
				dataSet.Stats[digist] += int(time.Duration(s.Value[idx]).Milliseconds())
				dataSet.TotalCPUTimeMs += int(time.Duration(s.Value[idx]).Milliseconds())
			}
		}
	}

	for tag, dataSet := range dataMap {
		pa.tags[tag] <- dataSet
	}
}
