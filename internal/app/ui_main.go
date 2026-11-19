package app

import (
	"runtime"
	"strings"
	"time"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/netease"
)

// The sidebar is two columns, as Music's All Playlists: navWidth for the
// categories and their entries, playlistWidth for the playlists beside
// them, which shows only while one of 我的 is chosen. sidebarWidth is the
// widest it gets, the two together.
const (
	navWidth      = 188
	playlistWidth = 224
	sidebarWidth  = navWidth + playlistWidth
)

func (a *app) view(c *ui.Context) {
	t := *c.Theme()
	a.pal = paletteFor(&t)
	p := a.pal
	t.Accent, t.AccentText = p.accent, p.onAccent
	t.Selection = p.accent.Alpha(0.3)
	c.SetTheme(&t)

	a.shortcuts(c)

	if c.Vibrancy() {
		c.Root().Background(ui.Transparent)
	}
	root := ui.Box(c).Fill()
	if !c.Focused(a.searchBox) {
		// Space plays and pauses wherever the focus is, as in Music. On
		// the container around everything, it comes before a button the
		// pointer focused, which would take Space as a click; on the
		// window, it would come after.
		root.OnShortcut(0, ui.KeySpace, a.pl.TogglePlay)
		// And where nothing has the focus, as once Now Playing covers the
		// button that had it.
		c.OnShortcut(0, ui.KeySpace, a.pl.TogglePlay)
	}
	npOpen, npSettled := a.npState(c)
	root.Children(func() {
		page := ui.Row(c).Fill().AlignItems(ui.Stretch)
		page.Children(func() {
			if npOpen && npSettled {
				// Covered by Now Playing: not built, which would cost each
				// of its frames as much again as the stage itself. Where
				// it is scrolled is the app's (pageScroll, navScroll,
				// trackList), so it shows again as it was.
				return
			}
			a.sidebar(c)
			// Keyed, so that a popover of the sidebar opening or closing
			// keeps what the page and its bars hold, their edges' fades.
			ui.Box(c.Key("main")).Grow(1).Background(p.bg).BorderWidth(0, 0, 0, 1).BorderColor(p.hairline).Children(func() {
				a.content(c)
				// The bars float over the page, which scrolls under them:
				// positioned, they paint over it.
				a.pageTop(c)
				a.playerBar(c)
			})
		})
		a.nowPlayingLayer(c, npOpen, npSettled)
	})
	a.loginDialog(c)
	a.playlistNameDialog(c)
	a.deletePlaylistDialog(c)
	if a.toast != "" {
		c.Toast(a.toast)
		a.toast = ""
	}
	a.syncWindows()
}

func (a *app) shortcuts(c *ui.Context) {
	// The side buttons of a mouse, the keys of keyboards that have them
	// and swipes between pages, and the keys of Music.
	c.OnShortcut(0, ui.KeyBack, a.back)
	c.OnShortcut(0, ui.KeyForward, a.goForward)
	c.OnShortcut(ui.Cmd, ui.KeyBracketLeft, a.back)
	c.OnShortcut(ui.Cmd, ui.KeyBracketRight, a.goForward)
	c.OnShortcut(ui.Cmd, ui.KeyF, func() {
		a.nowPlaying = false
		a.searchBox.Focus(c)
	})
	if a.nowPlaying {
		c.OnShortcut(0, ui.KeyEscape, func() { a.nowPlaying = false })
	}
}

// go opens a page, remembering the one before for the back button.
func (a *app) open(r route) {
	if r == a.route {
		return
	}
	a.history, a.forward = append(a.history, a.route), nil
	a.nowPlaying = false
	a.show(r)
}

// show shows the page of r.
func (a *app) show(r route) {
	a.route = r
	a.trackList, a.pageScroll = ui.ListState{}, ui.ScrollState{}
	if r.page == pagePlaylist {
		a.loadPlaylist(r.playlist, false)
	}
}

