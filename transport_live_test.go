package thunder

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

func TestLiveTransportReadiness(t *testing.T) {
	if os.Getenv("THUNDER_INTEGRATION") != "1" {
		t.Skip("set THUNDER_INTEGRATION=1 for authorized integration smoke")
	}
	keyPath := os.Getenv("THUNDER_API_KEY_FILE")
	bases := map[string]string{"v2": os.Getenv("THUNDER_V2_BASE_URL"), "v1": os.Getenv("THUNDER_V1_BASE_URL")}
	key, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatal("cannot read API key file")
	}
	for _, item := range []struct{ version, path string }{{"v2", "/info"}, {"v2", "/banks"}, {"v1", "/me"}} {
		t.Run(item.version+item.path, func(t *testing.T) {
			base := bases[item.version]
			if base == "" {
				t.Fatal("API base URL missing")
			}
			client, err := newTransport(strings.TrimSpace(string(key)), base, WithTimeout(15*time.Second))
			if err != nil {
				t.Fatal("invalid HTTP transport configuration")
			}
			defer client.close()
			ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
			defer cancel()
			started := time.Now()
			body, _, err := client.request(ctx, "GET", item.path, nil, true)
			if err != nil {
				t.Fatalf("integration smoke failed (%T)", err)
			}
			var value map[string]json.RawMessage
			if json.Unmarshal(body, &value) != nil {
				t.Fatal("response is not a JSON object")
			}
			t.Logf("PASS latency_ms=%d response_fields=%d", time.Since(started).Milliseconds(), len(value))
		})
	}
}
