package server

import (
	"context"
	"log"
	"net"

	metrics "github.com/DaniilPodruchnyi/metrics-collector/internal/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// startGRPCServer поднимает gRPC‑сервер для работы с метриками.
func (s *Server) startGRPCServer(ctx context.Context) {
	addr := s.config.GRPCAddress
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Printf("Failed to start gRPC listener on %s: %v", addr, err)
		return
	}

	opts := []grpc.ServerOption{
		grpc.UnaryInterceptor(s.grpcUnaryInterceptor()),
	}

	s.grpcServer = grpc.NewServer(opts...)
	metrics.RegisterMetricsServer(s.grpcServer, s)

	log.Printf("gRPC server listening on %s", addr)

	// Останавливаем сервер при отмене контекста.
	go func() {
		<-ctx.Done()
		if s.grpcServer != nil {
			s.grpcServer.GracefulStop()
		}
	}()

	if err := s.grpcServer.Serve(lis); err != nil && err != grpc.ErrServerStopped {
		log.Printf("gRPC server error: %v", err)
	}
}

// grpcUnaryInterceptor реализует проверку trusted_subnet на уровне gRPC.
func (s *Server) grpcUnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		// Если доверенная подсеть не настроена — пропускаем запрос.
		if s.trustedCIDR != nil {
			md, ok := metadata.FromIncomingContext(ctx)
			if !ok {
				return nil, status.Error(codes.PermissionDenied, "missing metadata")
			}

			ips := md.Get("x-real-ip")
			if len(ips) == 0 {
				return nil, status.Error(codes.PermissionDenied, "missing x-real-ip metadata")
			}

			ip := net.ParseIP(ips[0])
			if ip == nil {
				return nil, status.Error(codes.PermissionDenied, "invalid x-real-ip metadata")
			}

			if !s.trustedCIDR.Contains(ip) {
				return nil, status.Error(codes.PermissionDenied, "forbidden: IP not in trusted subnet")
			}
		}

		return handler(ctx, req)
	}
}