// back goes to the page before, as the back button of a mouse does, or
// closes Now Playing, which covers the pages.
func (a *app) back() {
	if a.nowPlaying {
		a.nowPlaying = false
		return
	}
	if len(a.history) == 0 {
		return
	}
	a.forward = append(a.forward, a.route)
	r := a.history[len(a.history)-1]
	a.history = a.history[:len(a.history)-1]
	a.show(r)
}

// goForward goes to the page back left, as the forward button of a mouse
// does.
func (a *app) goForward() {
	if len(a.forward) == 0 {
		return
	}
	a.history = append(a.history, a.route)
	r := a.forward[len(a.forward)-1]
	a.forward = a.forward[:len(a.forward)-1]
	a.nowPlaying = false
	a.show(r)
}

// sidebar lays the left of the window as two columns: the categories and
// their entries, and, while one of 我的 is chosen, the playlists beside
// them, with their covers.
func (a *app) sidebar(c *ui.Context) {
	p := a.pal
	nav := ui.Column(c).Width(navWidth).DragWindow()
	// Vibrancy is macOS's: on Windows the material behind it shows through
	// where the column pads its top as a dark strip, so it paints its own
	// color there.
	if runtime.GOOS != "darwin" || !c.Vibrancy() {
		nav.Background(p.sidebar)
	}
	nav.Children(func() { a.navColumn(c) })
	a.playlistColumn(c)
}

// columnsTop is how far the sidebar's columns keep from the top: under the
// window buttons where they sit over them, as the traffic lights do;
// elsewhere, as close to the top as to the sides.
func columnsTop(c *ui.Context) float32 {
	if c.TitleBar().Left > 0 {
		return topBar(c) + 4
	}
	return 14
}

// contentWidth is the room the pages have right of the sidebar: the window
// less the categories, and the playlists' column as wide as it has opened.
func (a *app) contentWidth(c *ui.Context) float32 {
	w, _ := c.Size()
	return w - navWidth - a.plW
}

// navColumn is the sidebar's categories and their entries.
func (a *app) navColumn(c *ui.Context) {
	p := a.pal
	// Where nothing in it takes the press, the column moves the window, as
	// the title bar it reaches up into would.
	ui.Column(c).Fill().Padding(columnsTop(c), 10, 10, 10).Gap(2).DragWindow().Children(func() {
		// The search field.
		field := ui.Row(c).Height(30).PaddingX(9).Gap(6).Radius(8).AlignItems(ui.Center).Margin(0, 0, 12, 4)
		focused := c.Focused(a.searchBox)
		if focused {
			field.Background(p.bg).Shadow(0, 0, 0, 3, p.accent.Alpha(0.25)).Border(1, p.accent.Alpha(0.6))
		} else {
			field.Background(p.hover).Border(1, p.hairline)
		}
		field.Children(func() {
			ui.Icon(c, iconSearch).Size(13, 13).TextColor(p.muted)
			in := ui.TextInputBase(c.Key("search"), &a.query).Bind(&a.searchBox).Grow(1).
				Placeholder("搜索歌曲、歌手").FontSize(12.5).Label("搜索")
			if in.Submitted() && strings.TrimSpace(a.query) != "" {
				a.search(strings.TrimSpace(a.query))
			}
		})

		ui.Scroll(c.Key("nav")).TrackScroll(&a.navScroll).Grow(1).Margin(0, -6).Padding(0, 6).Gap(2).Children(func() {
			a.navHeader(c, "浏览")
			a.navItem(c, iconHome, "为你推荐", route{page: pageHome})
			a.navItem(c, iconDaily, "每日推荐", route{page: pageDaily})
			a.navItem(c, iconRadio, "私人 FM", route{page: pageFM})
			a.navItem(c, iconCharts, "排行榜", route{page: pageCharts})
			if a.searched != "" {
				a.navItem(c, iconSearch, "搜索结果", route{page: pageSearch})
			}
			a.navGroup(c, "我的", &a.mineClosed, func() {
				a.mineItem(c, iconNote, "歌单", minePlaylists)
				a.mineItem(c, iconMic, "播客", minePodcasts)
				a.mineItem(c, iconHeart, "收藏", mineSaved)
			})
		})

		// What plays, as a card at the foot of the categories, over who
		// listens: they stay while the playlists' column comes and goes.
		if tr := a.pl.Current(); tr != nil {
			a.sidebarNowPlaying(c, tr)
		}
		a.accountRow(c)
	})
}

