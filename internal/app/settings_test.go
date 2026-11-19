package app

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// The colors chosen in the settings stand over the appearance's, each column
// of the sidebar its own, and a color that makes no sense, as in a file
// edited by hand, leaves the appearance's rather than crashing the app.
func TestChosenColors(t *testing.T) {
	base := paletteFor(&ui.Theme{})
	s := defaultSettings()
	s.MainColor, s.MainText, s.IconColor = "#eae4f2", "#3a2c46", "#ff6a3d"
	s.SideColor, s.SideText, s.SideIcon = "#fffaf3", "#5c4a3a", "#3b82f6"
	p := base
	s.colors(&p)
	if p.sidebar != ui.Hex("#eae4f2") {
		t.Errorf("the categories are on %v, want the color chosen", p.sidebar)
	}
	if w := p.words(p.nav); w.text != ui.Hex("#3a2c46") || w.muted == w.text {
		t.Errorf("the categories write in %v, want the color chosen, faded below", w.text)
	}
	if !p.nav.iconsSet || p.navIcons(true) != ui.Hex("#ff6a3d") || p.navIcons(false) != ui.Hex("#ff6a3d") {
		t.Errorf("the categories' icons are %v, want the color chosen", p.nav.icons)
	}
	if p.plBg() != ui.Hex("#fffaf3") {
		t.Errorf("the playlists are on %v, want the color chosen", p.plBg())
	}
	if w := p.words(p.pl); w.text != ui.Hex("#5c4a3a") {
		t.Errorf("the playlists write in %v, want the color chosen", w.text)
	}
	if p.plIcons() != ui.Hex("#3b82f6") {
		t.Errorf("the playlists' icons are %v, want the color chosen", p.plIcons())
	}
	// One column chosen leaves the other the appearance's.
	s, p = defaultSettings(), base
	s.SideColor = "#fffaf3"
	s.colors(&p)
	if p.sidebar != base.sidebar || p.plBg() != ui.Hex("#fffaf3") {
		t.Errorf("choosing the playlists' colors took the categories' too")
	}
	for _, bad := range []string{"", "blue", "#12345", "#gggggg", "ff6a3dx"} {
		p := base
		s.MainColor, s.MainText, s.IconColor = bad, bad, bad
		s.SideColor, s.SideText, s.SideIcon = bad, bad, bad
		s.colors(&p)
		if p.sidebar != base.sidebar || p.nav.iconsSet || p.pl.bgSet || p.pl.wordsSet || p.pl.iconsSet {
			t.Errorf("%q was taken for a color", bad)
		}
	}
}
