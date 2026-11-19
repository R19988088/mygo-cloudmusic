package netease

import (
	"fmt"
	"strconv"
)

// The listener's albums: the ones they saved, and one's songs, through the
// eapi, which carries the session.

// SavedAlbum is an album of the listener's: its cover, its name, who made
// it, and how many songs it holds.
type SavedAlbum struct {
	ID      int64    `json:"id"`
	Name    string   `json:"name"`
	PicURL  string   `json:"picUrl"`
	Size    int      `json:"size"`
	Artists []Artist `json:"artists"`
	// PublishTime is when it came out, in milliseconds, which the album
	// page reads where it knows it.
	PublishTime int64  `json:"publishTime"`
	Company     string `json:"company"`
	Description string `json:"description"`
}

// ArtistNames joins the album's artists as its credits read.
func (al *SavedAlbum) ArtistNames() string { return artistNames(al.Artists) }

// SavedAlbums returns the albums the listener saved, the latest first.
func SavedAlbums(limit int) ([]SavedAlbum, error) {
	var res struct {
		Code int          `json:"code"`
		Data []SavedAlbum `json:"data"`
	}
	data := map[string]any{"limit": strconv.Itoa(limit), "offset": "0", "total": "true"}
	if err := eapiPost("/api/album/sublist", data, &res); err != nil {
		return nil, err
	}
	if res.Code != 200 {
		return nil, fmt.Errorf("netease: saved albums: code %d", res.Code)
	}
	return res.Data, nil
}

// AlbumSongs is an album and the songs it holds.
type AlbumSongs struct {
	Album SavedAlbum
	Songs []Track
}

// AlbumDetail returns the album id and its songs. The songs come without
// privileges: theirs are asked for, as the daily songs' are.
func AlbumDetail(id int64) (*AlbumSongs, error) {
	var res struct {
		Code  int        `json:"code"`
		Album SavedAlbum `json:"album"`
		Songs []Track    `json:"songs"`
	}
	if err := get("/api/v1/album/"+strconv.FormatInt(id, 10), nil, &res); err != nil {
		return nil, err
	}
	if res.Album.ID == 0 {
		return nil, fmt.Errorf("netease: album %d: code %d", id, res.Code)
	}
	detailPrivileges(res.Songs)
	return &AlbumSongs{Album: res.Album, Songs: res.Songs}, nil
}