// playlistColumn is the column of playlists beside the categories: it
// slides open as one of 我的 is chosen, and shut as a page is opened, as
// Music's column of playlists does. It holds a cover and its name, under
// the heading of whose they are.
func (a *app) playlistColumn(c *ui.Context) {
	p := a.pal
	col := ui.Column(c).Clip().DragWindow()
	target := float32(0)
	if a.mineOn && !a.nowPlaying {
		target = playlistWidth
	}
	w := col.AnimateTo("width", target, ui.Animation{Duration: 240 * time.Millisecond})
	// A spring could overshoot what the column may be.
	w = min(max(w, 0), playlistWidth)
	a.plW = w
	col.Width(w)
	if runtime.GOOS != "darwin" || !c.Vibrancy() {
		// A little lighter than the categories beside it, as the playlists
		// of Music are.
		shade := float32(0.5)
		if p.dark {
			shade = 0.05
		}
		col.Background(p.sidebar.Mix(white, shade))
	}
	if target == 0 && w < 0.5 {
		// Shut: nothing to list, no line to draw, and nothing built behind
		// the clip.
		return
	}
	// Its border and padding are inside its width, which leaves it no
	// narrower than they are: the column would keep that much of itself
	// between the categories and the page as it shut, a strip of nothing.
	// The border comes as it opens, and the padding sits in a column within.
	col.BorderWidth(0, 0, 0, 1).BorderColor(p.hairline.Alpha(min(w/24, 1)))
	col.Children(func() {
		ui.Column(c).Fill().Padding(columnsTop(c), 8, 8, 8).Children(func() {
			ui.Scroll(c.Key("playlists")).TrackScroll(&a.plScroll).Grow(1).Margin(0, -6).Padding(0, 6).Gap(4).Children(func() {
				a.mineList(c)
			})
		})
	})
}

// navHeader is the heading of a group of the categories, as the headings
// of Library in Music.
func (a *app) navHeader(c *ui.Context, title string) {
	ui.Text(c, title).FontSize(11.5).FontWeight(600).TextColor(a.pal.faint).Margin(8, 4, 3, 6)
}

// navGroup is the heading of a group of entries which folds away and out
// again, its arrow turning as it does, as the headings of Library hold
// theirs in Music.
func (a *app) navGroup(c *ui.Context, title string, closed *bool, fn func()) {
	p := a.pal
	b := ui.ButtonBase(c.Key(title)).Height(26).Padding(0, 4, 0, 8).Radius(7).AlignItems(ui.Center).Justify(ui.Start).Gap(6).FocusRing(false).Label(title)
	if b.Hovered() {
		b.Background(p.hover)
	}
	b.Children(func() {
		ui.Text(c, title).FontSize(11.5).FontWeight(600).TextColor(p.faint).SingleLine()
		// Closed, the arrow points at the entries it hides, as a chevron
		// of the sidebar's headings does.
		turn := float32(-90)
		if !*closed {
			turn = 0
		}
		ui.Icon(c, iconDown).Size(10, 10).Rotate(b.AnimateTo("turn", turn, ui.Animation{Duration: 150 * time.Millisecond})).
			TextColor(p.faint)
	})
	if b.Clicked() {
		*closed = !*closed
	}
	if *closed {
		return
	}
	fn()
}

