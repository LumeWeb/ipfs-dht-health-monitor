package check

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// validCheckResponse builds a valid CheckResponse JSON payload for mock servers.
func validCheckResponse() CheckResponse {
	return CheckResponse{
		MutableResolution: &MutableResolution{
			InputPath:    "/ipns/example.com",
			ResolvedPath: "/ipfs/QmExample",
		},
		Providers: []ProviderOutput{
			{
				ID:     "12D3KooWTest",
				Source: SourceDHT,
				Addrs:  []string{"/ip4/1.2.3.4/tcp/4001"},
				DataAvailableOverBitswap: BitswapCheckOutput{
					Enabled:   true,
					Duration:  Duration(500 * time.Millisecond),
					Found:     true,
					Responded: true,
				},
				DataAvailableOverHTTP: HTTPCheckOutput{
					Enabled:   true,
					Duration:  Duration(200 * time.Millisecond),
					Found:     true,
					Connected: true,
				},
			},
		},
	}
}

func TestClient_Check_Success(t *testing.T) {
	t.Parallel()

	resp := validCheckResponse()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if got := q.Get("cid"); got != "/ipns/example.com" {
			t.Errorf("expected cid=/ipns/example.com, got %q", got)
		}
		if got := q.Get("httpRetrieval"); got != "on" {
			t.Errorf("expected httpRetrieval=on, got %q", got)
		}
		if got := q.Get("ipniIndexer"); got != "https://cid.contact" {
			t.Errorf("expected ipniIndexer=https://cid.contact, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Fatalf("encoding response: %v", err)
		}
	}))
	defer srv.Close()

	client := NewClient([]string{srv.URL}, 30*time.Second, "https://cid.contact")
	got, err := client.Check(t.Context(), srv.URL, "example.com", 10*time.Second)
	if err != nil {
		t.Fatalf("Check() error: %v", err)
	}

	if got.MutableResolution == nil {
		t.Fatal("MutableResolution is nil")
	}
	if got.MutableResolution.ResolvedPath != "/ipfs/QmExample" {
		t.Errorf("ResolvedPath = %q, want /ipfs/QmExample", got.MutableResolution.ResolvedPath)
	}
	if len(got.Providers) != 1 {
		t.Fatalf("len(Providers) = %d, want 1", len(got.Providers))
	}
	if got.Providers[0].Source != SourceDHT {
		t.Errorf("Source = %q, want %q", got.Providers[0].Source, SourceDHT)
	}
	if !got.Providers[0].DataAvailableOverBitswap.Found {
		t.Error("Bitswap.Found = false, want true")
	}
	if !got.Providers[0].DataAvailableOverHTTP.Found {
		t.Error("HTTP.Found = false, want true")
	}
}

func TestClient_Check_Error(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal server error"))
	}))
	defer srv.Close()

	client := NewClient([]string{srv.URL}, 30*time.Second, "https://cid.contact")
	_, err := client.Check(t.Context(), srv.URL, "example.com", 10*time.Second)
	if err == nil {
		t.Fatal("Check() expected error, got nil")
	}
}

func TestClient_Check_Timeout(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := NewClient([]string{srv.URL}, 30*time.Second, "https://cid.contact")
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()

	_, err := client.Check(ctx, srv.URL, "example.com", 10*time.Second)
	if err == nil {
		t.Fatal("Check() expected error due to timeout, got nil")
	}
	// The error should be context.DeadlineExceeded or wrapped around it.
	if ctx.Err() != context.DeadlineExceeded {
		t.Errorf("context error = %v, want DeadlineExceeded", ctx.Err())
	}
}
