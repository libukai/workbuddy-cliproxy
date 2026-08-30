package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v7/sdk/pluginabi"
	"github.com/router-for-me/CLIProxyAPI/v7/sdk/pluginapi"
)

func TestVerifiedModelCatalog(t *testing.T) {
	models := wbModels()
	ids := make(map[string]bool, len(models))
	for _, model := range models {
		ids[model.ID] = true
	}

	want := []string{
		"hy4-preview",
		"glm-5.3",
		"glm-5.3-flash",
		"minimax-m3",
		"kimi-k3",
		"kimi-k2.7",
		"kimi-k2.6",
	}
	for _, id := range want {
		if !ids[id] {
			t.Fatalf("verified model %q is missing", id)
		}
	}
	if ids["kimi-k2.7-code"] {
		t.Fatal("rejected upstream model id kimi-k2.7-code must not be registered")
	}
}

func TestAuthDataSchedulesRefreshBeforeExpiry(t *testing.T) {
	expiresAt := time.Now().UTC().Add(2 * time.Hour).Truncate(time.Second)
	auth := &storedAuth{Auth: storedTokens{
		AccessToken:  "access-token",
		RefreshToken: "refresh-token",
		ExpiresAt:    expiresAt.Unix(),
	}}

	data := toAuthData(auth)
	wantRefresh := expiresAt.Add(-authRefreshLead)
	if !data.NextRefreshAfter.Equal(wantRefresh) {
		t.Fatalf("NextRefreshAfter = %s, want %s", data.NextRefreshAfter, wantRefresh)
	}
	if got := data.Metadata["expires_at"]; got != expiresAt.Format(time.RFC3339) {
		t.Fatalf("expires_at = %v, want %s", got, expiresAt.Format(time.RFC3339))
	}
}

func TestLoginIdentityIsStableAndOpaque(t *testing.T) {
	auth := &storedAuth{
		Auth:    storedTokens{AccessToken: "access", RefreshToken: "refresh"},
		Account: storedAccount{UID: "user-123", EnterpriseID: "enterprise-456"},
	}
	first := identityForLogin(auth)
	second := identityForLogin(auth)
	if first != second {
		t.Fatalf("login identity is not stable: %+v != %+v", first, second)
	}
	if !strings.HasPrefix(first.ID, "workbuddy-") || first.FileName != first.ID+".json" {
		t.Fatalf("unexpected login identity: %+v", first)
	}
	if strings.Contains(first.ID, auth.Account.UID) || strings.Contains(first.FileName, auth.Account.EnterpriseID) {
		t.Fatalf("login identity exposes account identifiers: %+v", first)
	}

	other := *auth
	other.Account.UID = "user-789"
	if identityForLogin(&other).ID == first.ID {
		t.Fatal("different accounts received the same auth ID")
	}
}

func TestParseAuthPreservesCredentialFileIdentity(t *testing.T) {
	storage, err := json.Marshal(&storedAuth{Auth: storedTokens{AccessToken: "access"}})
	if err != nil {
		t.Fatal(err)
	}
	request, err := json.Marshal(pluginapi.AuthParseRequest{FileName: "workbuddy-account123.json", RawJSON: storage})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := handleParseAuth(request)
	if err != nil {
		t.Fatalf("handleParseAuth: %v", err)
	}
	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatal(err)
	}
	var response pluginapi.AuthParseResponse
	if err := json.Unmarshal(env.Result, &response); err != nil {
		t.Fatal(err)
	}
	if response.Auth.ID != "workbuddy-account123" || response.Auth.FileName != "workbuddy-account123.json" {
		t.Fatalf("parsed auth identity = %q / %q", response.Auth.ID, response.Auth.FileName)
	}
}