// mineItem is an entry under 我的: it picks which list the playlists' column
// shows, and shows which one is shown.
func (a *app) mineItem(c *ui.Context, icon *ui.SVG, label string, kind mineKind) {
	p := a.pal
	selected := a.mineOn && a.mineShown == kind && !a.nowPlaying
	b := ui.ButtonBase(c.Key(label)).Height(30).Padding(0, 8, 0, 26).Gap(10).Radius(7).AlignItems(ui.Center).Justify(ui.Start).FocusRing(false).Label(label)
	bg := ui.Transparent
	switch {
	case selected:
		bg = p.press
	case b.Hovered():
		bg = p.hover
	}
	b.Background(b.AnimateColor("bg", bg, ui.Animation{Duration: 120 * time.Millisecond}))
	iconColor := p.muted
	if selected {
		iconColor = p.text
	}
	b.Children(func() {
		ui.Icon(c, icon).Size(16, 16).TextColor(iconColor)
		ui.Text(c, label).FontSize(13).FontWeight(500).TextColor(p.text)
	})
	if b.Clicked() {
		a.mineShown, a.mineOn = kind, true
	}
}

// mineList lists what the entry of 我的 chosen holds, under its heading.
// Podcasts have no list yet: NetEase's come as shows, which the app does
// not load.
func (a *app) mineList(c *ui.Context) {
	switch a.mineShown {
	case mineSaved:
		a.playlistList(c, "收藏的歌单", a.saved, false, "登录后，收藏的歌单显示在这里")
	case minePodcasts:
		a.playlistList(c, "播客", nil, false, "播客还没有接入")
	default:
		a.playlistList(c, "我的歌单", a.mine, true, "登录后，你的歌单显示在这里")
	}
}

func (a *app) navItem(c *ui.Context, icon *ui.SVG, label string, r route) {
	p := a.pal
	selected := !a.mineOn && a.route.page == r.page && !a.nowPlaying
	b := ui.ButtonBase(c.Key(label)).Height(32).Padding(0, 8, 0, 14).Gap(10).Radius(7).AlignItems(ui.Center).Justify(ui.Start).FocusRing(false)
	bg := ui.Transparent
	switch {
	case selected:
		bg = p.press
	case b.Hovered():
		bg = p.hover
	}
	b.Background(b.AnimateColor("bg", bg, ui.Animation{Duration: 120 * time.Millisecond}))
	iconColor := p.muted
	if selected {
		iconColor = p.text
	}
	b.Children(func() {
		ui.Icon(c, icon).Size(17, 17).TextColor(iconColor)
		ui.Text(c, label).FontSize(13).FontWeight(500).TextColor(p.text)
	})
	if b.Clicked() {
		// A page opened closes the playlists' column, which one of 我的
		// opened.
		a.mineOn = false
		a.open(r)
	}
}

// playFrom plays tracks from at, and remembers which of 我的 they came
// from: the column of playlists beside the categories shows it again on
// the next launch.
func (a *app) playFrom(tracks []netease.Track, at int) {
	a.pl.PlayQueue(tracks, at)
	a.noteSource()
}

// noteSource remembers the list the song just played came from, or that it
// came from none of 我的, whose column of playlists then stays shut on the
// next launch.
func (a *app) noteSource() {
	name := ""
	if kind, ok := a.routeMine(a.route); ok {
		name = kind.setting()
	}
	if a.settings.SideMine == name {
		return
	}
	a.settings.SideMine = name
	a.settings.save()
}

// routeMine returns which of 我的 the page of r lists the playlists of, if
// it lists any: the listener's own, or the ones they saved.
func (a *app) routeMine(r route) (mineKind, bool) {
	if r.page != pagePlaylist {
		return 0, false
	}
	for _, pl := range a.mine {
		if pl.ID == r.playlist {
			return minePlaylists, true
		}
	}
	for _, pl := range a.saved {
		if pl.ID == r.playlist {
			return mineSaved, true
		}
	}
	return 0, false
}

// setting names the list of 我的 in the settings, for the next launch.
func (k mineKind) setting() string {
	switch k {
	case mineSaved:
		return "saved"
	case minePodcasts:
		return "podcasts"
	}
	return "playlists"
}

