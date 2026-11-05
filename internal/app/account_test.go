package app

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/netease"
)

// fakeSubscriptions answers for the artists and podcasts the listener
// follows, and tells on each time they are asked for.
func fakeSubscriptions(t *testing.T) chan struct{} {
	asked := make(chan struct{}, 8)
	ars, pods := followedArtists, subscribedPodcasts
	t.Cleanup(func() { followedArtists, subscribedPodcasts = ars, pods })
	followedArtists = func() ([]netease.ArtistProfile, error) { asked <- struct{}{}; return nil, nil }
	subscribedPodcasts = func() ([]netease.Podcast, error) { return nil, nil }
	return asked
}

// ownPageApp is signed in as user 9, whose page has what they follow.
func ownPageApp() *app {
	a := newTestApp()
	a.settings = defaultSettings()
	a.profile = &netease.Profile{UserID: 9, Nickname: "听歌的人"}
	user := netease.User{UserID: 9, Nickname: "听歌的人", Signature: "在音乐里，寻找同路的人。", Follows: 12, Followeds: 24}
	a.users[9] = &userData{
		user:      &user,
		playlists: []netease.Playlist{{ID: 30, Name: "通勤路上", TrackCount: 40}},
		artists:   []netease.ArtistProfile{{Artist: netease.Artist{ID: 7, Name: "MyGO!!!!!"}}, {Artist: netease.Artist{ID: 10, Name: "Ave Mujica"}}},
		podcasts: []netease.Podcast{
			{ID: 5, Name: "夜航电台", DJ: netease.User{Nickname: "主播"}, ProgramCount: 120, Description: "每晚陪你说说话。"},
			{ID: 6, Name: "通勤十分钟", ProgramCount: 48},
		},
		artistsShown: true, podcastsShown: true,
	}
	return a
}

// The listener's own page asks again for what they follow each time it
// shows, as they may have followed someone since, and a followed artist
// opens their page.
func TestOwnPageFollows(t *testing.T) {
	asked := fakeSubscriptions(t)
	a := ownPageApp()
	a.open(route{page: pageUser, id: 9})
	wait(t, asked, "the artists followed")
	tt := ui.NewTester(a.view, 1200, 1200)
	tt.Frame()
	if err := tt.Click("Ave Mujica"); err != nil {
		t.Fatal(err)
	}
	if a.route.page != pageArtist || a.route.id != 10 {
		t.Fatalf("route = %+v, want the page of artist 10", a.route)
	}
	a.users[9].subsLoading = 0 // what was asked for has landed
	a.back()
	wait(t, asked, "the artists followed, again")
}

// Someone else's page doesn't ask for what the listener follows, nor shows
// it.
func TestOthersPageHidesFollows(t *testing.T) {
	asked := fakeSubscriptions(t)
	a := ownPageApp()
	a.profile = &netease.Profile{UserID: 1}
	a.open(route{page: pageUser, id: 9})
	tt := ui.NewTester(a.view, 1200, 1200)
	tt.Frame()
	if len(asked) > 0 {
		t.Fatal("asked for what the listener follows")
	}
	if err := tt.Click("Ave Mujica"); err == nil {
		t.Fatal("the listener's artists showed on someone else's page")
	}
}

// The listener's own page asks for no frames once shown.
func TestOwnPageFrameBudget(t *testing.T) {
	fakeSubscriptions(t)
	a := ownPageApp()
	a.open(route{page: pageUser, id: 9})
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Advance(time.Second)
	tt.Frame()
	tt.Advance(time.Second)
	if tt.FramePending() {
		t.Error("the page asks for frames while idle")
	}
}

// With NM_FRAMES set, saves the listener's own page, scrolled to what they
// follow, in the light and the dark.
func TestOwnPageFrames(t *testing.T) {
	dir := os.Getenv("NM_FRAMES")
	if dir == "" {
		t.Skip()
	}
	fakeSubscriptions(t)
	for _, dark := range []bool{false, true} {
		a := ownPageApp()
		a.open(route{page: pageUser, id: 9})
		tt := ui.NewTester(a.view, 1200, 1200)
		tt.SetDark(dark)
		tt.Advance(time.Second)
		tt.Frame()
		name := "user-own.png"
		if dark {
			name = "user-own-dark.png"
		}
		f, _ := os.Create(filepath.Join(dir, name))
		png.Encode(f, tt.Image())
		f.Close()
	}
}
