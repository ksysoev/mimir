package tests

import (
	"io"
	"net/http"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func (s *integrationSuite) TestNodeAPI_MainFunctionality() {
	node := s.startNode("", "node-1")

	resp := s.doReq(http.MethodPut, node.baseURL+"/kv/txt", []byte("hello"), "", "text/plain")
	defer resp.Body.Close()
	require.Equal(s.T(), http.StatusOK, resp.StatusCode)
	assert.Equal(s.T(), "text/plain", resp.Header.Get("Content-Type"))

	resp = s.doReq(http.MethodPut, node.baseURL+"/kv/blob", []byte{0x01, 0x02}, "", "")
	defer resp.Body.Close()
	require.Equal(s.T(), http.StatusOK, resp.StatusCode)
	assert.Equal(s.T(), "application/octet-stream", resp.Header.Get("Content-Type"))

	resp = s.doReq(http.MethodGet, node.baseURL+"/kv/txt", nil, "", "")
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(s.T(), err)
	require.Equal(s.T(), http.StatusOK, resp.StatusCode)
	assert.Equal(s.T(), []byte("hello"), body)
	assert.Equal(s.T(), "txt", resp.Header.Get("X-Key"))

	resp = s.doReq(http.MethodPut, node.baseURL+"/kv/doc", []byte(`{"a":1}`), "", "application/json")
	defer resp.Body.Close()
	require.Equal(s.T(), http.StatusOK, resp.StatusCode)
	v1 := resp.Header.Get("X-Version")

	resp = s.doReq(http.MethodPut, node.baseURL+"/kv/doc?ifVersion="+v1, []byte(`{"a":2}`), "", "application/json")
	defer resp.Body.Close()
	require.Equal(s.T(), http.StatusOK, resp.StatusCode)
	v2 := resp.Header.Get("X-Version")
	assert.NotEqual(s.T(), v1, v2)

	resp = s.doReq(http.MethodPatch, node.baseURL+"/kv/doc", []byte(`{"b":3}`), "", "application/json")
	defer resp.Body.Close()
	require.Equal(s.T(), http.StatusOK, resp.StatusCode)

	resp = s.doReq(http.MethodGet, node.baseURL+"/kv/doc", nil, "", "")
	defer resp.Body.Close()
	body, err = io.ReadAll(resp.Body)
	require.NoError(s.T(), err)
	assert.JSONEq(s.T(), `{"a":2,"b":3}`, string(body))

	resp = s.doReq(http.MethodGet, node.baseURL+"/kv", nil, "", "")
	defer resp.Body.Close()
	listBody, err := io.ReadAll(resp.Body)
	require.NoError(s.T(), err)
	assert.Contains(s.T(), string(listBody), `"key":"txt"`)
	assert.Contains(s.T(), string(listBody), `"key":"doc"`)
}

func (s *integrationSuite) TestNodeAPI_AuthAndValidation() {
	node := s.startNode("secret", "node-1")

	resp := s.doReq(http.MethodPut, node.baseURL+"/kv/k", []byte(`{}`), "", "application/json")
	defer resp.Body.Close()
	require.Equal(s.T(), http.StatusUnauthorized, resp.StatusCode)

	resp = s.doReq(http.MethodPut, node.baseURL+"/kv/k", []byte(`{}`), "secret", "application/json")
	defer resp.Body.Close()
	require.Equal(s.T(), http.StatusOK, resp.StatusCode)

	resp = s.doReq(http.MethodPut, node.baseURL+"/kv/k?ifVersion=bad", []byte(`{}`), "secret", "application/json")
	defer resp.Body.Close()
	require.Equal(s.T(), http.StatusBadRequest, resp.StatusCode)

	resp = s.doReq(http.MethodPatch, node.baseURL+"/kv/k", []byte(`raw`), "secret", "text/plain")
	defer resp.Body.Close()
	require.Equal(s.T(), http.StatusUnsupportedMediaType, resp.StatusCode)
}