// sidebarNowPlaying is the card at the foot of the categories: the cover of
// what plays and its name, which opens Now Playing.
func (a *app) sidebarNowPlaying(c *ui.Context, tr *netease.Track) {
	p := a.pal
	card := ui.ButtonBase(c.Key("np-card")).Gap(10).Padding(8).Radius(12).AlignItems(ui.Center).FocusRing(false).Label("正在播放")
	bg := p.hover
	if card.Hovered() {
		bg = p.press
	}
	card.Background(card.AnimateColor("bg", bg, ui.Animation{Duration: 120 * time.Millisecond}))
	card.Children(func() {
		a.cover(c, tr.Album.PicURL, 96, 40, 6).Shadow(0, 2, 6, 0, p.cardShade)
		ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(2).Children(func() {
			ui.Text(c, tr.Name).FontSize(12.5).FontWeight(600).SingleLine()
			ui.Text(c, tr.ArtistNames()).FontSize(11.5).TextColor(p.muted).SingleLine()
		})
	})
	if card.Clicked() {
		a.nowPlaying = true
	}
}

// playerBarHeight is the height of the player along the foot of the pages.
const playerBarHeight = 84

// pageGap is the room between the bar over a page and its content.
const pageGap = 12

// insets returns the room the bars over a page take at its top and its
// bottom: pages pad their content by it, and scroll under the bars.
func (a *app) insets(c *ui.Context) (top, bottom float32) {
	top = topBar(c)
	if runtime.GOOS == "darwin" {
		// The edge reaches a little below the traffic lights' strip.
		top += topEdgePad
	}
	return top, playerBarHeight
}

// topEdgePad is the room the edge over a page leaves below the strip, on
// macOS.
const topEdgePad = 6

// buttonsHeight is the height of the window buttons off macOS, which fill
// the edge over a page.
const buttonsHeight = topBarHeight + topEdgePad

// topBar returns the height of the strip along the top of the window, as
// tall as the window buttons and the room above and below them, which
// what sits in it centers on.
func topBar(c *ui.Context) float32 {
	if runtime.GOOS != "darwin" {
		// The whole edge, which the buttons fill: what sits in the strip
		// centers on their glyphs.
		return buttonsHeight
	}
	return max(c.TitleBar().Height, topBarHeight)
}

// topBarHeight is the least height of the strip along the top.
const topBarHeight = 40

// startMark marks where a page's content starts, first in what scrolls:
// once it has moved up, content is under the bar at the top.
func (a *app) startMark(c *ui.Context) {
	b := ui.Box(c.Key("start-mark")).Height(0).Bounds()
	if b.W > 0 {
		top, _ := a.insets(c)
		a.underTop = b.Y < top+pageGap-0.5
	}
}

// endMark marks where a page's content ends, last in what scrolls: while it
// is below the top of the player, content is under the player.
func (a *app) endMark(c *ui.Context) {
	b := ui.Box(c.Key("end-mark")).Height(0).Bounds()
	if b.W > 0 {
		_, h := c.Size()
		_, bottom := a.insets(c)
		a.underBottom = b.Y > h-bottom+0.5
	}
}

// edgeShown returns how much of a scroll edge shows over el: fully while
// content is under its bar, and none at rest, as AppKit shows its edges.
func edgeShown(el ui.Element, under bool) float32 {
	target := float32(0)
	if under {
		target = 1
	}
	return el.AnimateTo("shown", target, ui.Animation{Duration: 180 * time.Millisecond})
}

