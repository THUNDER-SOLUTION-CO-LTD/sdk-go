package thunder

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

func TestIntegrationWaitAndError(t *testing.T) {
	if os.Getenv("THUNDER_JOB_INTEGRATION") != "1" {
		t.Skip("explicit integration smoke only")
	}
	key, err := os.ReadFile(os.Getenv("THUNDER_API_KEY_FILE"))
	if err != nil {
		t.Fatal("cannot read key file")
	}
	secret := strings.TrimSpace(string(key))
	raw, err := os.ReadFile(fixturePath("v2/jobs/get/done.json"))
	if err != nil {
		t.Fatal("cannot read job fixture")
	}
	var fixture struct {
		Call struct{ Args struct{ JobID string } }
	}
	if json.Unmarshal(raw, &fixture) != nil {
		t.Fatal("invalid job fixture")
	}
	roots := struct{ URLs map[string]string }{map[string]string{"v2": os.Getenv("THUNDER_V2_BASE_URL"), "v1": os.Getenv("THUNDER_V1_BASE_URL")}}
	for _, base := range roots.URLs {
		if !strings.HasPrefix(base, "https://") {
			t.Fatal("Explicit HTTPS base URLs required")
		}
	}
	c, err := NewClient(secret, WithBaseURLs(roots.URLs["v2"], roots.URLs["v1"]), WithMaxRetries(0))
	defer func() {
		if c != nil {
			c.Close()
		}
	}()
	if err != nil {
		t.Fatal("invalid config")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	job, err := c.Jobs.WaitForJob(ctx, fixture.Call.Args.JobID, &WaitOptions{Timeout: 20 * time.Second})
	if err != nil || job.Status != "done" {
		t.Fatal("integration wait failed")
	}
	_, err = c.Jobs.Get(ctx, "00000000-0000-0000-0000-000000000000")
	var api *ApiError
	if !errors.As(err, &api) || api.Status != 404 {
		t.Fatal("expected HTTP 404")
	}
	data, _ := json.Marshal(err)
	for _, view := range []string{err.Error(), fmt.Sprintf("%v", err), fmt.Sprintf("%+v", err), fmt.Sprintf("%#v", err), string(data)} {
		if strings.Contains(view, secret) {
			t.Fatal("credential retained")
		}
	}
	t.Log("integration wait terminal=done; error status=404; credential views safe")
}
