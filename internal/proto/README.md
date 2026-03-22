# gRPC / protobuf (`metrics.proto`)

Сгенерированные файлы:

- `metrics.pb.go`
- `metrics_grpc.pb.go`

Перегенерация (нужны `protoc`, `protoc-gen-go`, `protoc-gen-go-grpc` в `PATH`):

```bash
protoc \
  --proto_path=internal/proto \
  --go_out=internal/proto \
  --go_opt=paths=source_relative \
  --go-grpc_out=internal/proto \
  --go-grpc_opt=paths=source_relative \
  internal/proto/metrics.proto
```

gRPC использует **стандартный protobuf wire codec** (не JSON).
