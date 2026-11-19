package app

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
)

// Clicking the hint of the search field focuses it, and what is typed goes
// to the query: the field draws its own hint, so it must not take the press.
func TestSearchFieldTakesTheClick(t *testing.T) {
	a := newTestApp()
	tt := ui.NewTester(a.view, windowWidth, windowHeight)
	tt.Frame()
	hint, ok := tt.Find("搜索歌曲、歌手")
	if !ok {
		t.Fatal("no hint in the search field")
	}
	tt.ClickAt(hint.X+hint.W/2, hint.Y+hint.H/2)
	tt.Frame()
	tt.Type("晴天")
	tt.Frame()
	if a.query != "晴天" {
		t.Errorf("typed %q into the field, it holds %q", "晴天", a.query)
	}
}

// With NM_FRAMES set, saves the sidebar's search field in a color of the
// listener's, the hint and the field focused: the hint is the column's text
// color and the field keeps it readable once focused.
func TestSearchFieldFrames(t *testing.T) {
	dir := os.Getenv("NM_FRAMES")
	if dir == "" {
		t.Skip()
	}
	a := newTestApp()
	a.settings.MainColor, a.settings.MainText = "#c14657", "#ffffff"
	tt := ui.NewTester(a.view, windowWidth, windowHeight)
	shot := func(name string) {
		f, _ := os.Create(filepath.Join(dir, name))
		png.Encode(f, tt.Image())
		f.Close()
	}
	tt.Frame()
	shot("search-hint.png")
	hint, ok := tt.Find("搜索歌曲、歌手")
	if !ok {
		t.Fatal("no hint in the search field")
	}
	tt.SetFocused(true)
	tt.ClickAt(hint.X+hint.W/2, hint.Y+hint.H/2)
	tt.Frame()
	shot("search-focused.png")
}
