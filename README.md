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
