package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// settings are the listener's choices, kept in settings.json in the user
// data directory.
type settings struct {
	// Notch shows the quick player around the notch: the listener turns
	// it on.
	Notch bool `json:"notch"`
	// NotchActivity shows the cover and the music's levels beside the
	// notch while music plays, which hides the menu bar there.
	NotchActivity bool `json:"notchActivity"`
	// NotchVirtual keeps a notch of the app's own showing at the top of a
	// display without one; else it shows only while the pointer asks.
	NotchVirtual bool `json:"notchVirtual"`
	// NotchGreeted says the notch showed itself, once, as the first song
	// played (greet).
	NotchGreeted bool `json:"notchGreeted"`
	// Quality is the level of the streams: standard, higher, exhigh,
	// lossless or hires.
	Quality string `json:"quality"`
	// Appearance is light, dark, or system to follow the system.
	Appearance string `json:"appearance"`
	// MainColor, MainText and IconColor are the colors of the categories'
	// column as #rrggbb — the background it lays on, its words and its icons
	// — and SideColor, SideText and SideIcon the colors of the playlists'
	// column beside it: the listener's own, where they chose them. Empty
	// follows the appearance.
	MainColor string `json:"mainColor"`
	MainText  string `json:"mainText"`
	IconColor string `json:"iconColor"`
	SideColor string `json:"sideColor"`
	SideText  string `json:"sideText"`
	SideIcon  string `json:"sideIcon"`
	// Smoke makes the progress bars comets trailing smoke as music plays,
	// for a few percent of a core: the listener turns it on.
	Smoke bool `json:"smoke"`
	// LyricsStage shows lyrics on the stage, Folia's style, in Now
	// Playing, rather than scrolling.
	LyricsStage bool `json:"lyricsStage"`
	// Volume is where the volume was left, from 0 to 1.
	Volume float64 `json:"volume"`
	// Normalize evens out the loudness of tracks, by what NetEase
	// measured of each.
	Normalize bool `json:"normalize"`
	// HotComments shows a song's hot comments in its quiet parts, in
	// place of the lyrics (quietComment): the listener turns it on.
	HotComments bool `json:"hotComments"`
	// Fog fills Now Playing's backdrop with fog of the cover's color,
	// rising and swelling with the bass: an experiment the listener turns
	// on.
	Fog bool `json:"fog"`
	// CacheMB is how many megabytes the songs kept on disk may take; 0
	// keeps none.
	CacheMB int `json:"cacheMB"`
	// SideMine is the list of 我的 the column of playlists beside the
	// categories showed for the last song played: the listener's playlists
	// (playlists), their saved ones (saved), the albums they saved (albums),
	// or the podcasts they saved (podcasts); empty when it came from none of
	// them, and so nothing shows beside the categories.
	SideMine string `json:"sideMine"`
	// SidePlaylist is the playlist, album, or podcast that list held the last
	// song played in, whose page shows beside the column on the next launch.
	SidePlaylist int64 `json:"sidePlaylist"`
	// WindowWidth and WindowHeight are how big the window was left, and
	// WindowX and WindowY where, for the next launch; 0 for the size it
	// opens at, in the middle of the display.
	WindowX      int `json:"windowX"`
	WindowY      int `json:"windowY"`
	WindowWidth  int `json:"windowWidth"`
	WindowHeight int `json:"windowHeight"`

	path string // where they were loaded from, and are saved to
}

func defaultSettings() settings {
	return settings{NotchActivity: true, Quality: "exhigh", Appearance: "system", Volume: 0.8, Normalize: true, CacheMB: 2048}
}

// sideMine returns the list the column of playlists showed for the last
// song played, and whether it showed one at all.
func (s settings) sideMine() (mineKind, bool) {
	switch s.SideMine {
	case "playlists":
		return minePlaylists, true
	case "saved":
		return mineSaved, true
	case "podcasts":
		return minePodcasts, true
	case "albums":
		return mineAlbums, true
	}
	return minePlaylists, false
}

func settingsPath() (string, error) {
	dir, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "settings.json"), nil
}

// oldName is what the app was called before it took its bundle's name, and
// kept its data under.
const oldName = "网易云音乐"

