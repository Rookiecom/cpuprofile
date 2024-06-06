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
	cpuprofile.StartCPUProfiler(window, interval)
	defer cpuprofile.StopCPUProfiler()
	consumer := cpuprofile.NewConsumer(task.HandleTaskProfile) // 需要提供类型为 func(*ProfileData) error 的回调函数，作用为定义处理从数据采集者的传递的数据的方法。
	consumer.StartConsume()
	defer consumer.StopConsume()
	// 接下来是代码逻辑
}

```

example 文件夹提供了一个使用示例，100次任务，每次随机执行并行筛法求素数或者并行归并排序（执行不堵塞）。

## 兼容说明

由于执行 `cpuprofile.StartCPUProfiler(window, interval)` 意味着会定时执行 `pprof.StartCPUProfile` 和 `pprof.StopCPUProfile` ( pprof 是 runtime/pprof )，所以对于 go 官方提供的基于上述两个函数的服务，都要做兼容性处理。如果引用了本库、执行了 `cpuprofile.StartCPUProfiler(window, interval)`，而且使用 `pprof.StartCPUProfile` 来采集程序的 CPU 性能信息，两者就会互相影响，导致两者都不能正常工作。

所以，如果要使用 `pprof.StartCPUProfile` 和 `pprof.StopCPUProfile` 的功能，需要按照下面的方式做：

```go
w := bytes.Buffer{}
pc := cpuprofile.NewCollector(&w)
err := pc.StartCPUProfile()
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

测试分别统计了 100次并行求素数和 10次并行求素数的 CPU 用量，发现 100次的 CPU 用量约等于10次并行求素数的10倍，符合预期
