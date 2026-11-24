package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/netease"
)

// searchKind is what a search found, by kind: the aggregate of them all,
// which shows alone as 私人 FM does, and each of the kinds, which show with
// the column beside the categories that lists what the search found of them,
// as one of 我的 does.
type searchKind int

const (
	searchAll searchKind = iota
	searchSongs
	searchPlaylists
	searchPodcasts
	searchAlbums
	searchArtists
	searchMVs
)

// title names the kind, as the sidebar and the pages head it.
func (k searchKind) title() string {
	switch k {
	case searchSongs:
		return "单曲"
	case searchPlaylists:
		return "歌单"
	case searchPodcasts:
		return "播客"
	case searchAlbums:
		return "专辑"
	case searchArtists:
		return "歌手"
	case searchMVs:
		return "MV"
	}
	return "综合"
}

// icon is the kind's icon in the sidebar.
func (k searchKind) icon() *ui.SVG {
	switch k {
	case searchSongs:
		return iconNote
	case searchPlaylists:
		return iconQueue
	case searchPodcasts:
		return iconMic
	case searchAlbums:
		return iconDisc
	case searchArtists:
		return iconUser
	case searchMVs:
		return iconFilm
	}
	return iconSearch
}

// found is how many of the kind the search found.
func (k searchKind) found(a *app) int {
	r := a.results
	switch k {
	case searchSongs:
		return len(r.Songs)
	case searchPlaylists:
		return len(r.Playlists)
	case searchPodcasts:
		return len(r.Podcasts)
	case searchAlbums:
		return len(r.Albums)
	case searchArtists:
		return len(r.Artists)
	case searchMVs:
		return len(r.MVs)
	}
	return len(r.Songs) + len(r.Playlists) + len(r.Podcasts) +
		len(r.Albums) + len(r.Artists) + len(r.MVs)
}

// countLabel says how much of the kind was found, as the column heads it.
func (k searchKind) countLabel(n int) string {
	switch k {
	case searchSongs:
		return fmt.Sprintf("%d 首", n)
	case searchPlaylists:
		return fmt.Sprintf("%d 个歌单", n)
	case searchPodcasts:
		return fmt.Sprintf("%d 个播客", n)
	case searchAlbums:
		return fmt.Sprintf("%d 张专辑", n)
	case searchArtists:
		return fmt.Sprintf("%d 位歌手", n)
	case searchMVs:
		return fmt.Sprintf("%d 个 MV", n)
	}
	return fmt.Sprintf("%d 个结果", n)
}

// nothing is what a page of the kind says when it found none.
func (k searchKind) nothing() string {
	if k == searchAll {
		return "没有找到相关的结果"
	}
	return "没有找到相关的" + k.title()
}

// The 搜索结果 section of the sidebar.

// searchGroup is the 搜索结果 section of the sidebar, as 我的 is: a heading
// that folds its kinds away and out again, a cross that closes the search
// altogether, and under it the kinds of what was found.
func (a *app) searchGroup(c *ui.Context) {
	p := a.pal
	// The column's text, at 80%, as the search field's hint.
	t := p.words(p.nav).text.Alpha(0.8)
	ui.Row(c).Height(26).Gap(2).AlignItems(ui.Center).Children(func() {
		b := ui.ButtonBase(c.Key("搜索结果")).Grow(1).Height(26).Padding(0, 4, 0, 8).Radius(7).
			AlignItems(ui.Center).Justify(ui.Start).Gap(6).FocusRing(false).Label("搜索结果")
		if b.Hovered() {
			b.Background(p.hover)
		}
		b.Children(func() {
			ui.Text(c, "搜索结果").FontSize(11.5).FontWeight(600).TextColor(t).SingleLine()
			// Closed, the arrow points at the kinds it hides, as a chevron
			// of the sidebar's headings does.
			turn := float32(-90)
			if !a.searchClosed {
				turn = 0
			}
			ui.Icon(c, iconDown).Size(10, 10).Rotate(b.AnimateTo("turn", turn, ui.Animation{Duration: 150 * time.Millisecond})).
				TextColor(t)
		})
		if b.Clicked() {
			a.searchClosed = !a.searchClosed
		}
		// The cross closes the search, across from the heading it closes.
		if iconButton(c.Key("close-search"), iconX, 12, t.Alpha(0.7), "关闭搜索结果").Clicked() {
			a.closeSearch()
		}
	})
	if a.searchClosed {
		return
	}
	for _, k := range []searchKind{searchAll, searchSongs, searchPlaylists, searchPodcasts, searchAlbums, searchArtists, searchMVs} {
		a.searchItem(c, k)
	}
}

