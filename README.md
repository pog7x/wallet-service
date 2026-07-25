```bash
go test -run=^$$ -fuzz=FuzzParseMoney -fuzztime=30s ./internal/money
```

```bash
go test -v -cover ./...
```

```bash
golangci-lint run ./...
```

```bash
golangci-lint fmt ./...
```

```bash
GODEBUG=schedtrace=1000 go test -v -run "^TestTransferBatch" ./internal/account/
```

```bash
GODEBUG=schedtrace=1000,scheddetail=1 go test -v -run "^TestTransferBatch" ./internal/account/
```

```bash
go test -bench=BenchmarkTransfer_RunParallel -cpu=1,2,4,8 ./...
```

```bash
go test -bench=BenchmarkTransfer -run='^$' -cpuprofile=cpu.out
```

```bash
go test -bench=BenchmarkTransfer -run='^$' -memprofile=mem.out
```

```bash
go tool pprof cpu.out
```

```bash
go tool pprof -http=:8080 cpu.out
```
