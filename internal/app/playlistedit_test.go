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

// editTestApp is signed in, with liked songs, two playlists of their own
// and one saved, and shows their playlist 10.
func editTestApp() *app {
	a := newTestApp()
	a.settings = defaultSettings()
	a.profile = &netease.Profile{UserID: 1, Nickname: "小明"}
	a.mine = []netease.Playlist{
		{ID: 5, Name: "小明喜欢的音乐", SpecialType: 5},
		{ID: 10, Name: "通勤"},
		{ID: 11, Name: "睡前"},
	}
	a.saved = []netease.Playlist{{ID: 20, Name: "飙升榜"}}
	a.playlists[10] = &netease.Playlist{ID: 10, Name: "通勤", Tracks: []netease.Track{
		{ID: 1, Name: "一张相片", Artists: []netease.Artist{{Name: "林宥嘉"}}, Duration: 200000},
		{ID: 2, Name: "宝船", Artists: []netease.Artist{{Name: "DOUDOU"}}, Duration: 200000},
	}}
	a.route = route{page: pagePlaylist, playlist: 10}
	// 歌单 is chosen, which shows the sidebar's column of playlists.
	a.mineOn = true
	return a
}

// Choosing one of the listener's playlists from a song's menu adds the
// song to it, and the menu offers neither the liked songs nor the
// playlist the song is listed in.
func TestAddSongToPlaylist(t *testing.T) {
	type add struct{ pid, song int64 }
	calls := make(chan add, 1)
	addToPlaylist = func(pid int64, songs ...int64) error { calls <- add{pid, songs[0]}; return nil }
	defer func() { addToPlaylist = netease.AddToPlaylist }()

	a := editTestApp()
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	if err := tt.RightClick("宝船"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("添加到歌单", "通勤"); err == nil {
		t.Error("offered to add the song to the playlist it is in")
	}
	if err := tt.ChooseMenuItem("添加到歌单", "小明喜欢的音乐"); err == nil {
		t.Error("offered to add the song to the liked songs")
	}
	if err := tt.ChooseMenuItem("添加到歌单", "睡前"); err != nil {
		t.Fatalf("%v; the menu shows %q", err, tt.Menu())
	}
	tt.Frame()
	if got := <-calls; got != (add{11, 2}) {
		t.Errorf("added %+v, want song 2 to playlist 11", got)
	}
}

// Removing a song from the listener's playlist takes its row away at once,
// and asks NetEase to take it out.
func TestRemoveSongFromPlaylist(t *testing.T) {
	type del struct{ pid, song int64 }
	calls := make(chan del, 1)
	removeFromPlaylist = func(pid int64, songs ...int64) error { calls <- del{pid, songs[0]}; return nil }
	defer func() { removeFromPlaylist = netease.RemoveFromPlaylist }()

	a := editTestApp()
	// Playing the playlist: the queue keeps the song.
	a.pl.Cue(a.playlists[10].Tracks)
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	if err := tt.RightClick("宝船"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("从歌单中移除"); err != nil {
		t.Fatalf("%v; the menu shows %q", err, tt.Menu())
	}
	tt.Frame()
	if got := <-calls; got != (del{10, 2}) {
		t.Errorf("removed %+v, want song 2 from playlist 10", got)
	}
	if _, ok := tt.Find("宝船"); ok {
		t.Error("the song's row is still listed")
	}
	if q := a.pl.Queue(); len(q) != 2 || q[1].ID != 2 {
		t.Errorf("removing the song changed the queue playing: %v", q)
	}
}

// A playlist the listener saved, not made, offers no removing.
func TestNoRemovingFromSavedPlaylist(t *testing.T) {
	a := editTestApp()
	a.playlists[20] = &netease.Playlist{ID: 20, Name: "飙升榜", Tracks: a.playlists[10].Tracks}
	a.route = route{page: pagePlaylist, playlist: 20}
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	if err := tt.RightClick("宝船"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("从歌单中移除"); err == nil {
		t.Error("offered to remove a song from a saved playlist")
	}
}

// The sidebar's plus asks for a name, and Enter makes the playlist so
// named.
func TestCreatePlaylist(t *testing.T) {
	names := make(chan string, 1)
	createPlaylist = func(name string) (*netease.Playlist, error) {
		names <- name
		return &netease.Playlist{ID: 12, Name: name}, nil
	}
	defer func() { createPlaylist = netease.CreatePlaylist }()

	a := editTestApp()
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	if err := tt.Click("新建歌单"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(300 * time.Millisecond)
	tt.Frame()
	tt.Type("  周末  ")
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if got := <-names; got != "周末" {
		t.Errorf("made a playlist named %q, want 周末", got)
	}
}

// Choosing 新建歌单… from a song's menu makes the playlist, then adds the
// song to it.
func TestCreatePlaylistForSong(t *testing.T) {
	a := editTestApp()
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	if err := tt.RightClick("宝船"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("添加到歌单", "新建歌单…"); err != nil {
		t.Fatalf("%v; the menu shows %q", err, tt.Menu())
	}
	tt.Frame()
	if !a.naming.open || a.naming.add == nil || a.naming.add.ID != 2 {
		t.Fatalf("the dialog is %+v, want it open to add song 2", a.naming)
	}
}

// Renaming a playlist from the sidebar's menu shows its new name at once,
// and asks NetEase to rename it.
func TestRenamePlaylist(t *testing.T) {
	type rename struct {
		id   int64
		name string
	}
	calls := make(chan rename, 1)
	renamePlaylist = func(id int64, name string) error { calls <- rename{id, name}; return nil }
	defer func() { renamePlaylist = netease.RenamePlaylist }()

	a := editTestApp()
	a.route = route{page: pageHome}
	a.homeLoading = true
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	if err := tt.RightClick("睡前"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("重命名…"); err != nil {
		t.Fatalf("%v; the menu shows %q", err, tt.Menu())
	}
	tt.Advance(300 * time.Millisecond)
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("助眠")
	if err := tt.Click("保存"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if got := <-calls; got != (rename{11, "助眠"}) {
		t.Errorf("renamed %+v, want playlist 11 to 助眠", got)
	}
	if a.mine[2].Name != "助眠" {
		t.Errorf("the sidebar names it %q", a.mine[2].Name)
	}
	if _, ok := tt.Find("助眠"); !ok {
		t.Error("the new name does not show")
	}
}

// The liked songs can't be renamed: they offer no menu.
func TestNoRenamingLikedSongs(t *testing.T) {
	a := editTestApp()
	a.route = route{page: pageHome}
	a.homeLoading = true
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	tt.RightClick("小明喜欢的音乐")
	if err := tt.ChooseMenuItem("重命名…"); err == nil {
		t.Error("offered to rename the liked songs")
	}
}

// Deleting the playlist shown, once confirmed, takes it from the sidebar
// and goes back to the page before, with no way forward to it.
func TestDeletePlaylist(t *testing.T) {
	ids := make(chan int64, 1)
	deletePlaylist = func(id int64) error { ids <- id; return nil }
	defer func() { deletePlaylist = netease.DeletePlaylist }()

	a := editTestApp()
	a.homeLoading = true
	a.route = route{page: pageCharts}
	a.open(route{page: pagePlaylist, playlist: 10})
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	if err := tt.Click("更多"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("删除歌单…"); err != nil {
		t.Fatalf("%v; the menu shows %q", err, tt.Menu())
	}
	tt.Advance(300 * time.Millisecond)
	tt.Frame()
	select {
	case <-ids:
		t.Fatal("deleted before it was confirmed")
	default:
	}
	if err := tt.Click("删除"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if got := <-ids; got != 10 {
		t.Errorf("deleted playlist %d, want 10", got)
	}
	if a.editable(10) != nil {
		t.Error("the sidebar still lists it")
	}
	if a.route.page != pageCharts || len(a.history) != 0 || len(a.forward) != 0 {
		t.Errorf("on %+v, history %v, forward %v; want the charts with nowhere to go", a.route, a.history, a.forward)
	}
}

// Choosing 收藏 under 我的 lists the saved playlists in the column of
// playlists instead of the listener's own, and 播客, which has no list yet,
// says so. Closing 我的 hides its entries.
func TestMineSectionSwitchesTheColumn(t *testing.T) {
	a := editTestApp()
	a.homeLoading = true
	a.route = route{page: pageHome}
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	if _, ok := tt.Find("飙升榜"); ok {
		t.Fatal("the saved playlist shows before 收藏 is chosen")
	}
	if err := tt.Click("收藏"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if _, ok := tt.Find("飙升榜"); !ok {
		t.Error("收藏 does not list the saved playlists")
	}
	if _, ok := tt.Find("通勤"); ok {
		t.Error("收藏 still lists the listener's playlists")
	}
	if err := tt.Click("播客"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("播客还没有接入") {
		t.Error("播客 does not say it has no list")
	}
	if err := tt.Click("我的"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if _, ok := tt.Find("收藏"); ok {
		t.Error("我的 lists its entries once closed")
	}
}

// Choosing a page closes the column of playlists beside the categories,
// however it was opened, and choosing one of 我的 opens it again; a
// playlist opened from the column leaves it open.
func TestPlaylistColumnFollowsTheChoice(t *testing.T) {
	a := editTestApp()
	a.homeLoading = true
	a.route = route{page: pageHome}
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	if a.plW < playlistWidth-1 {
		t.Fatalf("the column is %.1f wide, want it open", a.plW)
	}
	if err := tt.Click("为你推荐"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(400 * time.Millisecond)
	tt.Frame()
	if a.plW > 1 {
		t.Errorf("the column is %.1f wide after a page was chosen, want it closed", a.plW)
	}
	if _, ok := tt.Find("睡前"); ok {
		t.Error("the column still lists playlists while closed")
	}
	if err := tt.Click("歌单"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(400 * time.Millisecond)
	tt.Frame()
	if a.plW < playlistWidth-1 {
		t.Errorf("the column is %.1f wide after 歌单 was chosen, want it open", a.plW)
	}
	if err := tt.Click("睡前"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(400 * time.Millisecond)
	tt.Frame()
	if a.plW < playlistWidth-1 {
		t.Errorf("the column is %.1f wide after a playlist in it was opened, want it open", a.plW)
	}
	if a.route.page != pagePlaylist || a.route.playlist != 11 {
		t.Errorf("opening 睡前 went to %+v", a.route)
	}
}

// Playing a song from one of the listener's playlists remembers 歌单 for
// the next launch, and one from a playlist they saved remembers 收藏; a
// song from a page that lists neither remembers nothing, which leaves the
// column shut.
func TestPlayingRemembersTheColumn(t *testing.T) {
	a := editTestApp()
	a.playlists[20] = &netease.Playlist{ID: 20, Name: "飙升榜", Tracks: []netease.Track{
		{ID: 3, Name: "起风了", Artists: []netease.Artist{{Name: "买辣椒也用券"}}, Duration: 200000},
	}}
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	if err := tt.Click("宝船"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.settings.SideMine != "playlists" {
		t.Errorf("playing from a playlist remembers %q, want playlists", a.settings.SideMine)
	}
	if a.settings.SidePlaylist != 10 {
		t.Errorf("playing from playlist 10 remembers %d", a.settings.SidePlaylist)
	}
	a.route = route{page: pagePlaylist, playlist: 20}
	tt.Frame()
	if err := tt.Click("起风了"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.settings.SideMine != "saved" {
		t.Errorf("playing from a saved playlist remembers %q, want saved", a.settings.SideMine)
	}
	if a.settings.SidePlaylist != 20 {
		t.Errorf("playing from playlist 20 remembers %d", a.settings.SidePlaylist)
	}
	a.route = route{page: pageCharts}
	a.noteSource()
	if a.settings.SideMine != "" || a.settings.SidePlaylist != 0 {
		t.Errorf("playing from the charts remembers %q %d, want none", a.settings.SideMine, a.settings.SidePlaylist)
	}
	// What was kept is what the app opens on: that list beside the
	// categories, and the page of its playlist rather than the home page.
	if kind, ok := (settings{SideMine: "saved"}).sideMine(); !ok || kind != mineSaved {
		t.Errorf("a kept saved list restores %v, %v", kind, ok)
	}
	fresh := editTestApp()
	fresh.route = route{page: pageHome}
	fresh.settings = settings{SideMine: "saved", SidePlaylist: 20}
	fresh.restoreSide()
	if !fresh.mineOn || fresh.mineShown != mineSaved {
		t.Errorf("the column opens on %v, %v", fresh.mineShown, fresh.mineOn)
	}
	if fresh.route.page != pagePlaylist || fresh.route.playlist != 20 {
		t.Errorf("the app opens on %+v, want playlist 20", fresh.route)
	}
	fresh.settings = settings{}
	fresh.route = route{page: pageHome}
	fresh.restoreSide()
	if fresh.mineOn {
		t.Error("nothing kept still opens the column")
	}
	if fresh.route.page != pageHome {
		t.Errorf("nothing kept opens %+v, want the home page", fresh.route)
	}
	// A list without the playlist it held, as the settings of an older app
	// keep, shows neither the column nor a page: not that list beside the
	// home page.
	half := editTestApp()
	half.settings = settings{SideMine: "playlists"}
	half.route = route{page: pageHome}
	half.restoreSide()
	if half.mineOn || half.route.page != pageHome {
		t.Errorf("half a memory opens %+v with the column %v", half.route, half.mineOn)
	}
}

// Cancelling the confirmation deletes nothing.
func TestCancelDeletingPlaylist(t *testing.T) {
	deletePlaylist = func(int64) error { t.Error("deleted the playlist"); return nil }
	defer func() { deletePlaylist = netease.DeletePlaylist }()

	a := editTestApp()
	a.route = route{page: pageHome}
	a.homeLoading = true
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	if err := tt.RightClick("睡前"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("删除歌单…"); err != nil {
		t.Fatalf("%v; the menu shows %q", err, tt.Menu())
	}
	tt.Advance(300 * time.Millisecond)
	tt.Frame()
	if err := tt.Click("取消"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(300 * time.Millisecond)
	tt.Frame()
	if a.editable(11) == nil {
		t.Error("the playlist left the sidebar")
	}
}

// With NM_FRAMES set, saves the listener's playlist with its sidebar, and
// the dialogs that name and delete playlists, to look at.
func TestPlaylistEditFrames(t *testing.T) {
	dir := os.Getenv("NM_FRAMES")
	if dir == "" {
		t.Skip()
	}
	a := editTestApp()
	tt := ui.NewTester(a.view, 1200, 800)
	save := func(name string) {
		tt.Advance(400 * time.Millisecond)
		tt.Frame()
		f, _ := os.Create(filepath.Join(dir, name+".png"))
		defer f.Close()
		png.Encode(f, tt.Image())
	}
	for _, dark := range []bool{false, true} {
		suffix := "-light"
		if dark {
			suffix = "-dark"
		}
		tt.SetDark(dark)
		save("playlist" + suffix)
		a.newPlaylist(nil)
		save("create" + suffix)
		a.naming.open = false
		a.startRename(a.editable(11))
		save("rename" + suffix)
		a.naming.open = false
		a.confirmDelete(a.editable(11))
		save("delete" + suffix)
		a.deleting.open = false
	}
}
