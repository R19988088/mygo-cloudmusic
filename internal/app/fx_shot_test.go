package app

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/netease"
)

// With NM_FRAMES set, saves the effects popover open, light and dark.
func TestEffectsPopoverFrames(t *testing.T) {
	dir := os.Getenv("NM_FRAMES")
	if dir == "" {
		t.Skip()
	}
	for _, dark := range []bool{false, true} {
		a := newTestApp()
		a.settings = defaultSettings()
		tr := netease.Track{ID: 7, Name: "春日影", Artists: []netease.Artist{{Name: "MyGO!!!!!"}}, Duration: 199000}
		a.pl.Cue([]netease.Track{tr})
		tt := ui.NewTester(a.view, 900, 620)
		tt.SetDark(dark)
		tt.Frame()
		if _, ok := tt.Find("音效"); !ok {
			t.Fatal("no effects button")
		}
		tt.Click("音效")
		tt.Advance(time.Second)
		tt.Frame()
		if !a.fxOpen || !tt.HasText("嘻哈音效") {
			t.Fatalf("popover not open: %v %q", a.fxOpen, tt.Texts())
		}
		// A preset picked turns the effects on and stays chosen, even though
		// the switch started off.
		if err := tt.Click("清澈人声"); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
		if !a.settings.EffectsOn || a.settings.Effect != "清澈人声" {
			t.Fatalf("not picked: on %v, effect %q", a.settings.EffectsOn, a.settings.Effect)
		}
		tt.Advance(400 * time.Millisecond)
		tt.Frame()
		name := "effects-light.png"
		if dark {
			name = "effects-dark.png"
		}
		f, _ := os.Create(filepath.Join(dir, name))
		png.Encode(f, tt.Image())
		f.Close()
	}
}
