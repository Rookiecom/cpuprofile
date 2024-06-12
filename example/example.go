package main

import (
	"context"
	"fmt"
	"log"
	"main/task"
	"math/rand"
	_ "net/http/pprof"
	"sync"
	"time"

	"github.com/Rookiecom/cpuprofile"
)

var (
	window = 1500 * time.Millisecond
)

func handleData(receiveChan chan *cpuprofile.DataSetAggregate, wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		dataSet := <-receiveChan
		if dataSet == nil {
			break
		}
		fmt.Println("----------------------------")
		for labelVal, cpuTime := range dataSet.Stats {
			log.Printf("label: %8s cpuTime: %d ms\n", labelVal, cpuTime)
		}
	}
	fmt.Println("----------------------------")
}

func main() {
	cpuprofile.StartCPUProfiler(window) // 采集CPU信息的窗口是window
	defer cpuprofile.StopCPUProfiler()
	cpuprofile.StartAggregator()
	defer cpuprofile.StopAggregator()
	receiveChan := make(chan *cpuprofile.DataSetAggregate)
	cpuprofile.RegisterTag("task", receiveChan)
	collectWg := sync.WaitGroup{}
	collectWg.Add(1)
	go handleData(receiveChan, &collectWg)
	wg := sync.WaitGroup{}
	ctx := context.Background()
	for i := 0; i < 150; i++ {
		wg.Add(1)
		number := rand.Int()
		if number%2 == 1 {
			go task.Prime(ctx, "task", "prime", true, &wg)
		} else {
			go task.MergeSort(ctx, "task", "mergeSort", true, &wg)
		}
	}
	wg.Wait()
	cpuprofile.UnRegisterTag("task")
	close(receiveChan)
	collectWg.Wait()
}
