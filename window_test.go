package cpuprofile

import (
	"context"
	"fmt"
	"runtime/pprof"
	"testing"
	"time"
)

func matrixTranspose(ctx context.Context, label, labelValue string) {
	matrix := make([][]int, 100)
	for i := range matrix {
		matrix[i] = make([]int, 100)
	}
	ctx = pprof.WithLabels(ctx, pprof.Labels(label, labelValue))
	pprof.SetGoroutineLabels(ctx)
	for {
		for i := 0; i < 100; i++ {
			for j := 0; j < i; j++ {
				t := matrix[i][j]
				matrix[i][j] = matrix[j][i]
				matrix[j][i] = t
			}
		}
		select {
		case <-ctx.Done():
			return
		default:
			continue
		}
	}
}

func startNMatrixT(ctx context.Context, num int, isCancel bool, cancelTime time.Duration, label, labelValue string) {
	newCtx := ctx
	var cancel context.CancelFunc
	if isCancel {
		newCtx, cancel = context.WithCancel(newCtx)
	}
	for i := 0; i < num; i++ {
		go matrixTranspose(newCtx, label, labelValue)
	}
	if isCancel {
		go func() {
			time.Sleep(cancelTime)
			cancel()
		}()
	}
}

func TestWindowProfile(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	StartCPUProfiler(ctx, time.Second)
	EnableWindowAggregator(4)
	startNMatrixT(ctx, 4, true, 6*time.Second, "task", "A")
	startNMatrixT(ctx, 2, false, 4*time.Second, "task", "B")
	startNMatrixT(ctx, 1, false, 4*time.Second, "task", "C")
	for i := 0; i < 12; i++ {
		time.Sleep(1 * time.Second)
		go func() {
			data := GetWindowData()
			for label, mp := range data {
				fmt.Println(label, *mp)
			}
			fmt.Println(data.TopN(2))
		}()
	}

}
