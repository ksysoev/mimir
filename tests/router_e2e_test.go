package tests

import (
	"io"
	"net/http"

	"github.com/ksysoev/mimir/pkg/router"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func (s *integrationSuite) TestRouterE2E_PutGetAndList() {
	internalKey := "internal-secret"
	clientKey := "client-secret"

	n1 := s.startNode(internalKey, "node-1")
	n2 := s.startNode(internalKey, "node-2")
	n3 := s.startNode(internalKey, "node-3")

	r := s.startRouter(clientKey, internalKey, []router.NodeConfig{
		{ID: "node-1", URL: n1.baseURL},
		{ID: "node-2", URL: n2.baseURL},
		{ID: "node-3", URL: n3.baseURL},
	})

	resp := s.doReq(http.MethodPut, r.baseURL+"/kv/cluster-key", []byte(`{"v":1}`), clientKey, "application/json")
	defer resp.Body.Close()
	require.Equal(s.T(), http.StatusOK, resp.StatusCode)

	resp = s.doReq(http.MethodGet, r.baseURL+"/kv/cluster-key", nil, clientKey, "")
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(s.T(), err)
	require.Equal(s.T(), http.StatusOK, resp.StatusCode)
	assert.JSONEq(s.T(), `{"v":1}`, string(body))

	resp = s.doReq(http.MethodGet, r.baseURL+"/kv", nil, clientKey, "")
	defer resp.Body.Close()
	listBody, err := io.ReadAll(resp.Body)
	require.NoError(s.T(), err)
	require.Equal(s.T(), http.StatusOK, resp.StatusCode)
	assert.Contains(s.T(), string(listBody), `"key":"cluster-key"`)
}
