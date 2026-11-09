package app

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/netease"
)

// longBio is longer than the two lines a page shows of a description.
var longBio = strings.Repeat("2005年，因参加选秀节目《我型我秀》而正式出道。", 20)

// bioPages is a page of each kind that shows a description, with the page
// opened onto it.
var bioPages = []struct {
	name string
	open func(a *app) route
}{
	{"artist", func(a *app) route {
		a.artists[1] = &artistData{profile: &netease.ArtistProfile{
			Artist:    netease.Artist{ID: 1, Name: "甲"},
			BriefDesc: longBio,
		}}
		return route{page: pageArtist, id: 1}
	}},
	{"album", func(a *app) route {
		a.albums[2] = &albumData{album: &netease.AlbumProfile{
			Album:       netease.Album{ID: 2, Name: "专辑"},
			Description: longBio,
		}}
		return route{page: pageAlbum, id: 2}
	}},
	{"playlist", func(a *app) route {
		a.playlists[3] = &netease.Playlist{ID: 3, Name: "歌单", Description: longBio}
		return route{page: pagePlaylist, id: 3}
	}},
	{"podcast", func(a *app) route {
		a.podcasts[4] = &podcastData{
			podcast: &netease.Podcast{ID: 4, Name: "播客", Description: longBio},
			page:    paged{loaded: true},
		}
		return route{page: pagePodcast, id: 4}
	}},
	{"user", func(a *app) route {
		a.users[5] = &userData{user: &netease.User{UserID: 5, Nickname: "听歌的人", Signature: longBio}}
		return route{page: pageUser, id: 5}
	}},
}

// A description longer than the two lines a page shows of it opens over the
// rest of itself, and closes again, on every page that shows one: a long
// one, as 薛之谦's biography, used to be cut off with no way to read the
// rest.
func TestDescriptionsOpen(t *testing.T) {
	for _, page := range bioPages {
		t.Run(page.name, func(t *testing.T) {
			a := newTestApp()
			a.settings = defaultSettings()
			r := page.open(a)
			a.open(r)
			tt := ui.NewTester(a.view, 1200, 800)
			tt.Frame()

			if err := tt.Click("展开"); err != nil {
				t.Fatalf("the description was cut without a way to read the rest: %v", err)
			}
			tt.Frame()
			if !a.bioOpen[r] {
				t.Fatal("展开 left the description cut")
			}
			if err := tt.Click("收起"); err != nil {
				t.Fatalf("an open description had no way to close it: %v", err)
			}
			tt.Frame()
			if a.bioOpen[r] {
				t.Error("收起 left the description open")
			}
		})
	}
}

// A description the two lines hold offers no way to open it.
func TestDescriptionThatFits(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	a.artists[12] = &artistData{profile: &netease.ArtistProfile{
		Artist:    netease.Artist{ID: 12, Name: "乙"},
		BriefDesc: "中国内地流行乐男歌手。",
	}}
	a.open(route{page: pageArtist, id: 12})
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	if err := tt.Click("展开"); err == nil {
		t.Error("a description that fits offered to open itself")
	}
}

// Each page keeps its own description as it was left: opening one leaves
// the pages gone to after it closed, and coming back finds it open.
func TestDescriptionOpenStaysWithItsPage(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	artist, album := bioPages[0], bioPages[1]
	artistRoute, albumRoute := artist.open(a), album.open(a)
	a.open(artistRoute)
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	if err := tt.Click("展开"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()

	a.open(albumRoute)
	tt.Frame()
	if a.bioOpen[albumRoute] {
		t.Error("the album's description opened with the artist's")
	}
	if !a.bioOpen[artistRoute] {
		t.Error("leaving the artist's page closed their biography")
	}

	a.open(artistRoute)
	tt.Frame()
	if err := tt.Click("收起"); err != nil {
		t.Fatalf("coming back found the biography closed: %v", err)
	}
}
