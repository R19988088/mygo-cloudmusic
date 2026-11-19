package app

import (
	"runtime"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/netease"
	"netease-music-mygo/internal/player"
)

type pageKind int

const (
	pageHome pageKind = iota
	pageDaily
	pageCharts
	pagePlaylist
	pageAlbum
	pagePodcast
	pageSearch
	pageFM
	// pagePick is the third level of a list of 我的 before anything in it is
	// chosen: the column beside the categories lists what to choose from.
	pagePick
)

type route struct {
	page     pageKind
	playlist int64
	album    int64
	podcast  int64
}

// mineKind is a list of the sidebar's 我的 section, which the column of
// playlists shows beside it.
type mineKind int

const (
	minePlaylists mineKind = iota // 歌单
	mineAlbums                    // 专辑
	mineSaved                     // 收藏
	minePodcasts                  // 播客
)

// app is the state both windows show. Everything in it is read and changed
// on the main thread: by the views, and by goroutines through post.
type app struct {
	win   *mygo.Window
	notch *notch
	tray  *tray
	art   *artCache
	pl    *player.Player
	pal   palette
	// The colors the settings window shows in its wells: the listener's, or
	// the appearance's while they chose none.
	mainW, iconW ui.Color

	route   route
	history []route
	forward []route // the pages gone back from, the last first to go to

	profile      *netease.Profile
	mine, saved  []netease.Playlist   // the listener's playlists: made, and saved
	albums       []netease.SavedAlbum // the albums the listener saved
	recPlaylists []netease.Playlist   // the day's playlists picked for the listener
	login        loginState
	accountOpen  bool // the dropdown of the account row shows
	// The sidebar's 我的 section: closed (the default is open), which of
	// its lists the column beside the categories shows, and whether one of
	// them is chosen: the column shows while one is, and closes as a page
	// is opened, animated by plW.
	mineClosed bool
	mineShown  mineKind
	mineOn     bool
	// mineAt is what each list of 我的 last showed: choosing that list beside
	// the categories opens its page again, or says which one to pick when it
	// shows none.
	mineAt   map[mineKind]int64
	plW      float32        // how wide that column is, as it last laid out
	sizeDue  bool           // the window's size is written soon
	naming   nameDialog     // the name of a playlist asked for
	deleting deleteDialog   // whether to delete a playlist, asked
	liked    map[int64]bool // the songs the listener likes
	toast    string         // a message to show once
	picked   pickedUp       // the song a login picked up, waiting to play

	daily       []netease.Track
	dailySpin   spinState // the record on the daily card
	dailyBars   barState  // the bars on its play button
	featured    []netease.Playlist
	charts      []netease.Playlist
	homeErr     string
	homeLoading bool

	playlists map[int64]*netease.Playlist
	loadingPl map[int64]bool
	plErr     map[int64]string
	// The album opened, with its songs, and the one loading.
	albumSongs map[int64]*netease.AlbumSongs
	loadingAl  map[int64]bool
	alErr      map[int64]string
	// The podcasts the listener follows, listed beside the categories, and
	// the podcast whose page shows, with its programs.
	podcasts        []netease.Podcast
	podcastInfo     map[int64]*netease.Podcast
	podcastPrograms map[int64][]netease.PodcastProgram
	podLoaded       map[int64]bool
	loadingPod      map[int64]bool
	podErr          map[int64]string
	trackList       ui.ListState
	// Where the page, the sidebar's categories and its playlists are
	// scrolled, kept while Now Playing covers them, unbuilt.
	pageScroll ui.ScrollState
	navScroll  ui.ScrollState
	plScroll   ui.ScrollState

	query     string
	searched  string
	results   []netease.Track
	searching bool
	searchErr string
	searchBox ui.Handle

	nowPlaying  bool
	npOpen      bool      // Now Playing as it last showed: open or not
	npChangedAt time.Time // when it last opened or closed
	npPane      int       // paneLyrics, paneQueue, paneStage or paneComments
	npPaneTop   float32   // how far the cover beside the pane starts below its top
	lyricScroll ui.ScrollState
	lyricGlide  lyricGlide
	lyricAt     int
	lyricTrack  int64
	queueList   ui.ListState
	queueAt     int // the row of the queue pane last scrolled to, -1 while it is hidden
	comments    commentState
	tabCount    rollState // the count on the comments' tab, as it rolls

	fmMode        int    // the FM mode chosen, in fmModes
	radioStarting string // the radio asked for its first songs, or ""

	scrub      float64
	scrubbing  bool
	volume     float64
	volumeOpen bool // the volume popover of the player bar shows
	mutedFrom  float64

	rowBars barState
	// underTop and underBottom say whether the page has content under its
	// bars, which show their scroll edges then.
	underTop, underBottom bool
	barScrub              scrubState
	npScrub               scrubState
	backdropCache         backdropCache // the backdrop of Now Playing, as last painted
	npArt                 *artwork      // the cover whose colors Now Playing shows
	npCoverState          coverState    // the cover of Now Playing, as it is tilted and flicked
	quiet                 quietState    // the quiet parts of the song playing, for its hot comments
	sig                   playerSig
	quitting              bool

	settings    settings
	settingsWin *mygo.Window
	cacheUsed   int64 // the bytes the song cache takes, -1 until measured
	updates     bool  // the update plugin is used (useUpdater)

	restored     bool   // playback picked up where it was left
	savedSession string // the session as last kept
	sessionDue   bool   // a save of the session is due
}

