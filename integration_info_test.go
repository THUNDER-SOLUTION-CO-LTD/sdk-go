package thunder

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestAccountInfoIntegration(t *testing.T) {
	if os.Getenv("THUNDER_READ_INTEGRATION") != "1" {
		t.Skip("Read-only integration is not enabled")
	}
	c, err := NewClient("")
	if err != nil {
		t.Fatal("client configuration")
	}
	defer c.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if _, err = c.GetInfo(ctx); err != nil {
		t.Fatalf("account info failed (%T)", err)
	}
}
