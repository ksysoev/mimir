package tests

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func (s *integrationSuite) TestConcurrentCounterIncrement_ThreeClients() {
	node := s.startNode("", "node-1")

	seedResp := s.doReq(http.MethodPut, node.baseURL+"/kv/counter", []byte(`{"value":0}`), "", "application/json")
	seedResp.Body.Close()

	require.Equal(s.T(), http.StatusOK, seedResp.StatusCode)

	const (
		clients       = 3
		incrementsPer = 100
	)

	var wg sync.WaitGroup

	errCh := make(chan error, clients)

	for i := 0; i < clients; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			rng := rand.New(rand.NewSource(time.Now().UnixNano())) //nolint:gosec // weak random is fine for retry jitter in tests
			success := 0

			for success < incrementsPer {
				getResp := s.doReq(http.MethodGet, node.baseURL+"/kv/counter", nil, "", "")

				if getResp.StatusCode != http.StatusOK {
					_ = getResp.Body.Close()
					errCh <- fmt.Errorf("get failed: %d", getResp.StatusCode)

					return
				}

				body, err := io.ReadAll(getResp.Body)
				_ = getResp.Body.Close()

				if err != nil {
					errCh <- err
					return
				}

				var state struct {
					Value int `json:"value"`
				}

				if err := json.Unmarshal(body, &state); err != nil {
					errCh <- err
					return
				}

				ver := getResp.Header.Get("X-Version")
				putBody := []byte(fmt.Sprintf(`{"value":%d}`, state.Value+1))
				putResp := s.doReq(http.MethodPut, node.baseURL+"/kv/counter?ifVersion="+ver, putBody, "", "application/json")

				switch putResp.StatusCode {
				case http.StatusOK:
					_ = putResp.Body.Close()
					success++
				case http.StatusConflict:
					_ = putResp.Body.Close()

					time.Sleep(time.Duration(rng.Intn(4)+1) * time.Millisecond)
				default:
					_ = putResp.Body.Close()
					errCh <- fmt.Errorf("unexpected PUT status: %d", putResp.StatusCode)

					return
				}
			}
		}()
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		require.NoError(s.T(), err)
	}

	finalResp := s.doReq(http.MethodGet, node.baseURL+"/kv/counter", nil, "", "")
	defer finalResp.Body.Close()

	require.Equal(s.T(), http.StatusOK, finalResp.StatusCode)

	body, err := io.ReadAll(finalResp.Body)
	require.NoError(s.T(), err)

	var out struct {
		Value int `json:"value"`
	}

	require.NoError(s.T(), json.Unmarshal(body, &out))
	assert.Equal(s.T(), clients*incrementsPer, out.Value)
	assert.Equal(s.T(), "301", finalResp.Header.Get("X-Version"))
}
