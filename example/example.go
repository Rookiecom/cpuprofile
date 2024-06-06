package main

import (
	"bytes"
	"context"
	"fmt"
	"main/task"
	"math/rand"
	"sync"
	"time"

	"github.com/Rookiecom/cpuprofile"
)

var (
	window   = 1500 * time.Millisecond
	interval = 500 * time.Millisecond
)

func main() {
	cpuprofile.StartCPUProfiler(window, interval) // 采集CPU信息的窗口是window，间隔是interval
	defer cpuprofile.StopCPUProfiler()
	consumer := cpuprofile.NewConsumer(task.HandleTaskProfile)
	consumer.StartConsume()
	defer consumer.StopConsume()
	wg := sync.WaitGroup{}
	ctx := context.Background()
	for i := 0; i < 100; i++ {
		wg.Add(1)
		number := rand.Int()
		if number%2 == 1 {
			go task.Prime(ctx, "prime", true, &wg)
		} else {
			go task.MergeSort(ctx, "mergeSort", true, &wg)
		}
	}
	wg.Wait()
}