// pageTop is the strip over the top of a page, frosting what scrolls under
// it as macOS 26's hard scroll edge does: it drags the window, and holds
// the back button once there is a page to go back to.
func (a *app) pageTop(c *ui.Context) {
	top, _ := a.insets(c)
	edge := ui.Box(c.Key("top-edge")).Absolute().Top(0).Left(0).Right(0).Height(top).DragWindow().
		Material(glass.ScrollEdge{Hard: true, Background: a.pal.bg})
	edge.Opacity(edgeShown(edge, a.underTop))
	if len(a.history) > 0 || len(a.forward) > 0 {
		ui.Row(c).Absolute().Top(0).Left(20).Height(topBar(c)).AlignItems(ui.Center).Children(func() {
			back := iconButton(c.Key("back"), iconBack, 18, a.pal.muted, "返回").Disabled(len(a.history) == 0)
			if len(a.history) == 0 {
				back.Opacity(0.35)
			}
			if back.Clicked() {
				a.back()
			}
			if len(a.forward) > 0 && iconButton(c.Key("forward"), iconForward, 18, a.pal.muted, "前进").Clicked() {
				a.goForward()
			}
		})
	}
}

func (a *app) content(c *ui.Context) {
	// Pages mark where their content is, as they scroll.
	a.underTop, a.underBottom = false, false
	switch a.route.page {
	case pageHome:
		a.homePage(c)
	case pageDaily:
		a.dailyPage(c)
	case pageCharts:
		a.chartsPage(c)
	case pagePlaylist:
		a.playlistPage(c)
	case pageSearch:
		a.searchPage(c)
	case pageFM:
		a.fmPage(c)
	}
}

// centered shows a message, with a retry button if retry is set: what a
// page shows in place of what failed to load.
func (a *app) centered(c *ui.Context, err string, retry func()) {
	top, bottom := a.insets(c)
	ui.Column(c).Grow(1).Padding(top, 0, bottom, 0).Center().Gap(12).Children(func() {
		ui.Text(c, err).TextColor(a.pal.muted).FontSize(13)
		if retry != nil && ui.Button(c, "重试").Clicked() {
			retry()
		}
	})
}

// Loading what the pages show.

func (a *app) loadHome() {
	a.homeLoading, a.homeErr = true, ""
	signedIn := a.profile != nil
	go func() {
		featured, err1 := netease.FeaturedPlaylists(30)
		charts, err2 := netease.Charts()
		daily, _ := netease.DailySongs()
		var rec []netease.Playlist
		if signedIn {
			rec, _ = netease.RecommendedPlaylists()
		}
		a.post(func() {
			a.homeLoading = false
			a.featured, a.charts, a.daily, a.recPlaylists = featured, charts, daily, rec
			if err1 != nil && err2 != nil {
				a.homeErr = "无法连接网易云音乐，请检查网络"
			}
		})
	}()
}

func (a *app) loadPlaylist(id int64, thenPlay bool) {
	if pl := a.playlists[id]; pl != nil {
		if thenPlay {
			a.playFrom(pl.Tracks, firstPlayable(pl.Tracks))
		}
		return
	}
	if a.loadingPl[id] {
		return
	}
	a.loadingPl[id] = true
	delete(a.plErr, id)
	go func() {
		pl, err := netease.PlaylistDetail(id)
		a.post(func() {
			delete(a.loadingPl, id)
			if err != nil {
				a.plErr[id] = "歌单加载失败"
				return
			}
			a.playlists[id] = pl
			if thenPlay {
				a.playFrom(pl.Tracks, firstPlayable(pl.Tracks))
			}
		})
	}()
}

func firstPlayable(tracks []netease.Track) int {
	for i := range tracks {
		if tracks[i].Playable() {
			return i
		}
	}
	return 0
}

func (a *app) search(q string) {
	a.history, a.forward = append(a.history, a.route), nil
	a.route = route{page: pageSearch}
	a.mineOn = false
	a.trackList, a.pageScroll = ui.ListState{}, ui.ScrollState{}
	a.nowPlaying = false
	a.runSearch(q)
}

// runSearch loads the results of q, wherever the listener is.
func (a *app) runSearch(q string) {
	a.searched, a.searching, a.searchErr, a.results = q, true, "", nil
	go func() {
		res, err := netease.Search(q, 60)
		a.post(func() {
			if q != a.searched {
				return
			}
			a.searching = false
			a.results = res
			if err != nil {
				a.searchErr = "搜索失败，请稍后再试"
			}
		})
	}()
}
