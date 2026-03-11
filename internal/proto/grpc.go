package metrics

import (
	"context"
	"encoding/json"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/encoding"
	"google.golang.org/grpc/status"
)

// jsonCodec реализует gRPC codec поверх JSON.
// Это позволяет использовать gRPC без сгенерированных protobuf-структур.
type jsonCodec struct{}

func (jsonCodec) Name() string {
	return "json"
}

func (jsonCodec) Marshal(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

func (jsonCodec) Unmarshal(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}

// JSONCodec — экспортируемый экземпляр кодека для использования клиентом.
var JSONCodec = jsonCodec{}

func init() {
	// Регистрируем JSON-кодек глобально.
	encoding.RegisterCodec(JSONCodec)
}

// Metric_MType соответствует enum MType из metrics.proto.
type Metric_MType int32

const (
	Metric_GAUGE   Metric_MType = 0
	Metric_COUNTER Metric_MType = 1
)

// Metric соответствует message Metric из metrics.proto.
type Metric struct {
	Id    string      `json:"id"`
	Type  Metric_MType `json:"type"`
	Delta int64       `json:"delta,omitempty"`
	Value float64     `json:"value,omitempty"`
}

// UpdateMetricsRequest соответствует message UpdateMetricsRequest.
type UpdateMetricsRequest struct {
	Metrics []*Metric `json:"metrics"`
}

// UpdateMetricsResponse соответствует message UpdateMetricsResponse.
type UpdateMetricsResponse struct{}

// MetricsServer определяет серверный интерфейс gRPC-сервиса Metrics.
type MetricsServer interface {
	UpdateMetrics(context.Context, *UpdateMetricsRequest) (*UpdateMetricsResponse, error)
}

// UnimplementedMetricsServer может встраиваться для forward‑совместимости.
type UnimplementedMetricsServer struct{}

func (UnimplementedMetricsServer) UpdateMetrics(context.Context, *UpdateMetricsRequest) (*UpdateMetricsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "method UpdateMetrics not implemented")
}

// RegisterMetricsServer регистрирует реализацию сервиса на gRPC‑сервере.
func RegisterMetricsServer(s *grpc.Server, srv MetricsServer) {
	s.RegisterService(&_Metrics_serviceDesc, srv)
}

// MetricsClient определяет клиентский интерфейс gRPC‑сервиса Metrics.
type MetricsClient interface {
	UpdateMetrics(ctx context.Context, in *UpdateMetricsRequest, opts ...grpc.CallOption) (*UpdateMetricsResponse, error)
}

type metricsClient struct {
	cc grpc.ClientConnInterface
}

// NewMetricsClient создаёт новый gRPC‑клиент.
func NewMetricsClient(cc grpc.ClientConnInterface) MetricsClient {
	return &metricsClient{cc}
}

func (c *metricsClient) UpdateMetrics(ctx context.Context, in *UpdateMetricsRequest, opts ...grpc.CallOption) (*UpdateMetricsResponse, error) {
	out := new(UpdateMetricsResponse)
	err := c.cc.Invoke(ctx, "/metrics.Metrics/UpdateMetrics", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Внутренняя обвязка для регистрации метода UpdateMetrics.
func _Metrics_UpdateMetrics_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(UpdateMetricsRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MetricsServer).UpdateMetrics(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/metrics.Metrics/UpdateMetrics",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MetricsServer).UpdateMetrics(ctx, req.(*UpdateMetricsRequest))
	}
	return interceptor(ctx, in, info, handler)
}

var _Metrics_serviceDesc = grpc.ServiceDesc{
	ServiceName: "metrics.Metrics",
	HandlerType: (*MetricsServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "UpdateMetrics",
			Handler:    _Metrics_UpdateMetrics_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "internal/proto/metrics.proto",
}

