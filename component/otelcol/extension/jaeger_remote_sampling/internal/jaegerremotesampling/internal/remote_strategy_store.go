// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/jaegerremotesampling/internal"

import (
	"context"
	"fmt"
	"io"
	"time"

	grpcstore "github.com/jaegertracing/jaeger/cmd/agent/app/configmanager/grpc"
	"github.com/jaegertracing/jaeger/cmd/collector/app/sampling/strategystore"
	"github.com/jaegertracing/jaeger/proto-gen/api_v2"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/configopaque"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type grpcRemoteStrategyStore struct {
	headerAdditions map[string]configopaque.String
	delegate        *grpcstore.ConfigManagerProxy
	cache           serviceStrategyCache
}

// NewRemoteStrategyStore returns a StrategyStore that delegates to the configured Jaeger gRPC endpoint, making
// extension-configured enhancements (header additions only for now) to the gRPC context of every outbound gRPC call.
// Note: it would be nice to expand the configuration surface to include an optional TTL-based caching behavior
// for service-specific outbound GetSamplingStrategy calls.
func NewRemoteStrategyStore(
	conn *grpc.ClientConn,
	grpcClientSettings *configgrpc.GRPCClientSettings,
	reloadInterval time.Duration,
) (strategystore.StrategyStore, io.Closer) {

	cache := newNoopStrategyCache()
	if reloadInterval > 0 {
		cache = newServiceStrategyCache(reloadInterval)
	}

	return &grpcRemoteStrategyStore{
		headerAdditions: grpcClientSettings.Headers,
		delegate:        grpcstore.NewConfigManager(conn),
		cache:           cache,
	}, cache
}

func (g *grpcRemoteStrategyStore) GetSamplingStrategy(
	ctx context.Context,
	serviceName string,
) (*api_v2.SamplingStrategyResponse, error) {

	if cachedResponse, ok := g.cache.get(ctx, serviceName); ok {
		return cachedResponse, nil
	}
	freshResult, err := g.delegate.GetSamplingStrategy(g.enhanceContext(ctx), serviceName)
	if err != nil {
		return nil, fmt.Errorf("remote call failed: %w", err)
	}
	g.cache.put(ctx, serviceName, freshResult)
	return freshResult, nil
}

// This function is used to add the extension configuration defined HTTP headers to a given outbound gRPC call's context.
func (g *grpcRemoteStrategyStore) enhanceContext(ctx context.Context) context.Context {
	md := metadata.New(nil)
	for k, v := range g.headerAdditions {
		md.Set(k, string(v))
	}
	return metadata.NewOutgoingContext(ctx, md)
}

func (g *grpcRemoteStrategyStore) Close() error {
	return g.cache.Close()
}
