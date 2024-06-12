# 使用说明

下载：

```shell
go get github.com/Rookiecom/cpuprofile
```

## 使用示例

使用方法：

```go
import "github.com/Rookiecom/cpuprofile"

func main() {
	cpuprofile.StartCPUProfiler(window) // 采集CPU信息的窗口是window
	defer cpuprofile.StopCPUProfiler()
	cpuprofile.StartAggregator()
	defer cpuprofile.StopAggregator()
	receiveChan := make(chan *cpuprofile.DataSetAggregate)
	cpuprofile.RegisterTag("task", receiveChan) 
	// 在 aggregator 处注册需要聚合标签 key 为 task 的样本的 CPU 时间
	// 使用者需要用 receiveChan 接受聚合后的信息
	// 接下来是代码逻辑
}

```

`cpuprofile.DataSetAggregate` 的结构如下：

```go
type DataSetAggregate struct {
	TotalCPUTimeMs int
	Stats          map[string]int // in milliseconds.
}
```

example 文件夹提供了一个使用示例，150次任务，每次随机执行并行筛法求素数或者并行归并排序（执行不堵塞），然后接受每次采样的样本的聚合结果并打印。

## 兼容说明

由于执行 `cpuprofile.StartCPUProfiler(window)` 意味着会定时执行 `pprof.StartCPUProfile` 和 `pprof.StopCPUProfile` ( pprof 是 runtime/pprof )，所以对于 go 官方提供的基于上述两个函数的服务，都要做兼容性处理。如果引用了本库、执行了 `cpuprofile.StartCPUProfiler(window, interval)`，而且使用 `pprof.StartCPUProfile` 来采集程序的 CPU 性能信息，两者就会互相影响，导致两者都不能正常工作。

所以，如果要使用 `pprof.StartCPUProfile` 和 `pprof.StopCPUProfile` 的功能，需要按照下面的方式做：

```go
w := bytes.Buffer{}
pc := cpuprofile.NewCollector()
err := pc.StartCPUProfile(&w)
if err != nil {
    fmt.Println(err)
    return
}
sec := 5 * time.Second
time.Sleep(sec)
err = pc.StopCPUProfile()
if err != nil {
    fmt.Println(err)
    return
}
// 代码逻辑
```

另外，go 通过这样的方式提供了 web 端查看程序执行性能信息的功能：

```go
import _ "net/http/pprof"

func main() {
	go func() {
        log.Println(http.ListenAndServe("localhost:6060", nil))
	}()
}

```

如果使用本库的基础上使用这样的功能，需要做以下替代：

```go
import "github.com/Rookiecom/cpuprofile"

func main() {
    cpuprofile.WebProfile(":6060")
}
```

# 测试说明

只对库做了负载测试，执行下面的命令即可运行测试：

```shell
go test -v -timeout 30s -run TestLoad
```

测试分别统计了 50次并行求素数 和 5次并行求素数的 CPU 用量、50次并行归并排序 和 5次归并排序的 CPU 用量，发现 50次的 CPU 用量约等于 5 次的 10 倍，符合预期

测试结果如下：

```txt
> go test -v -timeout 30s -run TestLoad
=== RUN   TestLoad
2024/06/12 23:44:58 cpu profiler started
2024/06/12 23:44:58 cpu profile data aggregator start
并行筛法求素数 5倍负载 和 50倍负载测试开始
并行归并排序 5倍负载 和 50倍负载测试开始
2024/06/12 23:45:12 cpu profile data aggregator stop
2024/06/12 23:45:12 cpu profiler stopped
----------- label key = prime -----------
label value: 50xload
  CPU使用量：8540 ms
label value: 5xload
  CPU使用量：920 ms
总统计量：9460 ms
----------- label key = mergeSort -----------
label value: 50xload
  CPU使用量：7890 ms
label value: 5xload
  CPU使用量：730 ms
总统计量：8620 ms
N倍负载并行测试完成
--- PASS: TestLoad (14.62s)
PASS
ok      github.com/Rookiecom/cpuprofile 14.794s
```