package main

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"
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
