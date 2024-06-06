package cpuprofile

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"runtime/pprof"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/google/pprof/profile"
)

const primeBound = 10000
const minBound = 10
const arrayLen = 500000

type tagsProfile struct {
	Key     string
	Tags    []string
	Value   int64   // pprof cpu times
	Percent float64 // <= 1.0
}

func labelToTags(label map[string][]string) []string {
	tags := make([]string, 0, len(label)*2)
	for k, v := range label {
		tags = append(tags, k, strings.Join(v, ","))
	}
	return tags
}

func tagsToKey(tags []string) string {
	if len(tags)%2 != 0 {
		return ""
	}
	tagsPair := make([]string, 0, len(tags)/2)
	for i := 0; i < len(tags); i += 2 {
		tagsPair = append(tagsPair, fmt.Sprintf("%s=%s", tags[i], tags[i+1]))
	}
	// sort tags to make it a unique key
	sort.Strings(tagsPair)
	return strings.Join(tagsPair, "|")
}

func analyse(data *bytes.Buffer) ([]*tagsProfile, error) {
	// parse protobuf data
	pf, err := profile.ParseData(data.Bytes())
	if err != nil {
		return nil, err
	}

	// filter cpu value index
	sampleIdx := -1
	for idx, st := range pf.SampleType {
		if st.Type == "cpu" {
			sampleIdx = idx
			break
		}
	}
	if sampleIdx < 0 {
		return nil, errors.New("profiler: sample type not found")
	}

	// calculate every sample expense
	counter := map[string]*tagsProfile{}
	var total int64
	for _, sm := range pf.Sample {
		value := sm.Value[sampleIdx]
		tags := labelToTags(sm.Label)
		tagsKey := tagsToKey(tags)
		tp, ok := counter[tagsKey]
		if !ok {
			tp = &tagsProfile{}
			counter[tagsKey] = tp
			tp.Key = tagsKey
			tp.Tags = tags
		}
		tp.Value += value
		total += value
	}

	profiles := make([]*tagsProfile, 0, len(counter))
	for _, l := range counter {
		l.Percent = float64(l.Value) / float64(total)
		profiles = append(profiles, l)
	}
	return profiles, nil
}

func selectPrime(ctx context.Context, c chan int, wg *sync.WaitGroup, enableProfile bool) {
	if enableProfile {
		pprof.SetGoroutineLabels(ctx)
	}
	defer wg.Done()
	prime, ok := <-c
	if !ok {
		return
	}
	// fmt.Println(prime)
	newChan := make(chan int)
	newWg := sync.WaitGroup{}
	newWg.Add(1)
	go selectPrime(ctx, newChan, &newWg, enableProfile)
	for n := range c {
		if n%prime != 0 {
			newChan <- n
		}
	}
	close(newChan)
	newWg.Wait()
}

func prime(ctx context.Context, label string, enableProfile bool, externWg *sync.WaitGroup) {
	// 筛法求素数, 用作测试任务
	defer externWg.Done()
	if enableProfile {
		defer pprof.SetGoroutineLabels(ctx)
		ctx = pprof.WithLabels(ctx, pprof.Labels("task", label))
		pprof.SetGoroutineLabels(ctx)
	}

	wg := sync.WaitGroup{}
	wg.Add(1)
	c := make(chan int)
	n := primeBound
	go selectPrime(ctx, c, &wg, enableProfile)
	for i := 2; i <= n; i++ {
		c <- i
	}
	close(c)
	wg.Wait()
}

func parallelStartNprime(ctx context.Context, number int, wg *sync.WaitGroup) {
	for i := 0; i < number; i++ {
		wg.Add(1)
		go prime(ctx, "prime"+" "+strconv.Itoa(number)+"x load", true, wg)
	}
}