func TestConcurrentRefreshIsDeduplicated(t *testing.T) {
	originalHostCall := hostCall
	t.Cleanup(func() { hostCall = originalHostCall })
	var calls atomic.Int32
	hostCall = func(method string, _ []byte) ([]byte, error) {
		if method != pluginabi.MethodHostHTTPDo {
			t.Fatalf("host callback method = %q, want %q", method, pluginabi.MethodHostHTTPDo)
		}
		calls.Add(1)
		time.Sleep(25 * time.Millisecond)
		body, err := json.Marshal(apiEnvelope{Code: 0, Data: json.RawMessage(`{"accessToken":"new-access","refreshToken":"new-refresh","expiresIn":3600}`)})
		if err != nil {
			return nil, err
		}
		return okEnvelope(pluginapi.HTTPResponse{StatusCode: http.StatusOK, Body: body})
	}

	storage, err := json.Marshal(&storedAuth{Auth: storedTokens{AccessToken: "old-access", RefreshToken: "dedupe-refresh-token"}})
	if err != nil {
		t.Fatal(err)
	}
	request, err := json.Marshal(rpcAuthRefreshRequest{
		AuthRefreshRequest: pluginapi.AuthRefreshRequest{AuthID: "workbuddy-account123", StorageJSON: storage},
		HostCallbackID:     "callback-refresh",
	})
	if err != nil {
		t.Fatal(err)
	}

	const workers = 8
	start := make(chan struct{})
	errs := make(chan error, workers)
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, errRefresh := handleRefreshAuth(request)
			errs <- errRefresh
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	for errRefresh := range errs {
		if errRefresh != nil {
			t.Fatalf("concurrent refresh: %v", errRefresh)
		}
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("host refresh calls = %d, want 1", got)
	}
}

func TestAggregateSSEReturnsReaderError(t *testing.T) {
	readerErr := errors.New("stream broke")
	reader := &dataThenErrorReader{
		data: []byte("data: {\"choices\":[{\"delta\":{\"content\":\"OK\"}}]}\n"),
		err:  readerErr,
	}

	_, err := aggregateSSE(reader, false)
	if !errors.Is(err, readerErr) {
		t.Fatalf("aggregateSSE error = %v, want %v", err, readerErr)
	}
}

func TestAggregateCompletionReturnsReaderError(t *testing.T) {
	readerErr := errors.New("completion broke")
	reader := &dataThenErrorReader{
		data: []byte("data: {\"choices\":[{\"delta\":{\"content\":\"OK\"}}]}\n"),
		err:  readerErr,
	}

	_, err := aggregateCompletion(reader, "test-model")
	if !errors.Is(err, readerErr) {
		t.Fatalf("aggregateCompletion error = %v, want %v", err, readerErr)
	}
}

func TestShutdownCancelsActiveStreams(t *testing.T) {
	lifecycleMu.Lock()
	lifecycleCtx, lifecycleStop = context.WithCancel(context.Background())
	lifecycleMu.Unlock()

	ctx, ok := beginStream()
	if !ok {
		t.Fatal("beginStream rejected an active plugin lifecycle")
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer streamWG.Done()
		<-ctx.Done()
	}()

	cliproxyPluginShutdown()
	select {
	case <-done:
	default:
		t.Fatal("plugin shutdown returned before the active stream stopped")
	}
}

func TestQuiesceRejectsNewStreams(t *testing.T) {
	lifecycleMu.Lock()
	lifecycleCtx, lifecycleStop = context.WithCancel(context.Background())
	hostStreams = make(map[string]struct{})
	lifecycleMu.Unlock()

	if _, err := handleMethod(methodPluginQuiesce, nil); err != nil {
		t.Fatalf("plugin quiesce: %v", err)
	}
	if _, ok := beginStream(); ok {
		t.Fatal("beginStream accepted a new stream after quiesce")
	}
	if _, err := handleMethod(pluginabi.MethodPluginRegister, nil); err != nil {
		t.Fatalf("plugin register after quiesce: %v", err)
	}
	if _, ok := beginStream(); !ok {
		t.Fatal("plugin register did not reactivate the lifecycle after quiesce")
	}
	streamWG.Done()
	quiescePlugin()
}

func TestUpstreamErrorPreservesHTTPStatusAndRetryability(t *testing.T) {
	tests := []struct {
		status    int
		code      string
		retryable bool
	}{
		{status: http.StatusBadRequest, code: "upstream_bad_request"},
		{status: http.StatusUnauthorized, code: "upstream_auth_error"},
		{status: http.StatusTooManyRequests, code: "upstream_rate_limit", retryable: true},
		{status: http.StatusServiceUnavailable, code: "upstream_server_error", retryable: true},
	}
	for _, tt := range tests {
		t.Run(http.StatusText(tt.status), func(t *testing.T) {
			raw := errorEnvelopeFromError(newUpstreamError(tt.status, []byte(`{"message":"upstream failure"}`)))
			var got envelope
			if err := json.Unmarshal(raw, &got); err != nil {
				t.Fatalf("decode error envelope: %v", err)
			}
			if got.Error == nil {
				t.Fatal("error envelope is missing error details")
			}
			if got.Error.HTTPStatus != tt.status || got.Error.Code != tt.code || got.Error.Retryable != tt.retryable {
				t.Fatalf("error = %+v, want status=%d code=%s retryable=%v", got.Error, tt.status, tt.code, tt.retryable)
			}
		})
	}
}