// moveOldData moves the user data and the song cache kept under oldName to
// the directories of the app's name, unless those hold something already:
// the login, settings and where playback was left go on. It must run before
// anything reads them.
func moveOldData() {
	for _, name := range []mygo.PathName{mygo.PathUserData, mygo.PathCache} {
		dir, err := mygo.App.Path(name)
		if err != nil {
			continue
		}
		moveDir(filepath.Join(filepath.Dir(dir), oldName), dir)
	}
}

// moveDir moves the directory old to dir when dir is empty, as Path made
// it, and old exists.
func moveDir(old, dir string) {
	if old == dir {
		return
	}
	if entries, err := os.ReadDir(dir); err != nil || len(entries) > 0 {
		return
	}
	if _, err := os.Stat(old); err != nil {
		return
	}
	if os.Remove(dir) == nil {
		if err := os.Rename(old, dir); err != nil {
			os.MkdirAll(dir, 0o700)
		}
	}
}

// loadSettings reads the settings, or returns the defaults.
func loadSettings() settings {
	s := defaultSettings()
	if path, err := settingsPath(); err == nil {
		if b, err := os.ReadFile(path); err == nil {
			json.Unmarshal(b, &s)
		}
		s.path = path
	}
	return s
}

// save keeps the settings where they were loaded from; defaults that were
// never loaded, as in tests, are not saved.
func (s settings) save() error {
	path := s.path
	if path == "" {
		return nil
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o600)
}

// applyAppearance makes the windows light, dark, or follow the system.
func (s settings) applyAppearance() {
	switch s.Appearance {
	case "light":
		mygo.Theme.SetSource(mygo.ThemeLight)
	case "dark":
		mygo.Theme.SetSource(mygo.ThemeDark)
	default:
		mygo.Theme.SetSource(mygo.ThemeSystem)
	}
}

// colors puts the colors the listener chose over the appearance's: those of
// the categories' column, and of the playlists' beside it.
func (s settings) colors(p *palette) {
	if c, ok := colorOf(s.MainColor); ok {
		p.sidebar, p.nav.bg, p.nav.bgSet = c, c, true
	}
	if c, ok := colorOf(s.MainText); ok {
		p.nav.words, p.nav.wordsSet = c, true
	}
	if c, ok := colorOf(s.IconColor); ok {
		p.nav.icons, p.nav.iconsSet = c, true
	}
	if c, ok := colorOf(s.SideColor); ok {
		p.pl.bg, p.pl.bgSet = c, true
	}
	if c, ok := colorOf(s.SideText); ok {
		p.pl.words, p.pl.wordsSet = c, true
	}
	if c, ok := colorOf(s.SideIcon); ok {
		p.pl.icons, p.pl.iconsSet = c, true
	}
}

// colorOf returns the color named as hex, and whether it names one: one that
// does not parse, as a file edited by hand, follows the appearance rather
// than the panic ui.Hex makes of it.
func colorOf(hex string) (ui.Color, bool) {
	h := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(hex), "#"))
	if len(h) != 3 && len(h) != 4 && len(h) != 6 && len(h) != 8 {
		return ui.Color{}, false
	}
	for i := range len(h) {
		if c := h[i]; !('0' <= c && c <= '9' || 'a' <= c && c <= 'f') {
			return ui.Color{}, false
		}
	}
	return ui.Hex(h), true
}

// hexOf is the color named as the settings keep one, #rrggbb.
func hexOf(c ui.Color) string {
	return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B)
}

// qualityLevel is the stream level NetEase names a quality by.
var qualityLevels = []struct{ level, label string }{
	{"standard", "标准（128k）"},
	{"higher", "较高（192k）"},
	{"exhigh", "极高（320k）"},
	{"lossless", "无损（FLAC）"},
	{"hires", "Hi-Res"},
}

// cacheLimits are the sizes the song cache may take, in megabytes.
var cacheLimits = []struct {
	mb    int
	label string
}{
	{0, "关闭"},
	{1024, "1 GB"},
	{2048, "2 GB"},
	{5120, "5 GB"},
	{10240, "10 GB"},
}
