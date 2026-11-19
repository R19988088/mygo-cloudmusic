package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/netease"
)

// albumPage shows the album the route names: its cover and credits, over
// the songs it holds.
func (a *app) albumPage(c *ui.Context) {
	id := a.route.album
	al := a.albumSongs[id]
	if al == nil {
		if !a.loadingAl[id] {
			a.centered(c, a.alErr[id], func() { a.loadAlbum(id) })
			return
		}
		s := a.pageShimmer(c)
		a.trackSkeleton(c, s, false, func(c *ui.Context) { a.playlistHeaderSkeleton(c, s) })
		return
	}
	a.trackRows(c, al.Songs, false, func(c *ui.Context) { a.albumHeader(c, al) })
}

// albumHeader is the album's cover over its credits: who made it, how many
// songs, and the year it came out, with the buttons that play them.
func (a *app) albumHeader(c *ui.Context, al *netease.AlbumSongs) {
	p := a.pal
	ui.Row(c).Gap(28).AlignItems(ui.End).Padding(0, 0, 26, 0).Children(func() {
		ui.Box(c).Size(212, 212).Radius(14).Shadow(0, 18, 40, -10, p.cardShade).Children(func() {
			a.coverFill(c, al.Album.PicURL, 600, 14)
		})
		ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(8).Children(func() {
			ui.Text(c, "专辑").FontSize(12).FontWeight(600).TextColor(p.accent)
			ui.Text(c, al.Album.Name).FontSize(30).FontWeight(800).LetterSpacing(-0.6).MaxLines(2).LineHeight(1.2)
			var meta []string
			if names := al.Album.ArtistNames(); names != "" {
				meta = append(meta, names)
			}
			meta = append(meta, fmt.Sprintf("%d 首", len(al.Songs)))
			if al.Album.PublishTime > 0 {
				meta = append(meta, time.UnixMilli(al.Album.PublishTime).Format("2006")+" 年")
			}
			ui.Text(c, strings.Join(meta, " · ")).FontSize(13).TextColor(p.muted)
			if d := strings.TrimSpace(al.Album.Description); d != "" {
				ui.Text(c, strings.ReplaceAll(d, "\n", " ")).FontSize(12.5).TextColor(p.muted).MaxLines(2).LineHeight(1.45).Margin(2, 0, 0, 0)
			}
			ui.Row(c).Gap(10).Margin(10, 0, 0, 0).Children(func() {
				if pillButton(c.Key("play"), iconPlay, "播放", p.accent, p.onAccent).Clicked() {
					if a.pl.Shuffle() {
						a.pl.ToggleShuffle()
					}
					a.playFrom(al.Songs, firstPlayable(al.Songs))
				}
				if pillButton(c.Key("shuffle"), iconShuffle, "随机播放", p.hover.Over(p.bg), p.text).Clicked() {
					if !a.pl.Shuffle() {
						a.pl.ToggleShuffle()
					}
					a.playFrom(al.Songs, firstPlayable(al.Songs))
				}
			})
		})
	})
}

// loadAlbums loads the albums the listener uid saved.
func (a *app) loadAlbums(uid int64) {
	go func() {
		albums, err := netease.SavedAlbums(60)
		a.post(func() {
			if err == nil && a.profile != nil && a.profile.UserID == uid {
				a.albums = albums
			}
		})
	}()
}

// loadAlbum loads the album id, with its songs, unless it is loaded or
// loading: what the page of it shows, or what the listener may play from
// the column of albums.
func (a *app) loadAlbum(id int64) {
	if a.albumSongs[id] != nil || a.loadingAl[id] {
		return
	}
	if a.loadingAl == nil {
		a.loadingAl, a.alErr = map[int64]bool{}, map[int64]string{}
	}
	a.loadingAl[id] = true
	delete(a.alErr, id)
	go func() {
		al, err := netease.AlbumDetail(id)
		a.post(func() {
			delete(a.loadingAl, id)
			if err != nil {
				a.alErr[id] = "专辑加载失败"
				return
			}
			if a.albumSongs == nil {
				a.albumSongs = map[int64]*netease.AlbumSongs{}
			}
			a.albumSongs[id] = al
		})
	}()
}