// searchItem is an entry under 搜索结果: 综合 shows its page alone, and each
// kind shows the column of what was found beside it, as one of 我的 does.
func (a *app) searchItem(c *ui.Context, kind searchKind) {
	p := a.pal
	label := kind.title()
	// 综合 shows with its page alone; each kind shows while its column
	// stays, as one of 我的 shows while its column does.
	selected := !a.nowPlaying && ((kind == searchAll && a.route.page == pageSearch && a.route.search == searchAll) ||
		(kind != searchAll && a.searchOn && a.searchShown == kind))
	b := ui.ButtonBase(c.Key("found-"+label)).Height(30).Padding(0, 8, 0, 26).Gap(10).Radius(7).
		AlignItems(ui.Center).Justify(ui.Start).FocusRing(false).Label(label)
	bg := ui.Transparent
	switch {
	case selected:
		bg = p.press
	case b.Hovered():
		bg = p.hover
	}
	b.Background(b.AnimateColor("bg", bg, ui.Animation{Duration: 120 * time.Millisecond}))
	iconColor := p.navIcons(selected)
	b.Children(func() {
		ui.Icon(c, kind.icon()).Size(16, 16).TextColor(iconColor)
		ui.Text(c, label).FontSize(13).FontWeight(500).TextColor(p.words(p.nav).text).Grow(1).SingleLine()
		if n := kind.found(a); n > 0 && kind != searchAll {
			ui.Text(c, fmt.Sprint(n)).FontSize(11.5).TextColor(p.words(p.nav).faint)
		}
	})
	if b.Clicked() {
		a.chooseSearch(kind)
	}
}

// chooseSearch shows the page of the kind chosen, with the column of what
// was found beside it — but 综合's, which shows alone.
func (a *app) chooseSearch(kind searchKind) {
	a.mineOn, a.searchShown = false, kind
	a.searchOn = kind != searchAll
	r := route{page: pageSearch, search: kind}
	if r == a.route {
		return
	}
	a.open(r)
}

// searchColumn lists what the search found of the kind shown, in the column
// beside the categories: songs which a click plays, and the rest as the lists
// of 我的 list theirs.
func (a *app) searchColumn(c *ui.Context) {
	p := a.pal
	kind := a.searchShown
	w := p.words(p.pl)
	ui.Row(c.Key("search-column")).Height(30).AlignItems(ui.Center).Padding(0, 4, 0, 6).Margin(8, 0, 0, 0).Children(func() {
		ui.Text(c, kind.title()).FontSize(13).FontWeight(700).TextColor(w.muted).Grow(1).SingleLine()
		if n := kind.found(a); n > 0 {
			ui.Text(c, kind.countLabel(n)).FontSize(11.5).TextColor(w.faint).SingleLine()
		}
	})
	switch kind {
	case searchSongs:
		for i := range a.results.Songs {
			tr := &a.results.Songs[i]
			cur := a.pl.Current()
			selected := cur != nil && cur.ID == tr.ID
			a.searchRow(c, (fmt.Sprint("find-song-", tr.ID)), tr.Album.PicURL, tr.Name, tr.ArtistNames(), selected, func() {
				if tr.Playable() {
					a.playFrom(a.results.Songs, i)
				}
			})
		}
	case searchPlaylists:
		for i := range a.results.Playlists {
			pl := &a.results.Playlists[i]
			selected := !a.nowPlaying && a.route.page == pagePlaylist && a.route.playlist == pl.ID
			a.searchRow(c, (fmt.Sprint("find-pl-", pl.ID)), pl.Cover(), pl.Name, "", selected, func() {
				a.open(route{page: pagePlaylist, playlist: pl.ID})
			})
		}
	case searchPodcasts:
		for i := range a.results.Podcasts {
			pc := &a.results.Podcasts[i]
			selected := !a.nowPlaying && a.route.page == pagePodcast && a.route.podcast == pc.ID
			a.searchRow(c, (fmt.Sprint("find-pod-", pc.ID)), pc.PicURL, pc.Name, "", selected, func() {
				a.open(route{page: pagePodcast, podcast: pc.ID})
			})
		}
	case searchAlbums:
		for i := range a.results.Albums {
			al := &a.results.Albums[i]
			selected := !a.nowPlaying && a.route.page == pageAlbum && a.route.album == al.ID
			a.searchRow(c, (fmt.Sprint("find-al-", al.ID)), al.PicURL, al.Name, "", selected, func() {
				a.open(route{page: pageAlbum, album: al.ID})
			})
		}
	case searchArtists:
		for i := range a.results.Artists {
			ar := &a.results.Artists[i]
			selected := !a.nowPlaying && a.route.page == pageArtist && a.route.artist == ar.ID
			a.searchRow(c, (fmt.Sprint("find-ar-", ar.ID)), ar.PicURL, ar.Name, "", selected, func() {
				a.open(route{page: pageArtist, artist: ar.ID})
			})
		}
	case searchMVs:
		for i := range a.results.MVs {
			mv := &a.results.MVs[i]
			selected := !a.nowPlaying && a.route.page == pageMV && a.route.mv == mv.ID
			a.searchRow(c, (fmt.Sprint("find-mv-", mv.ID)), mv.Cover, mv.Name, "", selected, func() {
				a.open(route{page: pageMV, mv: mv.ID})
			})
		}
	}
}

