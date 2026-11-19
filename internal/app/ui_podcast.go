package app

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/netease"
)

// What asks NetEase for the podcasts, which tests swap out.
var (
	savedPodcasts = netease.SavedPodcasts
	podcastOf     = netease.PodcastDetail
	programsOf    = netease.PodcastPrograms
)

// podcastProgramLimit is how many programs of a podcast load: the latest
// ones, which play from the top.
const podcastProgramLimit = 500

// podcastList lists the podcasts the listener follows, in the column beside
// the categories: a cover and its name, under a heading, as the albums they
// saved show.
func (a *app) podcastList(c *ui.Context) {
	p := a.pal
	ui.Row(c.Key("收藏的播客")).Height(30).AlignItems(ui.Center).Padding(0, 4, 0, 6).Margin(8, 0, 0, 0).Children(func() {
		ui.Text(c, "收藏的播客").FontSize(13).FontWeight(700).TextColor(p.muted).Grow(1).SingleLine()
	})
	if len(a.podcasts) == 0 {
		ui.Text(c, "登录后，收藏的播客显示在这里").FontSize(12).TextColor(p.faint).MaxLines(3).LineHeight(1.5).Margin(2, 10, 4, 6)
	}
	for i := range a.podcasts {
		pc := &a.podcasts[i]
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
	pc := a.podcastInfo[id]
	if pc == nil || !a.podLoaded[id] {
		if !a.loadingPod[id] {
			a.centered(c, a.podErr[id], func() { a.loadPodcast(id) })
			return
		}
		s := a.pageShimmer(c)
		a.trackSkeleton(c, s, false, func(c *ui.Context) { a.playlistHeaderSkeleton(c, s) })
		return
	}
	a.trackRows(c, netease.PodcastTracks(a.podcastPrograms[id]), true, func(c *ui.Context) {
		a.podcastHeader(c, pc, a.podcastPrograms[id])
	})
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

// loadPodcasts loads the podcasts the listener follows, for the column, on
// signing in.
func (a *app) loadPodcasts() {
	go func() {
		list, err := savedPodcasts(200)
		a.post(func() {
			if err == nil {
				a.podcasts = list
			}
		})
	}()
}

// loadPodcast loads the podcast id, with its programs, unless it is loaded
// or loading: what its page shows.
func (a *app) loadPodcast(id int64) {
	if a.loadingPod[id] || a.podLoaded[id] {
		return
	}
	a.initPods()
	a.loadingPod[id] = true
	delete(a.podErr, id)
	// The podcast itself: the column's, if it lists it, or the web player's.
	var known *netease.Podcast
	for i := range a.podcasts {
		if a.podcasts[i].ID == id {
			pc := a.podcasts[i]
			known = &pc
			break
		}
	}
	go func() {
		pc, err := known, error(nil)
		if pc == nil {
			pc, err = podcastOf(id)
		}
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
			a.podcastInfo[id], a.podcastPrograms[id] = pc, programs
			a.podLoaded[id] = true
		})
	}()
}

// initPods makes the maps the podcasts' pages keep, if they are not yet.
func (a *app) initPods() {
	if a.podcastInfo == nil {
		a.podcastInfo = map[int64]*netease.Podcast{}
	}
	if a.podcastPrograms == nil {
		a.podcastPrograms = map[int64][]netease.PodcastProgram{}
	}
	if a.podLoaded == nil {
		a.podLoaded = map[int64]bool{}
	}
	if a.loadingPod == nil {
		a.loadingPod = map[int64]bool{}
	}
	if a.podErr == nil {
		a.podErr = map[int64]string{}
	}
}
