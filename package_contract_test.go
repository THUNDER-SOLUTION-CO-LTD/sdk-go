package thunder

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPackageDefaultsAndEnvironment(t *testing.T) {
	t.Setenv("THUNDER_API_KEY", "")
	t.Setenv("THUNDER_V1_BASE_URL", "")
	t.Setenv("THUNDER_V2_BASE_URL", "")
	path := filepath.Join(t.TempDir(), "key")
	if err := os.WriteFile(path, []byte("fixture-key\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("THUNDER_API_KEY_FILE", path)
	c, err := NewClient("", WithHTTPClient(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("Authorization") != "Bearer fixture-key" || !(strings.HasPrefix(r.Header.Get("User-Agent"), "thunder-slip-go/") && strings.Contains(r.Header.Get("User-Agent"), " go/")) {
			t.Fatal("request headers")
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"data":[]}`))}, nil
	})}))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if c.v1.base != "https://api.thunder.in.th/v1" || c.v2.base != "https://api.thunder.in.th/v2" || WebhookSignatureHeader != "X-Thunder-Signature" {
		t.Fatal("defaults")
	}
	var _ ThunderError = (*ApiError)(nil)
	if (*Thunder)(c) != c {
		t.Fatal("client alias")
	}
	if _, err = c.Banks.List(context.Background()); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(c.String()+c.GoString(), "fixture-key") {
		t.Fatal("credential exposed")
	}
}
