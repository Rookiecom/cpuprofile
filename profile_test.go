package cpuprofile

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

func collectData(totalCPUTimeMs *int, mergeData map[string]int, receiveChan chan *DataSetAggregate, wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		dataSet := <-receiveChan
		if dataSet == nil {
			break
		}
		*totalCPUTimeMs += dataSet.TotalCPUTimeMs
		for label, val := range dataSet.stats {
			if _, ok := mergeData[label]; !ok {
				mergeData[label] = 0
			}
			mergeData[label] += val
		}
	}
}

func TestLoad(t *testing.T) {
	StartCPUProfiler(time.Duration(1000) * time.Millisecond)
	StartAggregator()
	primeReceiveChan := make(chan *DataSetAggregate)
	mergeSortReceiveChan := make(chan *DataSetAggregate)
	RegisterTag("prime", primeReceiveChan)
	RegisterTag("mergeSort", mergeSortReceiveChan)
	ctx := context.Background()
	wg := sync.WaitGroup{}
	primeMergeData := make(map[string]int)
	mergeSortMergeDate := make(map[string]int)
	primeTotalCPUTimeMs := 0
	mergeSortTotalCPUTimeMs := 0
	collectWg := sync.WaitGroup{}
	collectWg.Add(2)
	go collectData(&primeTotalCPUTimeMs, primeMergeData, primeReceiveChan, &collectWg)
	go collectData(&mergeSortTotalCPUTimeMs, mergeSortMergeDate, mergeSortReceiveChan, &collectWg)
	fmt.Println("并行筛法求素数 5倍负载 和 50倍负载测试开始")
	parallelStartNprime(ctx, 5, &wg)
	parallelStartNprime(ctx, 50, &wg)
	fmt.Println("并行归并排序 5倍负载 和 50倍负载测试开始")
	parallelStartNMergeSort(ctx, 5, &wg)
	parallelStartNMergeSort(ctx, 50, &wg)
	wg.Wait()
	UnRegisterTag("prime")
	close(primeReceiveChan)
	UnRegisterTag("mergeSort")
	close(mergeSortReceiveChan)
	StopAggregator()
	StopCPUProfiler()
	collectWg.Wait()
	fmt.Println("----------- label key = prime -----------")
	for label, val := range primeMergeData {
		fmt.Printf("任务：%s\n  CPU使用量：%d ms\n", label, val)
	}
	fmt.Printf("总统计量：%d ms\n", primeTotalCPUTimeMs)
	fmt.Println("----------- label key = mergeSort -----------")
	for label, val := range mergeSortMergeDate {
		fmt.Printf("任务：%s\n  CPU使用量：%d ms\n", label, val)
	}
	fmt.Printf("总统计量：%d ms\n", mergeSortTotalCPUTimeMs)
	fmt.Println("N倍负载并行测试完成")
}
