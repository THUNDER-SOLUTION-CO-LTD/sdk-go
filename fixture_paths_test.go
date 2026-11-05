package thunder

import (
	"os"
	"path/filepath"
)

func fixturePath(path string) string {
	root := os.Getenv("THUNDER_FIXTURES_DIR")
	if root == "" {
		root = "fixtures"
	}
	return filepath.Join(root, path)
}
