package app

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/netease"
)

// What asks NetEase for the podcasts, which tests swap out.
var (
	findPodcasts = netease.SearchPodcasts
	podcastOf    = netease.PodcastDetail
	programsOf   = netease.PodcastPrograms
)

const (
	// searchPodcastsLimit is how many podcasts a search lists.
	searchPodcastsLimit = 30
	// keptPodcasts is how many podcasts opened here the column keeps.
	keptPodcasts = 8
	// podcastProgramLimit is how many programs of a podcast load: the
	// latest ones, which play from the top.
	podcastProgramLimit = 100
)

// podcastList is the column of 播客: a field to search NetEase's podcasts
// by name, over the ones the search found, or else the ones opened here
// before, as a cover and its name. What the listener follows with NetEase
// is no longer served: the app keeps its own.
func (a *app) podcastList(c *ui.Context) {
	p := a.pal
	ui.Row(c.Key("播客")).Height(30).AlignItems(ui.Center).Padding(0, 4, 0, 6).Margin(8, 0, 0, 0).Children(func() {
		ui.Text(c, "播客").FontSize(13).FontWeight(700).TextColor(p.muted).Grow(1).SingleLine()
	})
	field := ui.Row(c).Height(28).PaddingX(8).Gap(6).Radius(8).AlignItems(ui.Center).Margin(0, 2, 0, 10).
		Background(p.hover).Border(1, p.hairline)
	field.Children(func() {
		ui.Icon(c, iconSearch).Size(12, 12).TextColor(p.muted)
		in := ui.TextInputBase(c.Key("podcast-search"), &a.podcastQuery).Bind(&a.podcastBox).Grow(1).
			Placeholder("搜索播客").FontSize(12).Label("搜索播客")
		if in.Submitted() {
			a.searchPodcasts(strings.TrimSpace(a.podcastQuery))
		}
	})

	if a.podcastSearching {
		ui.Text(c, "正在搜索…").FontSize(12).TextColor(p.faint).Margin(2, 6, 4, 6)
		return
	}
	list := a.podcasts
	if len(a.found) > 0 {
		list = a.found
	}
	if len(list) == 0 {
		empty := a.podcastErr
		if empty == "" {
			empty = "搜索播客的名字吧：打开过的播客会列在这里"
		}
		ui.Text(c, empty).FontSize(12).TextColor(p.faint).MaxLines(4).LineHeight(1.5).Margin(2, 10, 4, 6)
		return
	}
	for i := range list {
		pc := &list[i]
		selected := a.route.page == pagePodcast && a.route.podcast == pc.ID && !a.nowPlaying
		b := ui.ButtonBase(c.Key(pc.ID)).Height(52).PaddingX(6).Gap(11).Radius(9).AlignItems(ui.Center).Justify(ui.Start).FocusRing(false).Label(pc.Name)
		switch {
		case selected:
			b.Background(p.press)
		case b.Hovered():
			b.Background(p.hover)
		}
		b.Children(func() {
			a.cover(c, pc.PicURL, 140, 40, 8)
			ui.Text(c, pc.Name).FontSize(14.5).SingleLine().Shrink(1).MinWidth(0)
		})
		if b.Clicked() {
			a.open(route{page: pagePodcast, podcast: pc.ID})
		}
	}
}

// podcastPage shows the podcast the route names: its cover, its host and
// its programs, the latest first, which a click plays.
func (a *app) podcastPage(c *ui.Context) {
	id := a.route.podcast
	programs := a.podcastPrograms[id]
	pc := a.podcastInfo[id]
	if pc == nil {
		if !a.loadingPod[id] {
			a.centered(c, a.podErr[id], func() { a.loadPodcast(id) })
			return
		}
		s := a.pageShimmer(c)
		a.trackSkeleton(c, s, false, func(c *ui.Context) { a.playlistHeaderSkeleton(c, s) })
		return
	}
	a.trackRows(c, netease.PodcastTracks(programs), true, func(c *ui.Context) { a.podcastHeader(c, pc, programs) })
}

