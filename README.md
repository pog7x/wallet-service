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
