// Assets are the embedded web/ files. HUDIFF_DEV=1 reads them from disk so a
// running daemon never serves a stale page (see specs/handoff, 15:21 incident).
package daemon

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var (
	embedded fs.FS
	devDir   string
	once     sync.Once
)

func SetAssets(f fs.FS) { embedded = f }

func dev() string {
	once.Do(func() {
		if os.Getenv("HUDIFF_DEV") != "1" {
			return
		}
		devDir = os.Getenv("HUDIFF_WEB_DIR")
		if devDir == "" {
			cwd, _ := os.Getwd()
			devDir = filepath.Join(cwd, "web")
		}
	})
	return devDir
}

func asset(name string) []byte {
	if d := dev(); d != "" {
		if b, err := os.ReadFile(filepath.Join(d, name)); err == nil {
			return b
		}
	}
	b, err := fs.ReadFile(embedded, "web/"+name)
	if err != nil {
		return nil
	}
	return b
}

// Page inlines dlg.html the way the oracle's page() did.
func page(name string) []byte {
	html := string(asset(name))
	return []byte(strings.ReplaceAll(html, "<!--DLG-->", string(asset("dlg.html"))))
}