// searchRow is a row of the column of what was found: a picture, and its
// name over a second line, which says what it is.
func (a *app) searchRow(c *ui.Context, key any, pic, title, sub string, selected bool, click func()) {
	p := a.pal
	w := p.words(p.pl)
	b := ui.ButtonBase(c.Key(key)).Height(52).PaddingX(6).Gap(11).Radius(9).
		AlignItems(ui.Center).Justify(ui.Start).FocusRing(false).Label(title)
	switch {
	case selected:
		b.Background(p.press)
	case b.Hovered():
		b.Background(p.hover)
	}
	b.Children(func() {
		a.cover(c, pic, 120, 40, 8)
		if sub == "" {
			ui.Text(c, title).FontSize(14.5).SingleLine().Shrink(1).MinWidth(0).TextColor(w.text)
			return
		}
		ui.Column(c).Grow(1).Basis(0).Shrink(1).MinWidth(0).Gap(2).Children(func() {
			ui.Text(c, title).FontSize(13.5).FontWeight(500).SingleLine().TextColor(w.text)
			ui.Text(c, sub).FontSize(11.5).SingleLine().TextColor(w.muted)
		})
	})
	if b.Clicked() {
		click()
	}
}

// The pages of what a search found.

func (a *app) searchPage(c *ui.Context) {
	if a.route.search == searchAll {
		a.searchAllPage(c)
		return
	}
	a.searchKindPage(c)
}

// searchAllPage is what the search found, all of it: a few songs, then the
// artists, playlists, albums, podcasts and MVs, each headed with the way to
// all of them.
func (a *app) searchAllPage(c *ui.Context) {
	header := func(c *ui.Context) { a.heading(c, "“"+a.searched+"”", "搜索") }
	if a.searchAllEmpty() && a.searching {
		a.trackSkeleton(c, a.pageShimmer(c), true, header)
		return
	}
	if a.searchAllEmpty() {
		a.searchNothing(c, header)
		return
	}
	a.pageScrollable(c, "search-all", header, func(c *ui.Context) {
		a.searchSongs(c)
		a.searchArtists(c)
		a.searchPlaylists(c)
		a.searchAlbums(c)
		a.searchPodcasts(c)
		a.searchMVs(c)
	})
}

// searchAllEmpty reports whether the search found nothing at all.
func (a *app) searchAllEmpty() bool { return searchAll.found(a) == 0 }

// pageScrollable shows a page under a header, scrolled, with its edges'
// marks: what the pages of the search found share.
func (a *app) pageScrollable(c *ui.Context, key string, header func(*ui.Context), body func(*ui.Context)) {
	top, bottom := a.insets(c)
	ui.Scroll(c.Key(key)).TrackScroll(&a.pageScroll).Grow(1).
		Padding(top+pageGap, pagePad, bottom+48, pagePad).ScrollbarInsets(top, 0, bottom, 0).Children(func() {
		a.startMark(c)
		defer a.endMark(c)
		header(c)
		body(c)
	})
}

// searchNothing shows the page of a search that found nothing.
func (a *app) searchNothing(c *ui.Context, header func(*ui.Context)) {
	msg := a.searchErr
	if msg == "" {
		msg = a.route.search.nothing()
	}
	top, bottom := a.insets(c)
	ui.Column(c).Grow(1).Padding(top+pageGap, pagePad, bottom, pagePad).Children(func() {
		header(c)
		a.centered(c, msg, nil)
	})
}

