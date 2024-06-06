package cpuprofile

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

type testConsumer struct {
	mergeData []*tagsProfile
	splitDate [][]*tagsProfile
	consumer  *Consumer
}

func (t *testConsumer) testDataHandle(profileData *ProfileData) error {
	if profileData.Error != nil {
		return profileData.Error
	}
	profiles, err := analyse(profileData.Data)
	if err != nil {
		return err
	}
	t.splitDate = append(t.splitDate, profiles)
	for i := 0; i < len(profiles); i++ {
		j := 0
		for ; j < len(t.mergeData); j++ {
			if profiles[i].Key == t.mergeData[j].Key {
				t.mergeData[j].Value += profiles[i].Value
				break
			}
		}
		if j == len(t.mergeData) {
			t.mergeData = append(t.mergeData, profiles[i])
		}
	}
	return nil
}

func TestLoad(t *testing.T) {
	fmt.Println("并行筛法求素数 10倍负载 和 100倍负载测试开始")
	fmt.Println("N倍负载同时启动：------")
	StartCPUProfiler(time.Duration(1000)*time.Millisecond, time.Duration(500)*time.Millisecond)
	ParallelConsumer := testConsumer{}
	ParallelConsumer.consumer = NewConsumer(ParallelConsumer.testDataHandle)
	ParallelConsumer.consumer.StartConsume()
	ctx := context.Background()
	wg := sync.WaitGroup{}
	parallelStartNprime(ctx, 10, &wg)
	parallelStartNprime(ctx, 100, &wg)
	wg.Wait()
	ParallelConsumer.consumer.StopConsume()
	for i := range ParallelConsumer.mergeData {
		if ParallelConsumer.mergeData[i].Key == "" {
			continue
		}
		fmt.Printf("任务：%s\n  CPU使用量：%d\n", ParallelConsumer.mergeData[i].Key, ParallelConsumer.mergeData[i].Value)
	}
	fmt.Println("N倍负载并行测试完成")
}
