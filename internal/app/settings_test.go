package app

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// The colors chosen in the settings stand over the appearance's, and a color
// that makes no sense, as in a file edited by hand, leaves the appearance's
// rather than crashing the app.
func TestChosenColors(t *testing.T) {
	base := paletteFor(&ui.Theme{})
	s := defaultSettings()
	s.MainColor, s.IconColor = "#eef3f8", "#FF6A3D"
	p := base
	s.colors(&p)
	if p.bg != ui.Hex("#eef3f8") {
		t.Errorf("the pages are %v, want the color chosen", p.bg)
	}
	if !p.iconSet || p.navIcon(true) != ui.Hex("#ff6a3d") || p.navIcon(false) != ui.Hex("#ff6a3d") {
		t.Errorf("the icons are %v, want the color chosen", p.icon)
	}
	if hexOf(p.icon) != "#ff6a3d" {
		t.Errorf("the color kept as %s, want #ff6a3d", hexOf(p.icon))
	}
	for _, bad := range []string{"", "blue", "#12345", "#gggggg", "ff6a3dx"} {
		p := base
		s.MainColor, s.IconColor = bad, bad
		s.colors(&p)
		if p.bg != base.bg || p.iconSet {
			t.Errorf("%q was taken for a color", bad)
		}
	}
}