// searchSection heads a kind's share of what the search found, with the way
// to all of it when there is more than it shows.
func (a *app) searchSection(c *ui.Context, kind searchKind, total, shown int) {
	p := a.pal
	ui.Row(c).AlignItems(ui.Baseline).Margin(30, 0, 12, 0).Children(func() {
		ui.Text(c, kind.title()).FontSize(19).FontWeight(700).LetterSpacing(-0.3).Grow(1)
		if total <= shown {
			return
		}
		b := ui.ButtonBase(c.Key("all-" + kind.title())).Height(22).PaddingX(8).Radius(6).FocusRing(false).Label("查看全部")
		if b.Hovered() {
			b.Background(p.hover)
		}
		b.Children(func() {
			ui.Text(c, "查看全部").FontSize(12.5).TextColor(p.accent).SingleLine()
		})
		if b.Clicked() {
			a.chooseSearch(kind)
		}
	})
}

func (a *app) searchSongs(c *ui.Context) {
	n := len(a.results.Songs)
	if n == 0 {
		return
	}
	shown := min(n, 6)
	a.searchSection(c, searchSongs, n, shown)
	ui.Column(c).Gap(1).Children(func() {
		for i := range shown {
			a.searchSongRow(c, a.results.Songs, i)
		}
	})
}

// searchSongRow lists a song among the search's songs, small: its cover, its
// name over its artist, and how long it runs; a click plays from it.
func (a *app) searchSongRow(c *ui.Context, tracks []netease.Track, i int) {
	p := a.pal
	tr := &tracks[i]
	cur := a.pl.Current()
	playing := cur != nil && cur.ID == tr.ID
	b := ui.ButtonBase(c.Key(fmt.Sprint("song-", tr.ID))).Height(46).PaddingX(8).Gap(12).Radius(9).
		AlignItems(ui.Center).FocusRing(false).Label(tr.Name).ContextMenu(a.songItems(tr, 0))
	bg := ui.Transparent
	switch {
	case playing:
		bg = p.accent.Alpha(0.09)
	case b.Hovered():
		bg = p.hover
	}
	b.Background(b.AnimateColor("bg", bg, ui.Animation{Duration: 100 * time.Millisecond}))
	if !tr.Playable() {
		b.Opacity(0.42)
	}
	b.Children(func() {
		a.cover(c, tr.Album.PicURL, 80, 34, 5)
		ui.Column(c).Grow(1).Basis(0).Shrink(1).MinWidth(0).Gap(2).Children(func() {
			name := p.text
			if playing {
				name = p.accent
			}
			ui.Text(c, tr.Name).FontSize(13.5).FontWeight(500).TextColor(name).SingleLine()
			ui.Text(c, tr.ArtistNames()).FontSize(11.5).TextColor(p.muted).SingleLine()
		})
		ui.Text(c, clock(tr.Length())).FontSize(11.5).FontFeatures("tnum").TextColor(p.faint)
	})
	if b.Clicked() && tr.Playable() {
		a.playFrom(tracks, i)
	}
}

func (a *app) searchArtists(c *ui.Context) {
	n := len(a.results.Artists)
	if n == 0 {
		return
	}
	shown := min(n, a.columns(c, 120)*2)
	a.searchSection(c, searchArtists, n, shown)
	ui.Grid(c).Columns(min(shown, a.columns(c, 120))).GapX(20).GapY(22).Children(func() {
		for i := range shown {
			a.artistCard(c.Key(fmt.Sprint("ar-", a.results.Artists[i].ID)), &a.results.Artists[i])
		}
	})
}

func (a *app) searchPlaylists(c *ui.Context) {
	n := len(a.results.Playlists)
	if n == 0 {
		return
	}
	shown := min(n, a.columns(c, 168)*2)
	a.searchSection(c, searchPlaylists, n, shown)
	ui.Grid(c).Columns(a.columns(c, 168)).GapX(20).GapY(26).Children(func() {
		for i := range shown {
			a.playlistCard(c.Key(fmt.Sprint("pl-", a.results.Playlists[i].ID)), &a.results.Playlists[i])
		}
	})
}