// post runs fn on the main thread and redraws both windows. Without a
// window, as in tests, it drops fn.
func (a *app) post(fn func()) {
	if a.win == nil {
		return
	}
	a.win.Update(func() {
		fn()
		a.redraw()
	})
}

func (a *app) redraw() {
	if a.win != nil {
		a.win.Invalidate()
	}
	if a.notch != nil && a.notch.win != nil {
		a.notch.win.Invalidate()
	}
}

// shotHook, when set, drives the app to check its design.
var shotHook func(a *app)

func (a *app) showMain() {
	if a.win != nil {
		a.win.Show()
		a.win.Focus()
	}
}

// Run runs the app until it quits.
func Run() error {
	a := &app{
		playlists:   map[int64]*netease.Playlist{},
		loadingPl:   map[int64]bool{},
		plErr:       map[int64]string{},
		podcastInfo: map[int64]*netease.Podcast{},
		podLoaded:   map[int64]bool{},
		loadingPod:  map[int64]bool{},
		podErr:      map[int64]string{},
		lyricAt:     -1,
		queueAt:     -1,
		cacheUsed:   -1,
	}
	moveOldData()
	a.useUpdater()
	mygo.App.WhenReady(func() {
		a.settings = loadSettings()
		a.settings.applyAppearance()
		// The sidebar shows the list the last song played came from, and
		// nothing beside the categories when it came from none.
		a.restoreSide()
		a.npPane = a.lyricsPaneChosen()
		// Before the window: on Windows it lays out its content as it is
		// made, and the view asks the player what plays.
		a.art = newArtCache(a.post)
		a.pl = player.New(a.post)
		a.pl.Quality = func() string { return a.settings.Quality }
		a.pl.Normalize = func() bool { return a.settings.Normalize }
		a.pl.Cache = newSongCache(a.settings.CacheMB)
		a.pl.Changed = a.keepSession
		a.volume = a.settings.Volume
		a.pl.SetVolume(a.volume)
		b := a.place()
		a.win = mygo.NewWindow(mygo.WindowOptions{
			Title:     mygo.App.Name(),
			X:         b.X,
			Y:         b.Y,
			Width:     b.Width,
			Height:    b.Height,
			MinWidth:  880,
			MinHeight: 580,
			// No StateKey: the framework keeps the window's bounds when it
			// closes, and this app hides to the notification area instead,
			// so its bounds are the settings' (noteSize).
			TitleBarStyle: mygo.TitleBarHidden,
			// As tall as the edge over a page (Linux, Windows).
			TitleBarHeight: buttonsHeight,
			// Inset from the corner, within the sidebar, as macOS 26's
			// apps with a sidebar place them.
			TrafficLightPosition: &mygo.Point{X: 20, Y: 20},
			Vibrancy:             mygo.VibrancySidebar,
			BackgroundColor:      "light-dark(#ffffff, #141416)",
			Content:              ui.View(a.view),
		})
		a.setMenu()
		// How big it was left, kept a moment after it stops changing: the
		// window hides to the notification area rather than closing, and
		// what closes is the app, which may be ended without a word.
		a.win.OnResize(a.noteSize)
		a.win.OnMove(a.noteSize)
		a.boot()
		// The notch player is macOS's, over its menu bar: elsewhere it would
		// hang at the top of the screen as a black box.
		if runtime.GOOS == "darwin" {
			a.notch = newNotch(a)
		}
		// Elsewhere the app keeps in the notification area, which brings it
		// back once its window is closed.
		if runtime.GOOS != "darwin" {
			a.tray = startTray(a)
		}
		if shotHook != nil {
			shotHook(a)
		}
		// On macOS closing the window hides it: music plays on, under the
		// notch, and the Dock icon brings it back. Elsewhere the tray does,
		// so closing hides it too.
		a.win.OnClose(func(e *mygo.CloseEvent) {
			if a.quitting {
				return
			}
			a.saveSize()
			e.PreventDefault()
			if runtime.GOOS == "darwin" || (a.tray != nil && a.tray.ok) {
				a.win.Hide()
			} else {
				mygo.App.Quit()
			}
		})
	})
	mygo.App.OnBeforeQuit(func(*mygo.QuitEvent) {
		a.quitting = true
		if a.tray != nil {
			a.tray.remove()
		}
		a.saveSize()
		a.settings.Volume = a.volume
		a.settings.save()
		a.saveSession()
	})
	mygo.App.OnWindowAllClosed(func() {})
	mygo.App.OnActivate(func(bool) { a.showMain() })
	return mygo.App.Run()
}