func TestApplyStreamHeadersPreservesUpstreamMetadata(t *testing.T) {
	headers := http.Header{"X-Request-Id": {"request-123"}}
	applyStreamHeaders(headers)
	if got := headers.Get("X-Request-Id"); got != "request-123" {
		t.Fatalf("X-Request-Id = %q, want request-123", got)
	}
	if got := headers.Get("Content-Type"); got != "text/event-stream" {
		t.Fatalf("Content-Type = %q, want text/event-stream", got)
	}
}

func TestHostHTTPDoUsesCallbackContextAndDecodesResponse(t *testing.T) {
	originalHostCall := hostCall
	t.Cleanup(func() { hostCall = originalHostCall })
	hostCall = func(method string, raw []byte) ([]byte, error) {
		if method != pluginabi.MethodHostHTTPDo {
			t.Fatalf("host callback method = %q, want %q", method, pluginabi.MethodHostHTTPDo)
		}
		var request rpcHostHTTPRequest
		if err := json.Unmarshal(raw, &request); err != nil {
			t.Fatalf("decode host request: %v", err)
		}
		if request.HostCallbackID != "callback-1" || request.Method != http.MethodPost || request.URL != "https://example.test/chat" {
			t.Fatalf("unexpected host request: %+v", request)
		}
		if !bytes.Equal(request.Body, []byte(`{"model":"test"}`)) {
			t.Fatalf("host request body = %q", request.Body)
		}
		return okEnvelope(pluginapi.HTTPResponse{
			StatusCode: http.StatusOK,
			Headers:    http.Header{"X-Request-Id": {"upstream-1"}},
			Body:       []byte(`{"ok":true}`),
		})
	}

	response, err := doHostHTTP("callback-1", http.MethodPost, "https://example.test/chat", http.Header{"Content-Type": {"application/json"}}, []byte(`{"model":"test"}`))
	if err != nil {
		t.Fatalf("doHostHTTP: %v", err)
	}
	if response.StatusCode != http.StatusOK || response.Headers.Get("X-Request-Id") != "upstream-1" || !bytes.Equal(response.Body, []byte(`{"ok":true}`)) {
		t.Fatalf("unexpected host response: %+v", response)
	}
}

func TestHostHTTPStreamReaderReadsUntilDone(t *testing.T) {
	originalHostCall := hostCall
	t.Cleanup(func() { hostCall = originalHostCall })
	reads := 0
	hostCall = func(method string, raw []byte) ([]byte, error) {
		if method != pluginabi.MethodHostHTTPStreamRead {
			t.Fatalf("host callback method = %q, want stream read", method)
		}
		var request rpcHostHTTPStreamReadRequest
		if err := json.Unmarshal(raw, &request); err != nil {
			t.Fatalf("decode stream read request: %v", err)
		}
		if request.StreamID != "stream-1" {
			t.Fatalf("stream ID = %q, want stream-1", request.StreamID)
		}
		reads++
		if reads == 1 {
			return okEnvelope(rpcHostHTTPStreamReadResponse{Payload: []byte("hello ")})
		}
		return okEnvelope(rpcHostHTTPStreamReadResponse{Payload: []byte("world"), Done: true})
	}

	payload, err := io.ReadAll(&hostHTTPStreamReader{streamID: "stream-1"})
	if err != nil {
		t.Fatalf("read host stream: %v", err)
	}
	if string(payload) != "hello world" || reads != 2 {
		t.Fatalf("payload = %q, reads = %d", payload, reads)
	}
}

type dataThenErrorReader struct {
	data []byte
	err  error
}

func (r *dataThenErrorReader) Read(p []byte) (int, error) {
	if len(r.data) > 0 {
		n := copy(p, r.data)
		r.data = r.data[n:]
		return n, nil
	}
	if r.err != nil {
		err := r.err
		r.err = nil
		return 0, err
	}
	return 0, io.EOF
}