func (a *app) searchAlbums(c *ui.Context) {
	n := len(a.results.Albums)
	if n == 0 {
		return
	}
	shown := min(n, a.columns(c, 150)*2)
	a.searchSection(c, searchAlbums, n, shown)
	ui.Grid(c).Columns(a.columns(c, 150)).GapX(20).GapY(26).Children(func() {
		for i := range shown {
			a.albumCard(c.Key(fmt.Sprint("al-", a.results.Albums[i].ID)), &a.results.Albums[i])
		}
	})
}

func (a *app) searchPodcasts(c *ui.Context) {
	n := len(a.results.Podcasts)
	if n == 0 {
		return
	}
	shown := min(n, 4)
	a.searchSection(c, searchPodcasts, n, shown)
	ui.Column(c).Gap(1).Children(func() {
		for i := range shown {
			a.podcastRow(c, (fmt.Sprint("pod-", a.results.Podcasts[i].ID)), &a.results.Podcasts[i])
		}
	})
}

func (a *app) searchMVs(c *ui.Context) {
	n := len(a.results.MVs)
	if n == 0 {
		return
	}
	shown := min(n, a.columns(c, 220)*2)
	a.searchSection(c, searchMVs, n, shown)
	ui.Grid(c).Columns(a.columns(c, 220)).GapX(20).GapY(26).Children(func() {
		for i := range shown {
			a.mvCard(c.Key(fmt.Sprint("mv-", a.results.MVs[i].ID)), &a.results.MVs[i])
		}
	})
}

// searchKindPage shows what the search found of the kind its route names,
// all of it, as the pages of 我的 show theirs.
func (a *app) searchKindPage(c *ui.Context) {
	kind := a.route.search
	header := func(c *ui.Context) { a.heading(c, "“"+a.searched+"”", "搜索 · "+kind.title()) }
	if kind.found(a) == 0 && a.searching {
		a.trackSkeleton(c, a.pageShimmer(c), true, header)
		return
	}
	if kind.found(a) == 0 {
		a.searchNothing(c, header)
		return
	}
	switch kind {
	case searchSongs:
		a.trackRows(c, a.results.Songs, true, header)
	case searchPlaylists:
		a.pageScrollable(c, "search-playlists", header, func(c *ui.Context) {
			ui.Grid(c).Columns(a.columns(c, 168)).GapX(20).GapY(28).Children(func() {
				for i := range a.results.Playlists {
					a.playlistCard(c.Key(fmt.Sprint("pl-", a.results.Playlists[i].ID)), &a.results.Playlists[i])
				}
			})
		})
	case searchAlbums:
		a.pageScrollable(c, "search-albums", header, func(c *ui.Context) {
			ui.Grid(c).Columns(a.columns(c, 150)).GapX(20).GapY(28).Children(func() {
				for i := range a.results.Albums {
					a.albumCard(c.Key(fmt.Sprint("al-", a.results.Albums[i].ID)), &a.results.Albums[i])
				}
			})
		})
	case searchArtists:
		a.pageScrollable(c, "search-artists", header, func(c *ui.Context) {
			ui.Grid(c).Columns(a.columns(c, 120)).GapX(20).GapY(26).Children(func() {
				for i := range a.results.Artists {
					a.artistCard(c.Key(fmt.Sprint("ar-", a.results.Artists[i].ID)), &a.results.Artists[i])
				}
			})
		})
	case searchPodcasts:
		a.pageScrollable(c, "search-podcasts", header, func(c *ui.Context) {
			ui.Column(c).Gap(1).Children(func() {
				for i := range a.results.Podcasts {
					a.podcastRow(c, (fmt.Sprint("pod-", a.results.Podcasts[i].ID)), &a.results.Podcasts[i])
				}
			})
		})
	case searchMVs:
		a.pageScrollable(c, "search-mvs", header, func(c *ui.Context) {
			ui.Grid(c).Columns(a.columns(c, 220)).GapX(20).GapY(28).Children(func() {
				for i := range a.results.MVs {
					a.mvCard(c.Key(fmt.Sprint("mv-", a.results.MVs[i].ID)), &a.results.MVs[i])
				}
			})
		})
	}
}

