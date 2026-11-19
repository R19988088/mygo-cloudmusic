package netease

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/egoist/mygo"
)

func TestQRLoginLive(t *testing.T) {
	if os.Getenv("LIVE") == "" {
		t.Skip()
	}
	// A device of the test's own, out of the keychain.
	deviceOnce.Do(func() { deviceID = "0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF0123" })
	key, err := QRKey()
	if err != nil {
		t.Fatal(err)
	}
	code, msg, _, _, err := QRCheck(key)
	if err != nil || code != QRWaiting {
		t.Fatalf("check: %d %q %v", code, msg, err)
	}
	t.Logf("key %s: %d %s", key, code, msg)
}

func TestSongURLEapiLive(t *testing.T) {
	if os.Getenv("LIVE") == "" {
		t.Skip()
	}
	deviceOnce.Do(func() { deviceID = "0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF0123" })
	var res map[string]any
	err := eapiPost("/api/song/enhance/player/url/v1", map[string]any{"ids": "[2652820720]", "level": "exhigh", "encodeType": "mp3"}, &res)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(res)
	t.Log(string(b)[:min(400, len(b))])
}

// TestSongDownloadURLLive checks the fallback that fixes -460 on the
// player's URL: the client's download link answers with a stream of the
// same level.
func TestSongDownloadURLLive(t *testing.T) {
	if os.Getenv("LIVE") == "" {
		t.Skip()
	}
	deviceOnce.Do(func() { deviceID = "0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF0123" })
	st, err := downloadStream(2652820720, "exhigh")
	if err != nil || st.URL == "" || st.Size == 0 || st.Type != "mp3" || st.Level == "" {
		t.Fatalf("download %+v %v", st, err)
	}
	t.Logf("%s %d bytes %s", st.Level, st.Size, st.Type)
}

// TestPodcastsLive checks the podcasts' chain: the web player's API lists
// the ones the listener follows, the client's lists their programs, and
// the latest program's song streams.
func TestPodcastsLive(t *testing.T) {
	if os.Getenv("LIVE") == "" {
		t.Skip()
	}
	deviceOnce.Do(func() { deviceID = "0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF0123" })
	// The listener's session, out of the keychain: only theirs answers what
	// they follow.
	session, err := mygo.Secrets.Service("dev.egoist.cloudmusic").Get(sessionSecret)
	if err != nil || len(session) == 0 {
		t.Skip("no session")
	}
	var cookies []*http.Cookie
	for _, part := range strings.Split(string(session), ";") {
		if name, value, ok := strings.Cut(strings.TrimSpace(part), "="); ok {
			cookies = append(cookies, &http.Cookie{Name: name, Value: value, Path: "/"})
		}
	}
	httpClient.Jar.SetCookies(siteURL, cookies)
	saved, err := SavedPodcasts(200)
	if err != nil || len(saved) == 0 {
		t.Fatalf("%d podcasts, %v", len(saved), err)
	}
	first := &saved[0]
	if first.ID == 0 || first.Name == "" || first.PicURL == "" || first.Host() == "" {
		t.Fatalf("first podcast %+v", first)
	}
	pc, err := PodcastDetail(first.ID)
	if err != nil || pc.Name == "" {
		t.Fatalf("detail %+v %v", pc, err)
	}
	programs, err := PodcastPrograms(first.ID, 10)
	if err != nil || len(programs) == 0 {
		t.Fatalf("%d programs, %v", len(programs), err)
	}
	tr := programs[0].Track()
	t.Logf("%d podcasts; %s by %s: %d programs, latest %q, %v", len(saved), pc.Name, pc.Host(), len(programs), tr.Name, tr.Length())
	if !tr.Playable() || tr.ID == 0 {
		t.Fatalf("latest program %+v", tr)
	}
	if st, err := SongURL(tr.ID, "exhigh"); err != nil || st.URL == "" {
		t.Fatalf("stream %+v %v", st, err)
	}
}

func TestPrivilegesLive(t *testing.T) {
	if os.Getenv("LIVE") == "" {
		t.Skip()
	}
	deviceOnce.Do(func() { deviceID = "0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF0123" })
	pl, err := PlaylistDetail(3778678)
	if err != nil {
		t.Fatal(err)
	}
	with := 0
	for _, tr := range pl.Tracks {
		if tr.Privilege != nil {
			with++
		}
	}
	daily, err := DailySongs()
	if err != nil {
		t.Fatal(err)
	}
	dailyWith := 0
	for _, tr := range daily {
		if tr.Privilege != nil {
			dailyWith++
		}
	}
	t.Logf("playlist: %d of %d tracks with privileges; daily: %d of %d", with, len(pl.Tracks), dailyWith, len(daily))
	if with == 0 || dailyWith == 0 {
		t.Fatal("no privileges")
	}
	st, err := SongURL(2652820720, "exhigh")
	if err != nil || st.URL == "" || st.Size == 0 || st.Type != "mp3" {
		t.Fatalf("stream %+v %v", st, err)
	}
}

func TestRecentPlaysLive(t *testing.T) {
	// The session of a development build, which has the listener's plays.
	path := os.Getenv("NM_SESSION")
	if os.Getenv("LIVE") == "" || path == "" {
		t.Skip()
	}
	deviceOnce.Do(func() { deviceID = "0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF0123" })
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var cookies []*http.Cookie
	for _, part := range strings.Split(string(b), ";") {
		if name, value, ok := strings.Cut(strings.TrimSpace(part), "="); ok {
			cookies = append(cookies, &http.Cookie{Name: name, Value: value, Path: "/"})
		}
	}
	httpClient.Jar.SetCookies(siteURL, cookies)
	plays, err := RecentPlays(20)
	if err != nil || len(plays) == 0 {
		t.Fatalf("%d plays, %v", len(plays), err)
	}
	for _, p := range plays[:min(5, len(plays))] {
		t.Logf("%s %s %q playable %v", p.At.Format("01-02 15:04"), p.Device, p.Track.Name, p.Track.Playable())
	}
	if plays[0].Track.ID == 0 || plays[0].Device == "" || plays[0].At.Year() < 2020 {
		t.Fatalf("latest play %+v", plays[0])
	}
}
