package netease

import (
	"fmt"
)

// The podcasts: what the listener follows answers on the web player's API,
// as the albums they saved do, and the programs of one there too.

// Podcast is a show of NetEase's: its cover, its name, who hosts it, and
// how many programs it has.
type Podcast struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	PicURL       string `json:"picUrl"`
	Desc         string `json:"desc"`
	ProgramCount int    `json:"programCount"`
	SubCount     int64  `json:"subCount"`
	Category     string `json:"category"`
	DJ           struct {
		Nickname  string `json:"nickname"`
		AvatarURL string `json:"avatarUrl"`
	} `json:"dj"`
}

func (p *Podcast) Host() string { return p.DJ.Nickname }

// SavedPodcasts returns the podcasts the listener follows.
func SavedPodcasts(limit int) ([]Podcast, error) {
	var res struct {
		DJRadios []Podcast `json:"djRadios"`
		Data     struct {
			DJRadios []Podcast `json:"djRadios"`
			List     []Podcast `json:"list"`
		} `json:"data"`
	}
	data := map[string]any{"offset": "0", "limit": fmt.Sprint(limit), "total": "true"}
	if err := weapiPost("/weapi/djradio/get/subed", data, &res); err != nil {
		return nil, err
	}
	switch {
	case len(res.DJRadios) > 0:
		return res.DJRadios, nil
	case len(res.Data.DJRadios) > 0:
		return res.Data.DJRadios, nil
	}
	return res.Data.List, nil
}

// PodcastDetail returns the podcast id: its cover, its name, who hosts it,
// and how many programs it has.
func PodcastDetail(id int64) (*Podcast, error) {
	var res struct {
		Data Podcast `json:"data"`
	}
	if err := weapiPost("/weapi/djradio/v2/get", map[string]any{"id": fmt.Sprint(id)}, &res); err != nil {
		return nil, err
	}
	if res.Data.ID == 0 {
		return nil, fmt.Errorf("netease: podcast %d: nothing came back", id)
	}
	return &res.Data, nil
}

// PodcastProgram is an episode of a podcast: its name, its cover, when it
// was published, and the song it plays as.
type PodcastProgram struct {
	ID          int64   `json:"id"`
	Name        string  `json:"name"`
	CoverURL    string  `json:"coverUrl"`
	Serial      int     `json:"serialNum"`
	Duration    int64   `json:"duration"` // milliseconds
	Description string  `json:"description"`
	CreateTime  int64   `json:"createTime"` // milliseconds
	MainSong    Track   `json:"mainSong"`
	MainTrackID int64   `json:"mainTrackId"`
	Radio       Podcast `json:"radio"`
}

// PodcastPrograms returns the programs of the podcast radio, the latest
// first.
func PodcastPrograms(radio int64, limit int) ([]PodcastProgram, error) {
	var res struct {
		Programs []PodcastProgram `json:"programs"`
	}
	data := map[string]any{"radioId": fmt.Sprint(radio), "offset": "0", "limit": fmt.Sprint(limit), "asc": "false"}
	if err := weapiPost("/weapi/dj/program/byradio", data, &res); err != nil {
		return nil, err
	}
	return res.Programs, nil
}

// Track is the song a program plays, named as the program and credited to
// the podcast, its cover the program's.
func (p *PodcastProgram) Track() Track {
	tr := p.MainSong
	if tr.ID == 0 {
		tr.ID = p.MainTrackID
	}
	if p.Name != "" {
		tr.Name = p.Name
	}
	if len(tr.Artists) == 0 {
		if name := p.Radio.Host(); name != "" {
			tr.Artists = []Artist{{Name: name}}
		} else if p.Radio.Name != "" {
			tr.Artists = []Artist{{Name: p.Radio.Name}}
		}
	}
	if tr.Album.PicURL == "" {
		tr.Album.PicURL = p.CoverURL
	}
	if tr.Album.Name == "" {
		tr.Album.Name = p.Radio.Name
	}
	if tr.Duration == 0 {
		tr.Duration = p.Duration
	}
	return tr
}

// PodcastTracks turns the programs into the songs the player plays.
func PodcastTracks(programs []PodcastProgram) []Track {
	out := make([]Track, len(programs))
	for i := range programs {
		out[i] = programs[i].Track()
	}
	return out
}