// albumCard is an album the search found: its cover, its name, and who made
// it, which opens its page.
func (a *app) albumCard(c *ui.Context, al *netease.SavedAlbum) {
	p := a.pal
	card := ui.ButtonBase(c).Column().Gap(9).AlignItems(ui.Stretch).FocusRing(false).Label(al.Name)
	hovered := card.Hovered()
	card.Children(func() {
		art := ui.Box(c).AspectRatio(1).Radius(10)
		lift := ui.Pose{}
		if hovered {
			lift = ui.Pose{Y: -3}
			art.Shadow(0, 12, 28, -6, p.cardShade)
		} else {
			art.Shadow(0, 2, 8, -2, p.cardShade)
		}
		art.AnimatePose(lift, ui.Snappy)
		art.Children(func() { a.coverFill(c, al.PicURL, 400, 10) })
		ui.Text(c, al.Name).FontSize(13).FontWeight(500).MaxLines(2).LineHeight(1.35)
		sub := al.ArtistNames()
		if sub == "" && al.Size > 0 {
			sub = fmt.Sprintf("%d 首", al.Size)
		}
		if sub != "" {
			ui.Text(c, sub).FontSize(12).TextColor(p.muted).SingleLine()
		}
	})
	if card.Clicked() {
		a.open(route{page: pageAlbum, album: al.ID})
	}
}

// artistCard is an artist the search found: their picture round, their name,
// and how much they have made, which opens their page.
func (a *app) artistCard(c *ui.Context, ar *netease.SearchArtist) {
	p := a.pal
	card := ui.ButtonBase(c).Column().Gap(10).AlignItems(ui.Center).FocusRing(false).Label(ar.Name)
	hovered := card.Hovered()
	card.Children(func() {
		portrait := ui.Box(c).Size(96, 96).Radius(999)
		lift := ui.Pose{}
		if hovered {
			lift = ui.Pose{Y: -3}
			portrait.Shadow(0, 12, 28, -6, p.cardShade)
		} else {
			portrait.Shadow(0, 2, 8, -2, p.cardShade)
		}
		portrait.AnimatePose(lift, ui.Snappy)
		portrait.Children(func() { a.coverFill(c, ar.PicURL, 320, 48) })
		ui.Text(c, ar.Name).FontSize(13.5).FontWeight(500).SingleLine()
		var meta []string
		if ar.MusicSize > 0 {
			meta = append(meta, fmt.Sprintf("%d 首", ar.MusicSize))
		}
		if ar.AlbumSize > 0 {
			meta = append(meta, fmt.Sprintf("%d 张专辑", ar.AlbumSize))
		}
		if len(meta) > 0 {
			ui.Text(c, strings.Join(meta, " · ")).FontSize(11.5).TextColor(p.muted).SingleLine()
		}
	})
	if card.Clicked() {
		a.open(route{page: pageArtist, artist: ar.ID})
	}
}

// mvCard is an MV the search found: its picture, its name and who made it,
// which opens its page.
func (a *app) mvCard(c *ui.Context, mv *netease.MV) {
	p := a.pal
	card := ui.ButtonBase(c).Column().Gap(9).AlignItems(ui.Stretch).FocusRing(false).Label(mv.Name)
	hovered := card.Hovered()
	card.Children(func() {
		art := ui.Box(c).AspectRatio(16.0 / 9.0).Radius(10)
		lift := ui.Pose{}
		if hovered {
			lift = ui.Pose{Y: -3}
			art.Shadow(0, 12, 28, -6, p.cardShade)
		} else {
			art.Shadow(0, 2, 8, -2, p.cardShade)
		}
		art.AnimatePose(lift, ui.Snappy)
		art.Children(func() {
			a.coverFill(c, mv.Cover, 480, 10)
			ui.Row(c).Absolute().Top(8).Right(8).Padding(2, 7).Radius(999).Gap(3).AlignItems(ui.Center).
				Background(ui.RGBA(0, 0, 0, 0.35)).TextColor(ui.RGB(255, 255, 255)).Children(func() {
				ui.Icon(c, iconPlay).Size(9, 9)
				ui.Text(c, plays(mv.PlayCount)).FontSize(10.5).FontWeight(600)
			})
			// The film of it, which the MV plays as.
			ui.Row(c).Absolute().Left(8).Bottom(8).Size(26, 26).Radius(999).Center().
				Background(ui.RGBA(255, 255, 255, 0.92)).TextColor(ui.Hex("#1d1d1f")).Children(func() {
				ui.Icon(c, iconPlay).Size(11, 11).Margin(0, 0, 0, 1)
			})
		})
		ui.Text(c, mv.Name).FontSize(13).FontWeight(500).MaxLines(2).LineHeight(1.35)
		if names := mv.ArtistNames(); names != "" {
			ui.Text(c, names).FontSize(12).TextColor(p.muted).SingleLine()
		}
	})
	if card.Clicked() {
		a.open(route{page: pageMV, mv: mv.ID})
	}
}

