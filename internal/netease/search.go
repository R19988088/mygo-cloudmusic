package netease

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"
)

// What a search finds, by kind: the web player asks for one kind at a time —
// songs (1), playlists (1000), podcasts (1009), albums (10), artists (100),
// and MVs (1004) — and no one type answers them all.

// SearchArtist is an artist a search finds: their picture, the names they go
// by, and how much they have made.
type SearchArtist struct {
	ID        int64    `json:"id"`
	Name      string   `json:"name"`
	PicURL    string   `json:"picUrl"`
	Alias     []string `json:"alias"`
	AlbumSize int      `json:"albumSize"`
	MusicSize int      `json:"musicSize"`
	MVSize    int      `json:"mvSize"`
}

// Names joins the artist's aliases as their page credits them.
func (ar *SearchArtist) Names() string {
	out := make([]string, 0, len(ar.Alias)+1)
	out = append(out, ar.Name)
	for _, a := range ar.Alias {
		if a != "" && a != ar.Name {
			out = append(out, a)
		}
	}
	return joinNames(out)
}

// MV is a music video a search finds: its cover, who made it, how long it
// runs, and how often it was watched.
type MV struct {
	ID         int64    `json:"id"`
	Name       string   `json:"name"`
	Cover      string   `json:"cover"`
	PlayCount  float64  `json:"playCount"`
	ArtistName string   `json:"artistName"`
	ArtistID   int64    `json:"artistId"`
	Duration   int64    `json:"duration"` // milliseconds
	Desc       string   `json:"desc"`
	Artists    []Artist `json:"artists"`
}

// ArtistNames joins the MV's artists as its credits read, or its one name
// when it lists none.
func (m *MV) ArtistNames() string {
	if names := artistNames(m.Artists); names != "" {
		return names
	}
	return m.ArtistName
}

// URL is the MV's page on the NetEase website, where it plays.
func (m *MV) URL() string { return apiBase + "/mv?id=" + strconv.FormatInt(m.ID, 10) }

// SearchResults is what a search finds, of every kind.
type SearchResults struct {
	Songs     []Track
	Playlists []Playlist
	Podcasts  []Podcast
	Albums    []SavedAlbum
	Artists   []SearchArtist
	MVs       []MV
}

// How much of each kind a search keeps: more than its aggregate page shows,
// and as many as the pages of the kinds list.
const (
	searchSongLimit     = 60
	searchPlaylistLimit = 30
	searchPodcastLimit  = 24
	searchAlbumLimit    = 30
	searchArtistLimit   = 30
	searchMVLimit       = 30
)

// searchJSON asks for one kind of what matches query, as typ names it.
func searchJSON(typ, query string, limit int, out any) error {
	q := url.Values{"s": {query}, "type": {typ}, "limit": {strconv.Itoa(limit)}, "offset": {"0"}}
	return get("/api/cloudsearch/pc", q, out)
}

// SearchAll returns what matches query, of every kind at once: one kind
// failing leaves the others, and only all of them failing is an error.
func SearchAll(query string) (*SearchResults, error) {
	var (
		songs     []Track
		playlists []Playlist
		podcasts  []Podcast
		albums    []SavedAlbum
		artists   []SearchArtist
		mvs       []MV
		errs      [6]error
		wg        sync.WaitGroup
	)
	wg.Add(6)
	go func() {
		defer wg.Done()
		var res struct {
			Result struct {
				Songs []Track `json:"songs"`
			} `json:"result"`
		}
		if errs[0] = searchJSON("1", query, searchSongLimit, &res); errs[0] == nil {
			songs = res.Result.Songs
		}
	}()
	go func() {
		defer wg.Done()
		var res struct {
			Result struct {
				Playlists []Playlist `json:"playlists"`
			} `json:"result"`
		}
		if errs[1] = searchJSON("1000", query, searchPlaylistLimit, &res); errs[1] == nil {
			playlists = res.Result.Playlists
		}
	}()
	go func() {
		defer wg.Done()
		var res struct {
			Result struct {
				DJRadios []Podcast `json:"djRadios"`
			} `json:"result"`
		}
		if errs[2] = searchJSON("1009", query, searchPodcastLimit, &res); errs[2] == nil {
			podcasts = res.Result.DJRadios
		}
	}()
	go func() {
		defer wg.Done()
		var res struct {
			Result struct {
				Albums []SavedAlbum `json:"albums"`
			} `json:"result"`
		}
		if errs[3] = searchJSON("10", query, searchAlbumLimit, &res); errs[3] == nil {
			albums = res.Result.Albums
		}
	}()
	go func() {
		defer wg.Done()
		var res struct {
			Result struct {
				Artists []SearchArtist `json:"artists"`
			} `json:"result"`
		}
		if errs[4] = searchJSON("100", query, searchArtistLimit, &res); errs[4] == nil {
			artists = res.Result.Artists
		}
	}()
	go func() {
		defer wg.Done()
		var res struct {
			Result struct {
				MVs []MV `json:"mvs"`
			} `json:"result"`
		}
		if errs[5] = searchJSON("1004", query, searchMVLimit, &res); errs[5] == nil {
			mvs = res.Result.MVs
		}
	}()
	wg.Wait()
	failed := 0
	for _, err := range errs {
		if err != nil {
			failed++
		}
	}
	if failed == len(errs) {
		return nil, errs[0]
	}
	// Search marks whole catalogues as taken down that the listener may
	// play, as YOASOBI's for a subscriber: the songs' details say right.
	if len(songs) > 0 {
		detailPrivileges(songs)
	}
	return &SearchResults{
		Songs:     songs,
		Playlists: playlists,
		Podcasts:  podcasts,
		Albums:    albums,
		Artists:   artists,
		MVs:       mvs,
	}, nil
}

// ArtistTopSongs returns the artist's most played songs.
func ArtistTopSongs(id int64) ([]Track, error) {
	var res struct {
		Code  int     `json:"code"`
		Songs []Track `json:"songs"`
	}
	q := url.Values{"id": {strconv.FormatInt(id, 10)}}
	if err := get("/api/artist/top/song", q, &res); err != nil {
		return nil, err
	}
	if res.Code != 200 || len(res.Songs) == 0 {
		return nil, fmt.Errorf("netease: artist %d: code %d, %d songs", id, res.Code, len(res.Songs))
	}
	detailPrivileges(res.Songs)
	return res.Songs, nil
}

// joinNames joins names as credits read, without repeating one.
func joinNames(names []string) string {
	out := make([]string, 0, len(names))
	for _, n := range names {
		if n == "" {
			continue
		}
		seen := false
		for _, o := range out {
			if o == n {
				seen = true
				break
			}
		}
		if !seen {
			out = append(out, n)
		}
	}
	return strings.Join(out, ", ")
}
