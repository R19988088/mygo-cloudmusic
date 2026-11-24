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

// With NM_FRAMES set, saves what the sidebar and the pages of a search show:
// the section of what was found, as 我的's, the aggregate page, the pages of
// a kind with the column beside them, and an artist's and an MV's.
func TestSearchFrames(t *testing.T) {
	dir := os.Getenv("NM_FRAMES")
	if dir == "" {
		t.Skip()
	}
	a := newTestApp()
	a.settings = defaultSettings()
	a.searched = "kikuo"
	a.results = netease.SearchResults{
		Songs: []netease.Track{
			{ID: 1, Name: "愛して愛して愛して", Artists: []netease.Artist{{Name: "きくお"}, {Name: "初音ミク"}}, Duration: 267000, Album: netease.Album{Name: "きくおミク3", PicURL: "https://p1.music.126.net/a.jpg"}},
			{ID: 2, Name: "君はできない子", Artists: []netease.Artist{{Name: "きくお"}}, Duration: 268000, Album: netease.Album{Name: "きくおミク", PicURL: "https://p1.music.126.net/b.jpg"}},
			{ID: 3, Name: "ぽっかんカラー", Artists: []netease.Artist{{Name: "きくお"}, {Name: "兎眠りおん"}}, Duration: 272000, Album: netease.Album{Name: "きくおミク", PicURL: "https://p1.music.126.net/c.jpg"}},
			{ID: 4, Name: "ごめんね ごめんね", Artists: []netease.Artist{{Name: "きくお"}}, Duration: 223000, Album: netease.Album{Name: "きくおミク2", PicURL: "https://p1.music.126.net/d.jpg"}},
			{ID: 5, Name: "暗叫", Artists: []netease.Artist{{Name: "きくお"}}, Duration: 214000, Album: netease.Album{Name: "きくおミク4", PicURL: "https://p1.music.126.net/e.jpg"}},
			{ID: 6, Name: "陽差しはつかの間", Artists: []netease.Artist{{Name: "きくお"}, {Name: "si_ku"}}, Duration: 205000, Album: netease.Album{Name: "きくおミク6", PicURL: "https://p1.music.126.net/f.jpg"}},
			{ID: 7, Name: "深い森のなかで", Artists: []netease.Artist{{Name: "きくお"}, {Name: "綺萱"}}, Duration: 231000, Album: netease.Album{Name: "きくおミク6", PicURL: "https://p1.music.126.net/g.jpg"}},
		},
		Artists: []netease.SearchArtist{
			{ID: 14640, Name: "きくお", PicURL: "https://p1.music.126.net/kikuo.jpg", Alias: []string{"kikuo sound works", "菊地恵介"}, MusicSize: 318, AlbumSize: 29},
			{ID: 159692, Name: "初音ミク", PicURL: "https://p1.music.126.net/miku.jpg", MusicSize: 1200, AlbumSize: 400},
			{ID: 9001, Name: "兎眠りおん", MusicSize: 40, AlbumSize: 6},
		},
		Playlists: []netease.Playlist{
			{ID: 6710874136, Name: "kikuo", CoverURL: "https://p1.music.126.net/pl1.jpg", PlayCount: 52717, TrackCount: 93},
			{ID: 6710874137, Name: "きくお 全曲", CoverURL: "https://p1.music.126.net/pl2.jpg", PlayCount: 3210, TrackCount: 44},
			{ID: 6710874138, Name: "vocaloid 暗黑系", CoverURL: "https://p1.music.126.net/pl3.jpg", PlayCount: 991000, TrackCount: 120},
			{ID: 6710874139, Name: "作业用 邦乐", CoverURL: "https://p1.music.126.net/pl4.jpg", PlayCount: 12000, TrackCount: 60},
		},
		Albums: []netease.SavedAlbum{
			{ID: 83988815, Name: "きくおミク6", PicURL: "https://p1.music.126.net/al1.jpg", Size: 12, Artists: []netease.Artist{{Name: "きくお"}}},
			{ID: 83988816, Name: "きくおミク3", PicURL: "https://p1.music.126.net/al2.jpg", Size: 11, Artists: []netease.Artist{{Name: "きくお"}}},
			{ID: 83988817, Name: "きくおミク2", PicURL: "https://p1.music.126.net/al3.jpg", Size: 10, Artists: []netease.Artist{{Name: "きくお"}}},
			{ID: 83988818, Name: "きくおミク", PicURL: "https://p1.music.126.net/al4.jpg", Size: 9, Artists: []netease.Artist{{Name: "きくお"}}},
		},
		Podcasts: []netease.Podcast{{ID: 1229028495, Name: "Kikuo自用", PicURL: "https://p1.music.126.net/pod1.jpg", ProgramCount: 28, Category: "其他"}},
		MVs: []netease.MV{
			{ID: 14577196, Name: "しかばねの踊り", Cover: "https://p1.music.126.net/mv1.jpg", PlayCount: 153620, ArtistName: "きくお", Duration: 221000},
			{ID: 14577197, Name: "愛して愛して愛して", Cover: "https://p1.music.126.net/mv2.jpg", PlayCount: 98000, ArtistName: "きくお", Duration: 267000},
			{ID: 14577198, Name: "ごめんね ごめんね", Cover: "https://p1.music.126.net/mv3.jpg", PlayCount: 41000, ArtistName: "きくお", Duration: 223000},
		},
	}
	a.route = route{page: pageSearch}
	// The host of the podcast, and the maps their pages look their finds up
	// in.
	a.results.Podcasts[0].DJ.Nickname = "无机质的冥王星"
	a.noteFinds(&a.results)
	tt := ui.NewTester(a.view, 1280, 820)
	shot := func(name string) {
		f, _ := os.Create(filepath.Join(dir, name))
		defer f.Close()
		png.Encode(f, tt.Image())
	}
	tt.Frame()
	shot("search-all.png")
	// The column beside the categories slides open, and the entries'
	// colors settle: give them a moment.
	settle := func() {
		tt.Frame()
		tt.Advance(time.Second)
		tt.Frame()
	}
	a.chooseSearch(searchSongs)
	settle()
	shot("search-songs.png")
	a.chooseSearch(searchPlaylists)
	settle()
	shot("search-playlists.png")
	a.chooseSearch(searchArtists)
	settle()
	shot("search-artists.png")
	a.artistSongs = map[int64][]netease.Track{14640: a.results.Songs}
	a.open(route{page: pageArtist, artist: 14640})
	settle()
	shot("search-artist-page.png")
	// As wide as a listener's window may be: the page's rows run the width
	// of it, as the pages of songs do.
	wide := ui.NewTester(a.view, 1520, 820)
	wide.Frame()
	wide.Advance(time.Second)
	wide.Frame()
	f, _ := os.Create(filepath.Join(dir, "search-artist-wide.png"))
	png.Encode(f, wide.Image())
	f.Close()
	a.open(route{page: pageMV, mv: 14577196})
	settle()
	shot("search-mv-page.png")
}

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