// podcastRow is a podcast the search found: its cover, its name, and who
// hosts it with how many programs it has, which opens its page.
func (a *app) podcastRow(c *ui.Context, key any, pc *netease.Podcast) {
	p := a.pal
	selected := !a.nowPlaying && a.route.page == pagePodcast && a.route.podcast == pc.ID
	b := ui.ButtonBase(c.Key(key)).Height(60).PaddingX(8).Gap(13).Radius(10).AlignItems(ui.Center).FocusRing(false).Label(pc.Name)
	switch {
	case selected:
		b.Background(p.press)
	case b.Hovered():
		b.Background(p.hover)
	}
	b.Children(func() {
		a.cover(c, pc.PicURL, 160, 48, 9)
		ui.Column(c).Grow(1).Basis(0).Shrink(1).MinWidth(0).Gap(3).Children(func() {
			ui.Text(c, pc.Name).FontSize(13.5).FontWeight(500).SingleLine()
			var meta []string
			if host := pc.Host(); host != "" {
				meta = append(meta, host)
			}
			if pc.ProgramCount > 0 {
				meta = append(meta, fmt.Sprintf("%d 期", pc.ProgramCount))
			}
			if pc.Category != "" {
				meta = append(meta, pc.Category)
			}
			ui.Text(c, strings.Join(meta, " · ")).FontSize(11.5).TextColor(p.muted).SingleLine()
		})
	})
	if b.Clicked() {
		a.open(route{page: pagePodcast, podcast: pc.ID})
	}
}

// The page of an artist the search found: their picture and what they have
// made, over the songs they are known for.

func (a *app) artistPage(c *ui.Context) {
	id := a.route.artist
	songs := a.artistSongs[id]
	if songs == nil {
		if !a.loadingAr[id] {
			a.centered(c, a.arErr[id], func() { a.loadArtist(id) })
			return
		}
		s := a.pageShimmer(c)
		a.trackSkeleton(c, s, false, func(c *ui.Context) { a.playlistHeaderSkeleton(c, s) })
		return
	}
	a.trackRows(c, songs, false, func(c *ui.Context) { a.artistHeader(c, id, songs) })
}

func (a *app) artistHeader(c *ui.Context, id int64, songs []netease.Track) {
	p := a.pal
	ar := a.artistInfo[id]
	name, pic := "", ""
	// Their other names, a couple of them: past that they run wider than the
	// line they head.
	var alias []string
	var musicSize, albumSize int
	if ar != nil {
		name, pic = ar.Name, ar.PicURL
		musicSize, albumSize = ar.MusicSize, ar.AlbumSize
		for _, s := range ar.Alias {
			if s != "" && s != name && len(alias) < 2 {
				alias = append(alias, s)
			}
		}
	}
	ui.Row(c).Gap(28).AlignItems(ui.End).Padding(0, 0, 26, 0).Children(func() {
		ui.Box(c).Size(196, 196).Radius(999).Shadow(0, 18, 40, -10, p.cardShade).Children(func() {
			a.coverFill(c, pic, 560, 98)
		})
		ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(8).Children(func() {
			ui.Text(c, "歌手").FontSize(12).FontWeight(600).TextColor(p.accent)
			ui.Text(c, name).FontSize(30).FontWeight(800).LetterSpacing(-0.6).MaxLines(2).LineHeight(1.2)
			var meta []string
			if len(alias) > 0 {
				meta = append(meta, strings.Join(alias, "、"))
			}
			if musicSize > 0 {
				meta = append(meta, fmt.Sprintf("%d 首作品", musicSize))
			}
			if albumSize > 0 {
				meta = append(meta, fmt.Sprintf("%d 张专辑", albumSize))
			}
			if len(meta) > 0 {
				ui.Text(c, strings.Join(meta, " · ")).FontSize(13).TextColor(p.muted)
			}
			ui.Text(c, "热门作品").FontSize(12.5).TextColor(p.muted).Margin(2, 0, 0, 0)
			ui.Row(c).Gap(10).Margin(10, 0, 0, 0).Children(func() {
				if pillButton(c.Key("play"), iconPlay, "播放全部", p.accent, p.onAccent).Clicked() {
					if a.pl.Shuffle() {
						a.pl.ToggleShuffle()
					}
					a.playFrom(songs, firstPlayable(songs))
				}
				if pillButton(c.Key("shuffle"), iconShuffle, "随机播放", p.hover.Over(p.bg), p.text).Clicked() {
					if !a.pl.Shuffle() {
						a.pl.ToggleShuffle()
					}
					a.playFrom(songs, firstPlayable(songs))
				}
			})
		})
	})
}

