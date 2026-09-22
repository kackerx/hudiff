package diagram

import "testing"

// A cached SVG is only safe to reuse while the renderer that wrote it would
// still produce the same bytes. Multiboard diagrams stayed animated for a whole
// release because the key did not say which renderer wrote the file.
func TestCacheKeyChangesWithRenderer(t *testing.T) {
	before := cacheKey("a -> b", "light", false)
	old := RendererVersion
	RendererVersion = old + "-next"
	defer func() { RendererVersion = old }()
	if after := cacheKey("a -> b", "light", false); after == before {
		t.Fatal("bumping RendererVersion left the key alone: an SVG from an older renderer would still be served")
	}
}

func TestCacheKeyCoversEveryInput(t *testing.T) {
	base := cacheKey("a -> b", "light", false)
	for name, got := range map[string]string{
		"source": cacheKey("a -> c", "light", false),
		"theme":  cacheKey("a -> b", "dark", false),
		"sketch": cacheKey("a -> b", "light", true),
	} {
		if got == base {
			t.Errorf("%s does not reach the cache key", name)
		}
	}
}

// boardSuffix keeps one file per board; board 0 has to stay unsuffixed so the
// page's plain request and its explicit board=0 hit the same file.
func TestBoardSuffix(t *testing.T) {
	if boardSuffix(0) != "" || boardSuffix(-1) != "" {
		t.Error("board 0 must be the unsuffixed file")
	}
	if boardSuffix(1) == boardSuffix(2) {
		t.Error("boards need distinct files")
	}
}
