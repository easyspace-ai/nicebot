package clob

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type httpClient interface {
	Do(req *http.Request) (*http.Response, error)
}

func defaultHTTPClient() *http.Client {
	return &http.Client{Timeout: 15 * time.Second}
}

func doJSON(ctx context.Context, c httpClient, method, url string, headers map[string]string, bodyBytes []byte) (any, error) {
	var body io.Reader
	if bodyBytes != nil {
		body = bytes.NewReader(bodyBytes)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}

	// py_clob_client overload headers
	req.Header.Set("User-Agent", "py_clob_client")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Content-Type", "application/json")
	if method == http.MethodGet {
		req.Header.Set("Accept-Encoding", "gzip")
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		// Attempt to parse json error
		var j any
		_ = json.Unmarshal(b, &j)
		if j != nil {
			return nil, fmt.Errorf("CLOB API status=%d error=%v", resp.StatusCode, j)
		}
		return nil, fmt.Errorf("CLOB API status=%d error=%s", resp.StatusCode, string(b))
	}

	// Try json
	var out any
	if err := json.Unmarshal(b, &out); err == nil {
		return out, nil
	}
	return string(b), nil
}
