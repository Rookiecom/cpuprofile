package cpuprofile

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/google/pprof/profile"
)

var (
	globalAggregator       = newAggregator()
	enableWindowAggregator = false
	windowAggregator       *WindowAggregator
)

type DataSetAggregate struct {
	TotalCPUTimeMs int
	Stats          map[string]int // in milliseconds.
}

type Aggregator struct {
	sync.Mutex

	dataCh ProfileConsumer

	tags map[string]chan *DataSetAggregate
}

type DataSetAggregateMap map[string]*DataSetAggregate

type WindowAggregator struct {
	sync.Mutex
	isLoad    bool
	start     int
	end       int
	window    int // represents a multiple of the sampling window
	segData   []DataSetAggregateMap
	mergeData DataSetAggregateMap
}

type LabelValueTime struct {
	LabelValue string
	Time       int
}

func newAggregator() *Aggregator {
	return &Aggregator{
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

func GetWindowData() DataSetAggregateMap {
	windowAggregator.Lock()
	defer windowAggregator.Unlock()
	ret := DataSetAggregateMap{}
	for label, mp := range windowAggregator.mergeData {
		ret[label] = &DataSetAggregate{
			TotalCPUTimeMs: mp.TotalCPUTimeMs,
			Stats:          make(map[string]int),
		}
		for labelValue, t := range mp.Stats {
			ret[label].Stats[labelValue] = t
		}
	}
	return ret
}

func (da DataSetAggregateMap) TopN(num int) []LabelValueTime {
	var totalData []LabelValueTime
	for _, mp := range da {
		for labelValue, t := range mp.Stats {
			totalData = append(totalData, LabelValueTime{labelValue, t})
		}
	}
	sort.Slice(totalData, func(i, j int) bool {
		return totalData[i].Time > totalData[j].Time
	})
	if len(totalData) >= num {
		return totalData[0:num]
	}
	return totalData
}

func (da DataSetAggregateMap) TopNWithLabel(num int, label string) []LabelValueTime {
	var totalData []LabelValueTime

	for labelValue, t := range da[label].Stats {
		totalData = append(totalData, LabelValueTime{labelValue, t})
	}

	sort.Slice(totalData, func(i, j int) bool {
		return totalData[i].Time > totalData[j].Time
	})
	if len(totalData) >= num {
		return totalData[0:num]
	}
	return totalData
}

func EnableWindowAggregator(window int) {
	enableWindowAggregator = true
	windowAggregator = &WindowAggregator{
		isLoad:    false,
		start:     0,
		end:       0,
		window:    window,
		segData:   make([]DataSetAggregateMap, window),
		mergeData: map[string]*DataSetAggregate{},
	}
}

func (wa *WindowAggregator) move(dataMap DataSetAggregateMap) {
	wa.Lock()
	defer wa.Unlock()
	if wa.start == wa.end && wa.isLoad {
		for key, dataSet := range wa.segData[wa.start] {
			mergeDataSet, ok := wa.mergeData[key]
			if !ok {
				mergeDataSet = &DataSetAggregate{
					Stats: make(map[string]int),
				}
				wa.mergeData[key] = mergeDataSet
			}
			mergeDataSet.TotalCPUTimeMs -= dataSet.TotalCPUTimeMs
			for k, v := range dataSet.Stats {
				mergeDataSet.Stats[k] -= v
			}
		}
		wa.start = (wa.start + 1) % wa.window
	}
	for key, dataSet := range dataMap {
		mergeDataSet, ok := wa.mergeData[key]
		if !ok {
			mergeDataSet = &DataSetAggregate{
				Stats: make(map[string]int),
			}
			wa.mergeData[key] = mergeDataSet
		}
		mergeDataSet.TotalCPUTimeMs += dataSet.TotalCPUTimeMs
		for k, v := range dataSet.Stats {
			mergeDataSet.Stats[k] += v
		}
	}
	wa.segData[wa.end] = dataMap
	wa.end = (wa.end + 1) % wa.window
	wa.isLoad = true
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

func (pa *Aggregator) start(ctx context.Context) error {
	go pa.aggregateProfileData(ctx)
	// log.Println("cpu profile data aggregator start")
	return nil
}

func (pa *Aggregator) aggregateProfileData(ctx context.Context) {
	// register cpu profile consumer.
	globalCPUProfiler.register(pa.dataCh)
	defer func() {
		globalCPUProfiler.unregister(pa.dataCh)
	}()

	for {
		select {
		case <-ctx.Done():
			// log.Println("cpu profile data aggregator stop")
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
		return
	}
	if len(pa.tags) == 0 && !enableWindowAggregator {
		return
	}
	dataMap := make(DataSetAggregateMap)
	pf, err := profile.ParseData(data.Data.Bytes())
	if err != nil {
		return
	}
	idx := len(pf.SampleType) - 1
	for _, s := range pf.Sample {
		for label := range s.Label {
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
		if _, ok := pa.tags[tag]; !ok {
			continue
		}
		pa.tags[tag] <- dataSet
	}

	if enableWindowAggregator {
		windowAggregator.move(dataMap)
	}
}