// loadArtist loads the artist id's songs, unless they are loaded or loading:
// what their page shows.
func (a *app) loadArtist(id int64) {
	if a.loadingAr[id] || a.artistSongs[id] != nil {
		return
	}
	if a.loadingAr == nil {
		a.loadingAr, a.arErr = map[int64]bool{}, map[int64]string{}
	}
	a.loadingAr[id] = true
	delete(a.arErr, id)
	go func() {
		songs, err := netease.ArtistTopSongs(id)
		a.post(func() {
			delete(a.loadingAr, id)
			if err != nil {
				a.arErr[id] = "歌手页加载失败"
				return
			}
			if a.artistSongs == nil {
				a.artistSongs = map[int64][]netease.Track{}
			}
			a.artistSongs[id] = songs
		})
	}()
}

// mvPage shows the MV the search found: its picture, its name and credits,
// and the way to it on the web, where it plays: the app plays songs.
func (a *app) mvPage(c *ui.Context) {
	p := a.pal
	mv := a.mvInfo[a.route.mv]
	if mv == nil {
		a.centered(c, "没有找到这个 MV", nil)
		return
	}
	top, bottom := a.insets(c)
	ui.Scroll(c.Key("mv")).TrackScroll(&a.pageScroll).Grow(1).
		Padding(top+pageGap, pagePad, bottom+48, pagePad).ScrollbarInsets(top, 0, bottom, 0).Children(func() {
		a.startMark(c)
		defer a.endMark(c)
		ui.Row(c).Children(func() {
			ui.Box(c).Width(min(560, a.contentWidth(c)-2*pagePad)).AspectRatio(16.0/9.0).Radius(16).
				Shadow(0, 18, 44, -14, p.cardShade).Children(func() {
				a.coverFill(c, mv.Cover, 900, 16)
			})
		})
		ui.Text(c, "MV").FontSize(12).FontWeight(600).TextColor(p.accent).Margin(26, 0, 6, 0)
		ui.Text(c, mv.Name).FontSize(28).FontWeight(800).LetterSpacing(-0.6).MaxLines(2).LineHeight(1.25)
		var meta []string
		if names := mv.ArtistNames(); names != "" {
			meta = append(meta, names)
		}
		if mv.PlayCount > 0 {
			meta = append(meta, plays(mv.PlayCount)+" 次观看")
		}
		if mv.Duration > 0 {
			meta = append(meta, clock(time.Duration(mv.Duration)*time.Millisecond))
		}
		ui.Text(c, strings.Join(meta, " · ")).FontSize(13).TextColor(p.muted).Margin(10, 0, 0, 0)
		if d := strings.TrimSpace(mv.Desc); d != "" {
			ui.Text(c, strings.ReplaceAll(d, "\n", " ")).FontSize(12.5).TextColor(p.muted).
				MaxLines(3).LineHeight(1.5).Margin(10, 0, 0, 0)
		}
		ui.Row(c).Gap(10).Margin(20, 0, 0, 0).Children(func() {
			if pillButton(c.Key("watch"), nil, "在浏览器中观看", p.accent, p.onAccent).Clicked() {
				mygo.Shell.OpenExternal(mv.URL())
			}
			if pillButton(c.Key("artist"), iconUser, "查看歌手", p.hover.Over(p.bg), p.text).Clicked() {
				if mv.ArtistID != 0 {
					if a.artistInfo == nil {
						a.artistInfo = map[int64]*netease.SearchArtist{}
					}
					if _, ok := a.artistInfo[mv.ArtistID]; !ok {
						a.artistInfo[mv.ArtistID] = &netease.SearchArtist{ID: mv.ArtistID, Name: mv.ArtistName}
					}
					a.open(route{page: pageArtist, artist: mv.ArtistID})
				}
			}
		})
		ui.Text(c, "MV 的画面在网易云音乐里播放，这里只显示它的资料").FontSize(12).TextColor(p.faint).Margin(16, 0, 0, 0)
	})
}
