// Package orchestrator is responsible for instrumenting inbound xDS client
// requests to the correct aggregated key, forwarding a representative request
// to the upstream origin server, and managing the lifecycle of downstream and
// upstream connections and associates streams. It implements
// go-control-plane's Cache interface in order to receive xDS-based requests,
// send responses, and handle gRPC streams.
//
// This file manages the bookkeeping of downstream clients by tracking inbound
// requests to their corresponding response channels. The contents of this file
// are intended to only be used within the orchestrator module and should not
// be exported.
package orchestrator

import (
	"testing"

	gcp "github.com/envoyproxy/go-control-plane/pkg/cache/v2"
	"github.com/envoyproxy/xds-relay/internal/app/cache"
	"github.com/envoyproxy/xds-relay/internal/app/transport"
	"github.com/envoyproxy/xds-relay/internal/pkg/stats"
	"github.com/stretchr/testify/assert"
)

var (
	mockRequest = gcp.Request{
		TypeUrl: "type.googleapis.com/envoy.api.v2.Listener",
	}
	mockScope = stats.NewMockScope("mockDownstream")
)

func Test_downstreamResponseMap_createWatch(t *testing.T) {
	responseMap := newDownstreamResponseMap()
	assert.Equal(t, 0, len(responseMap.watches))
	responseMap.createWatch(transport.NewRequestV2(&mockRequest), mockScope)
	assert.Equal(t, 1, len(responseMap.watches))
}

func Test_downstreamResponseMap_get(t *testing.T) {
	responseMap := newDownstreamResponseMap()
	request := transport.NewRequestV2(&mockRequest)
	responseMap.createWatch(request, mockScope)
	assert.Equal(t, 1, len(responseMap.watches))
	if _, ok := responseMap.get(request); !ok {
		t.Error("request not found")
	}
}

func Test_downstreamResponseMap_delete(t *testing.T) {
	responseMap := newDownstreamResponseMap()
	request := transport.NewRequestV2(&mockRequest)
	request2 := transport.NewRequestV2(&gcp.Request{
		TypeUrl: "type.googleapis.com/envoy.api.v2.Cluster",
	})
	responseMap.createWatch(request, mockScope)
	responseMap.createWatch(request2, mockScope)
	assert.Equal(t, 2, len(responseMap.watches))
	if _, ok := responseMap.get(request); !ok {
		t.Error("request not found")
	}
	if _, ok := responseMap.get(request2); !ok {
		t.Error("request not found")
	}
	responseMap.delete(request)
	assert.Equal(t, 1, len(responseMap.watches))
	if _, ok := responseMap.get(request); ok {
		t.Error("request found, when should be deleted")
	}
	responseMap.delete(request2)
	assert.Equal(t, 0, len(responseMap.watches))
}

func Test_downstreamResponseMap_deleteAll(t *testing.T) {
	responseMap := newDownstreamResponseMap()
	request := transport.NewRequestV2(&mockRequest)
	request2 := transport.NewRequestV2(&gcp.Request{
		TypeUrl: "type.googleapis.com/envoy.api.v2.Cluster",
	})
	request3 := transport.NewRequestV2(&gcp.Request{
		TypeUrl: "type.googleapis.com/envoy.api.v2.RouteConfiguration",
	})
	responseMap.createWatch(request, mockScope)
	responseMap.createWatch(request2, mockScope)
	responseMap.createWatch(request3, mockScope)
	assert.Equal(t, 3, len(responseMap.watches))
	m := cache.NewRequestsStore()
	m.Set(request)
	m.Set(request2)
	responseMap.deleteAll(m)
	assert.Equal(t, 1, len(responseMap.watches))
	if _, ok := responseMap.get(request); ok {
		t.Error("request found, when should be deleted")
	}
	if _, ok := responseMap.get(request2); ok {
		t.Error("request found, when should be deleted")
	}
}
