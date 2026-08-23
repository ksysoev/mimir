package tests

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/ksysoev/mimir/pkg/api"
	"github.com/ksysoev/mimir/pkg/core"
	"github.com/ksysoev/mimir/pkg/repo/inmemory"
	"github.com/ksysoev/mimir/pkg/router"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type startedServer struct {
	baseURL string
	stop    func()
}

type integrationSuite struct {
	suite.Suite
	stops []func()
}

func newIntegrationSuite() *integrationSuite { return &integrationSuite{} }

func (s *integrationSuite) BeforeTest(_, _ string) {
	s.stops = nil
}

func (s *integrationSuite) AfterTest(_, _ string) {
	for i := len(s.stops) - 1; i >= 0; i-- {
		s.stops[i]()
	}
}

func (s *integrationSuite) trackStop(stop func()) { s.stops = append(s.stops, stop) }

func (s *integrationSuite) getFreeAddr() string {
	s.T().Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(s.T(), err)
	defer ln.Close()

	return ln.Addr().String()
}

func (s *integrationSuite) waitForLivez(baseURL string) {
	s.T().Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(baseURL + "/livez")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(20 * time.Millisecond)
	}

	s.T().Fatalf("server at %s did not become healthy in time", baseURL)
}

func (s *integrationSuite) startNode(apiKey, nodeID string) startedServer {
	s.T().Helper()

	addr := s.getFreeAddr()
	baseURL := "http://" + addr

	store := inmemory.NewStore(inmemory.Config{MaxKeys: 10000})
	svc := core.New(store)

	a, err := api.New(api.Config{Listen: addr, Key: apiKey, NodeID: nodeID, MaxBodySize: 1 << 20}, svc)
	require.NoError(s.T(), err)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- a.Run(ctx) }()

	s.waitForLivez(baseURL)

	stop := func() {
		cancel()
		select {
		case err := <-done:
			require.NoError(s.T(), err)
		case <-time.After(2 * time.Second):
			s.T().Fatalf("timeout stopping node %s", baseURL)
		}
	}
	s.trackStop(stop)

	return startedServer{baseURL: baseURL, stop: stop}
}

func (s *integrationSuite) startRouter(clientKey, internalKey string, nodes []router.NodeConfig) startedServer {
	s.T().Helper()

	addr := s.getFreeAddr()
	baseURL := "http://" + addr

	cfg := &router.Config{Listen: addr, Key: clientKey, InternalKey: internalKey, Nodes: nodes, MaxBodySize: 1 << 20}
	r, err := router.New(cfg)
	require.NoError(s.T(), err)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- r.Run(ctx, cfg) }()

	s.waitForLivez(baseURL)

	stop := func() {
		cancel()
		select {
		case err := <-done:
			require.NoError(s.T(), err)
		case <-time.After(2 * time.Second):
			s.T().Fatalf("timeout stopping router %s", baseURL)
		}
	}
	s.trackStop(stop)

	return startedServer{baseURL: baseURL, stop: stop}
}

func (s *integrationSuite) doReq(method, url string, body []byte, apiKey, contentType string) *http.Response {
	s.T().Helper()

	var rd io.Reader = http.NoBody
	if body != nil {
		rd = bytes.NewReader(body)
	}

	req, err := http.NewRequest(method, url, rd)
	require.NoError(s.T(), err)

	if apiKey != "" {
		req.Header.Set("X-API-Key", apiKey)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	resp, err := http.DefaultClient.Do(req)
	require.NoError(s.T(), err)

	return resp
}