// podcastHeader is the podcast's cover over who hosts it, how many programs
// it has and what it is about, with the buttons that play them.
func (a *app) podcastHeader(c *ui.Context, pc *netease.Podcast, programs []netease.PodcastProgram) {
	p := a.pal
	ui.Row(c).Gap(28).AlignItems(ui.End).Padding(0, 0, 26, 0).Children(func() {
		ui.Box(c).Size(212, 212).Radius(14).Shadow(0, 18, 40, -10, p.cardShade).Children(func() {
			a.coverFill(c, pc.PicURL, 600, 14)
		})
		ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(8).Children(func() {
			ui.Text(c, "播客").FontSize(12).FontWeight(600).TextColor(p.accent)
			ui.Text(c, pc.Name).FontSize(30).FontWeight(800).LetterSpacing(-0.6).MaxLines(2).LineHeight(1.2)
			var meta []string
			if host := pc.Host(); host != "" {
				meta = append(meta, host)
			}
			meta = append(meta, fmt.Sprintf("%d 期", len(programs)))
			if pc.Category != "" {
				meta = append(meta, pc.Category)
			}
			ui.Text(c, strings.Join(meta, " · ")).FontSize(13).TextColor(p.muted)
			if d := strings.TrimSpace(pc.Desc); d != "" {
				ui.Text(c, strings.ReplaceAll(d, "\n", " ")).FontSize(12.5).TextColor(p.muted).MaxLines(2).LineHeight(1.45).Margin(2, 0, 0, 0)
			}
			tracks := netease.PodcastTracks(programs)
			ui.Row(c).Gap(10).Margin(10, 0, 0, 0).Children(func() {
				if pillButton(c.Key("play"), iconPlay, "播放最新", p.accent, p.onAccent).Clicked() {
					if a.pl.Shuffle() {
						a.pl.ToggleShuffle()
					}
					a.playFrom(tracks, firstPlayable(tracks))
				}
				if pillButton(c.Key("shuffle"), iconShuffle, "随机播放", p.hover.Over(p.bg), p.text).Clicked() {
					if !a.pl.Shuffle() {
						a.pl.ToggleShuffle()
					}
					a.playFrom(tracks, firstPlayable(tracks))
				}
			})
		})
	})
}

// searchPodcasts lists the podcasts whose names match query, in the column,
// in place of the ones opened here.
func (a *app) searchPodcasts(query string) {
	if query == "" {
		a.found, a.podcastErr = nil, ""
		return
	}
	a.podcastSearching = true
	a.podcastErr = ""
	go func() {
		list, err := findPodcasts(query, searchPodcastsLimit)
		a.post(func() {
			a.podcastSearching = false
			if err != nil {
				a.podcastErr, a.found = "搜索播客失败", nil
				return
			}
			a.found = list
			if len(list) == 0 {
				a.podcastErr = "没有找到相关播客"
			}
		})
	}()
}

// loadPodcast loads the podcast id, with its programs, unless it is loaded
// or loading: what its page shows.
func (a *app) loadPodcast(id int64) {
	if a.loadingPod[id] || a.podcastInfo[id] != nil {
		return
	}
	if a.loadingPod == nil {
		a.loadingPod, a.podErr = map[int64]bool{}, map[int64]string{}
	}
	a.loadingPod[id] = true
	delete(a.podErr, id)
	go func() {
		pc, err := podcastOf(id)
		var programs []netease.PodcastProgram
		if err == nil {
			programs, err = programsOf(id, podcastProgramLimit)
		}
		a.post(func() {
			delete(a.loadingPod, id)
			if err != nil {
				a.podErr[id] = "播客加载失败"
				return
			}
			if a.podcastInfo == nil {
				a.podcastInfo = map[int64]*netease.Podcast{}
			}
			if a.podcastPrograms == nil {
				a.podcastPrograms = map[int64][]netease.PodcastProgram{}
			}
			a.podcastInfo[id], a.podcastPrograms[id] = pc, programs
			a.keepPodcast(pc)
		})
	}()
}

// loadKeptPodcasts loads what the podcasts opened here are, as the column
// lists them beside the categories before a search.
func (a *app) loadKeptPodcasts() {
	ids := a.settings.Podcasts
	if len(ids) == 0 {
		return
	}
	go func() {
		var found []*netease.Podcast
		for _, id := range ids {
			if pc, err := podcastOf(id); err == nil {
				found = append(found, pc)
			}
		}
		a.post(func() {
			if a.podcastInfo == nil {
				a.podcastInfo = map[int64]*netease.Podcast{}
			}
			for _, pc := range found {
				a.podcastInfo[pc.ID] = pc
			}
			a.listKeptPodcasts()
		})
	}()
}

// keepPodcast remembers the podcast opened, the newest first, and lists it
// beside the categories: the app's own followings, as NetEase serves none.
func (a *app) keepPodcast(pc *netease.Podcast) {
	ids := []int64{pc.ID}
	for _, id := range a.settings.Podcasts {
		if id != pc.ID {
			ids = append(ids, id)
		}
	}
	if len(ids) > keptPodcasts {
		ids = ids[:keptPodcasts]
	}
	if a.podcastInfo == nil {
		a.podcastInfo = map[int64]*netease.Podcast{}
	}
	a.podcastInfo[pc.ID] = pc
	a.settings.Podcasts = ids
	a.settings.save()
	a.listKeptPodcasts()
}

// listKeptPodcasts lists the podcasts the settings kept, in their order.
func (a *app) listKeptPodcasts() {
	a.podcasts = a.podcasts[:0]
	for _, id := range a.settings.Podcasts {
		if pc := a.podcastInfo[id]; pc != nil {
			a.podcasts = append(a.podcasts, *pc)
		}
	}
}