// The size the window opens at, before the listener has left it any other.
const (
	windowWidth  = 1180
	windowHeight = 780
)

// windowBounds is where and how big the window opens: 0, 0 for the middle
// of its display.
type windowBounds struct {
	X, Y, Width, Height int
}

// place returns the bounds the window opens at: what the settings kept of
// the last window, if enough of it would show on a display; else its size,
// in the middle of one, as its place went with a display that is gone.
func (a *app) place() windowBounds {
	b := windowBounds{Width: windowWidth, Height: windowHeight}
	if a.settings.WindowWidth < 880 || a.settings.WindowHeight < 580 {
		return b
	}
	b.Width, b.Height = a.settings.WindowWidth, a.settings.WindowHeight
	x, y := a.settings.WindowX, a.settings.WindowY
	for _, d := range displays() {
		area := d.WorkArea
		w := min(area.X+area.Width, x+b.Width) - max(area.X, x)
		h := min(area.Y+area.Height, y+b.Height) - max(area.Y, y)
		if w >= min(100, b.Width) && h >= min(50, b.Height) {
			b.X, b.Y = x, y
			return b
		}
	}
	return b
}

// displays is the displays a kept place is checked against.
var displays = func() []mygo.Display { return mygo.Screen.Displays() }

// sizeDelay is how long the window's bounds must stay unchanged to be kept.
const sizeDelay = 400 * time.Millisecond

// noteSize keeps where and how big the window is a moment after it stops
// changing, so a drag of its edge does not write the settings on every
// frame.
func (a *app) noteSize() {
	if a.sizeDue {
		return
	}
	a.sizeDue = true
	sizeAfter(sizeDelay, func() {
		a.sizeDue = false
		a.saveSize()
	})
}

// sizeAfter runs fn on the main thread after d, without redrawing a window.
var sizeAfter = func(d time.Duration, fn func()) {
	time.AfterFunc(d, func() { mygo.RunOnMain(fn) })
}

// saveSize keeps the bounds the window has now, unless it is maximized or
// minimized, whose bounds are not the ones to open at.
func (a *app) saveSize() {
	if a.win == nil || a.win.IsMaximized() || a.win.IsMinimized() {
		return
	}
	b := a.win.Bounds()
	if b.Width < 880 || b.Height < 580 {
		return
	}
	if a.settings.WindowX == b.X && a.settings.WindowY == b.Y &&
		a.settings.WindowWidth == b.Width && a.settings.WindowHeight == b.Height {
		return
	}
	a.settings.WindowX, a.settings.WindowY = b.X, b.Y
	a.settings.WindowWidth, a.settings.WindowHeight = b.Width, b.Height
	a.settings.save()
}
