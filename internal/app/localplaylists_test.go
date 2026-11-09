package app

import (
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/netease"
)

// localTestApp is signed out, keeps its playlists in a store of its own, and
// shows a home page that asks nothing of the network.
func localTestApp(t *testing.T) *app {
	t.Helper()
	a := newTestApp()
	a.settings = defaultSettings()
	a.localPath = filepath.Join(t.TempDir(), "playlists.json")
	a.homeLoading = true
	return a
}

// Signed out, the sidebar offers to make a playlist; the one made opens
// empty, and is kept on this computer.
func TestCreatePlaylistSignedOut(t *testing.T) {
	a := localTestApp(t)
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	if err := tt.Click("创建第一个歌单"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(300 * time.Millisecond)
	tt.Frame()
	tt.Type("通勤")
	tt.Key(0, ui.KeyEnter)
	tt.Frame()

	if len(a.local) != 1 || a.local[0].Name != "通勤" {
		t.Fatalf("the playlists kept are %+v, want one named 通勤", a.local)
	}
	if id := a.local[0].ID; !a.isLocal(id) || a.route != (route{page: pagePlaylist, id: id}) {
		t.Errorf("on %+v, want the page of the playlist made (%d)", a.route, id)
	}
	if _, ok := tt.Find("通勤"); !ok {
		t.Error("the sidebar does not list the playlist made")
	}
	if kept := readLocalPlaylists(a.localPath); len(kept) != 1 || kept[0].Name != "通勤" {
		t.Errorf("playlists.json holds %+v, want the playlist made", kept)
	}
}

// Choosing 新建歌单… from a song's menu signed out makes a playlist kept on
// this computer, with the song in it.
func TestCreatePlaylistForSongSignedOut(t *testing.T) {
	a := localTestApp(t)
	a.local = []localPlaylist{{ID: -1, Name: "通勤", Tracks: []netease.Track{
		{ID: 2, Name: "宝船", Artists: []netease.Artist{{Name: "DOUDOU"}}, Duration: 200000},
	}}}
	a.show(route{page: pagePlaylist, id: -1})
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	if err := tt.RightClick("宝船"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("添加到歌单", "新建歌单…"); err != nil {
		t.Fatalf("%v; the menu shows %q", err, tt.Menu())
	}
	tt.Advance(300 * time.Millisecond)
	tt.Frame()
	tt.Type("睡前")
	tt.Key(0, ui.KeyEnter)
	tt.Frame()

	if len(a.local) != 2 || a.local[0].Name != "睡前" {
		t.Fatalf("the playlists kept are %+v, want 睡前 made", a.local)
	}
	if got := a.local[0].Tracks; len(got) != 1 || got[0].ID != 2 {
		t.Errorf("the playlist made holds %+v, want song 2", got)
	}
}

// Signed out, a song's menu puts the song at the top of a playlist kept on
// this computer, and keeps it there.
func TestAddSongToLocalPlaylist(t *testing.T) {
	a := localTestApp(t)
	a.local = []localPlaylist{
		{ID: -1, Name: "通勤", Tracks: []netease.Track{
			{ID: 1, Name: "一张相片", Duration: 200000},
		}},
		{ID: -2, Name: "睡前", Tracks: []netease.Track{
			{ID: 3, Name: "旧歌", Duration: 200000},
		}},
	}
	a.show(route{page: pagePlaylist, id: -1})
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	if err := tt.RightClick("一张相片"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("添加到歌单", "睡前"); err != nil {
		t.Fatalf("%v; the menu shows %q", err, tt.Menu())
	}
	tt.Frame()

	kept := a.local[a.localIndex(-2)].Tracks
	if len(kept) != 2 || kept[0].ID != 1 || kept[1].ID != 3 {
		t.Errorf("睡前 holds %+v, want song 1 at its top", kept)
	}
	if got := readLocalPlaylists(a.localPath); len(got) != 2 || len(got[1].Tracks) != 2 || got[1].Tracks[0].ID != 1 {
		t.Errorf("playlists.json holds %+v, want the song in 睡前", got)
	}
}

// Removing a song from a playlist kept on this computer takes its row away,
// and keeps it out of the playlist.
func TestRemoveSongFromLocalPlaylist(t *testing.T) {
	a := localTestApp(t)
	a.local = []localPlaylist{{ID: -1, Name: "通勤", Tracks: []netease.Track{
		{ID: 1, Name: "一张相片", Duration: 200000},
		{ID: 2, Name: "宝船", Duration: 200000},
	}}}
	a.show(route{page: pagePlaylist, id: -1})
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	if err := tt.RightClick("宝船"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("从歌单中移除"); err != nil {
		t.Fatalf("%v; the menu shows %q", err, tt.Menu())
	}
	tt.Frame()

	if _, ok := tt.Find("宝船"); ok {
		t.Error("the song's row is still listed")
	}
	if got := readLocalPlaylists(a.localPath); len(got) != 1 || len(got[0].Tracks) != 1 || got[0].Tracks[0].ID != 1 {
		t.Errorf("playlists.json holds %+v, want the song gone", got)
	}
}

// Renaming a playlist kept on this computer shows its new name at once,
// keeps it, and asks NetEase nothing.
func TestRenameLocalPlaylist(t *testing.T) {
	renamePlaylist = func(int64, string) error {
		t.Error("asked NetEase to rename a playlist kept on this computer")
		return nil
	}
	defer func() { renamePlaylist = netease.RenamePlaylist }()

	a := localTestApp(t)
	a.local = []localPlaylist{{ID: -1, Name: "通勤"}}
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	if err := tt.RightClick("通勤"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("重命名…"); err != nil {
		t.Fatalf("%v; the menu shows %q", err, tt.Menu())
	}
	tt.Advance(300 * time.Millisecond)
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("助眠")
	if err := tt.Click("保存"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()

	if a.local[0].Name != "助眠" {
		t.Errorf("the playlist is named %q", a.local[0].Name)
	}
	if _, ok := tt.Find("助眠"); !ok {
		t.Error("the new name does not show")
	}
	if got := readLocalPlaylists(a.localPath); len(got) != 1 || got[0].Name != "助眠" {
		t.Errorf("playlists.json holds %+v, want the new name", got)
	}
}

// Deleting a playlist kept on this computer, once confirmed, takes it from
// the sidebar and leaves its page for the one before.
func TestDeleteLocalPlaylist(t *testing.T) {
	a := localTestApp(t)
	a.local = []localPlaylist{{ID: -1, Name: "通勤"}}
	a.route = route{page: pageHome}
	a.open(route{page: pagePlaylist, id: -1})
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	if err := tt.Click("更多"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("删除歌单…"); err != nil {
		t.Fatalf("%v; the menu shows %q", err, tt.Menu())
	}
	tt.Advance(300 * time.Millisecond)
	tt.Frame()
	if len(a.local) != 1 {
		t.Fatal("deleted the playlist before it was confirmed")
	}
	if err := tt.Click("删除"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()

	if len(a.local) != 0 {
		t.Errorf("the playlists kept are %+v, want none", a.local)
	}
	if a.route.page != pageHome || len(a.history) != 0 || len(a.forward) != 0 {
		t.Errorf("on %+v, history %v, forward %v; want the home page with nowhere to go", a.route, a.history, a.forward)
	}
	if kept := readLocalPlaylists(a.localPath); len(kept) != 0 {
		t.Errorf("playlists.json holds %+v, want none", kept)
	}
}

// The page of a playlist kept on this computer lists its songs from the
// store, after the pages were cleared as signing in or out clears them.
func TestLocalPlaylistPageAfterPagesCleared(t *testing.T) {
	a := localTestApp(t)
	a.local = []localPlaylist{{ID: -1, Name: "通勤", Tracks: []netease.Track{
		{ID: 1, Name: "一张相片", Duration: 200000},
	}}}
	// What signing in or out does to what the pages had loaded.
	a.playlists = map[int64]*netease.Playlist{}
	a.show(route{page: pagePlaylist, id: -1})
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()

	if _, ok := tt.Find("一张相片"); !ok {
		t.Error("the page of the playlist kept lists none of its songs")
	}
	if pl := a.playlists[-1]; pl == nil || len(pl.Tracks) != 1 {
		t.Errorf("the playlist loaded as %+v, want its song", pl)
	}
}

// A playlist kept on this computer is written down with its songs, their
// artists and covers, and read back as it was.
func TestLocalPlaylistsRoundTrip(t *testing.T) {
	a := newTestApp()
	a.localPath = filepath.Join(t.TempDir(), "playlists.json")
	a.local = []localPlaylist{{ID: -1, Name: "通勤", Tracks: []netease.Track{{
		ID: 186016, Name: "宝船", Artists: []netease.Artist{{ID: 7, Name: "DOUDOU"}},
		Album:    netease.Album{ID: 9, Name: "宝船", PicURL: "https://p1.music.126.net/x.jpg"},
		Duration: 200000,
	}}}}
	a.saveLocalPlaylists()

	got := readLocalPlaylists(a.localPath)
	if len(got) != 1 || got[0].ID != -1 || got[0].Name != "通勤" || len(got[0].Tracks) != 1 {
		t.Fatalf("read %+v, want the playlist saved", got)
	}
	tr := got[0].Tracks[0]
	if tr.ID != 186016 || tr.Name != "宝船" || tr.Duration != 200000 ||
		len(tr.Artists) != 1 || tr.Artists[0].Name != "DOUDOU" || tr.Album.PicURL == "" {
		t.Errorf("the song came back as %+v", tr)
	}
}

// Signed out, a song's heart likes it in 我喜欢的音乐, kept on this
// computer: no account is asked for, and it is written down.
func TestLikeSongSignedOut(t *testing.T) {
	a := localTestApp(t)
	tr := netease.Track{ID: 7, Name: "宝船", Artists: []netease.Artist{{Name: "DOUDOU"}}, Duration: 200000}
	a.toggleLike(&tr)

	if a.login.open {
		t.Error("asked the listener to sign in")
	}
	if !a.isLiked(&tr) {
		t.Error("the heart does not read liked")
	}
	i := a.likesIndex()
	if i < 0 || a.local[i].Name != likesName || a.local[i].Special != 5 {
		t.Fatalf("the liked songs are kept as %+v, want 我喜欢的音乐", a.local)
	}
	if got := a.local[i].Tracks; len(got) != 1 || got[0].ID != 7 || got[0].Name != "宝船" {
		t.Errorf("我喜欢的音乐 holds %+v, want the song liked", got)
	}
	if kept := readLocalStore(a.localPath).Playlists; len(kept) != 1 || len(kept[0].Tracks) != 1 {
		t.Errorf("playlists.json holds %+v, want the song liked", kept)
	}
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	if _, ok := tt.Find(likesName); !ok {
		t.Error("the sidebar does not list 我喜欢的音乐")
	}
}

// Unliking a song signed out takes it out of 我喜欢的音乐 again, and out of
// the store.
func TestUnlikeSongSignedOut(t *testing.T) {
	a := localTestApp(t)
	a.local = []localPlaylist{{ID: likesLocalID, Name: likesName, Special: 5, Tracks: []netease.Track{
		{ID: 7, Name: "宝船", Duration: 200000},
		{ID: 8, Name: "一张相片", Duration: 200000},
	}}}
	a.seedLiked()
	a.toggleLike(&netease.Track{ID: 7, Name: "宝船"})

	if a.isLiked(&netease.Track{ID: 7}) {
		t.Error("the heart still reads liked")
	}
	if !a.isLiked(&netease.Track{ID: 8}) {
		t.Error("unliking one song unliked another")
	}
	i := a.likesIndex()
	if got := a.local[i].Tracks; len(got) != 1 || got[0].ID != 8 {
		t.Errorf("我喜欢的音乐 holds %+v, want song 7 gone", got)
	}
	if kept := readLocalStore(a.localPath).Playlists; len(kept) != 1 || len(kept[0].Tracks) != 1 {
		t.Errorf("playlists.json holds %+v, want song 7 gone", kept)
	}
}

// The songs liked here open as a playlist of their own, which offers
// neither renaming nor deleting nor a heart mode: they are the hearts'.
func TestLikesPlaylistPage(t *testing.T) {
	a := localTestApp(t)
	a.local = []localPlaylist{{ID: likesLocalID, Name: likesName, Special: 5, Tracks: []netease.Track{
		{ID: 7, Name: "宝船", Artists: []netease.Artist{{Name: "DOUDOU"}}, Duration: 200000},
	}}}
	a.seedLiked()
	a.show(route{page: pagePlaylist, id: likesLocalID})
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()

	if _, ok := tt.Find("宝船"); !ok {
		t.Error("the page lists none of the songs liked")
	}
	for _, no := range []string{"更多", "心动模式", "收藏", "已收藏"} {
		if _, ok := tt.Find(no); ok {
			t.Errorf("the likes page offers %q", no)
		}
	}
}

// A playlist made after some songs were liked takes an id of its own: the
// liked songs' is no playlist's to take.
func TestPlaylistMadeAfterLikes(t *testing.T) {
	a := localTestApp(t)
	a.local = []localPlaylist{{ID: likesLocalID, Name: likesName, Special: 5}}
	if got := a.nextLocalID(); got != -1 {
		t.Errorf("the next playlist takes id %d, want -1", got)
	}
	a.newLocalPlaylist("通勤", nil)
	if a.likesIndex() < 0 || a.local[a.localIndex(-1)].Name != "通勤" {
		t.Errorf("the playlists kept are %+v", a.local)
	}
}

// Clicking 收藏 on someone else's playlist signed out keeps it on this
// computer: no login is asked for, the sidebar lists it, and it is written
// down.
func TestSavePlaylistSignedOut(t *testing.T) {
	savePlaylist = func(int64, bool) error {
		t.Error("asked NetEase to save a playlist with no account")
		return nil
	}
	defer func() { savePlaylist = netease.SubscribePlaylist }()

	a := localTestApp(t)
	pl := &netease.Playlist{ID: 30, Name: "华语经典", TrackCount: 2}
	pl.Creator.UserID = 2
	a.playlists[30] = pl
	a.route = route{page: pagePlaylist, id: 30}
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	if err := tt.Click("收藏"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()

	if a.login.open {
		t.Error("asked the listener to sign in")
	}
	if len(a.savedHere) != 1 || a.savedHere[0].ID != 30 || a.savedHere[0].Name != "华语经典" {
		t.Fatalf("saved here are %+v, want playlist 30", a.savedHere)
	}
	if !a.isSaved(a.playlists[30]) {
		t.Error("the playlist does not read saved")
	}
	if _, ok := tt.Find("已收藏"); !ok {
		t.Error("the page still offers to save it")
	}
	if kept := readLocalStore(a.localPath).Saved; len(kept) != 1 || kept[0].ID != 30 {
		t.Errorf("playlists.json holds %+v, want playlist 30 saved", kept)
	}
}

// Clicking 已收藏 takes a playlist saved on this computer out of it again,
// and out of the sidebar.
func TestUnsavePlaylistSignedOut(t *testing.T) {
	savePlaylist = func(int64, bool) error {
		t.Error("asked NetEase about a playlist saved on this computer")
		return nil
	}
	defer func() { savePlaylist = netease.SubscribePlaylist }()

	a := localTestApp(t)
	a.savedHere = []netease.Playlist{{ID: 30, Name: "华语经典", Subscribed: true}}
	a.playlists[30] = &netease.Playlist{ID: 30, Name: "华语经典", Subscribed: true}
	a.route = route{page: pagePlaylist, id: 30}
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	if err := tt.Click("已收藏"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()

	if a.login.open {
		t.Error("asked the listener to sign in")
	}
	if len(a.savedHere) != 0 || a.isSaved(a.playlists[30]) {
		t.Errorf("still saved here: %+v", a.savedHere)
	}
	if kept := readLocalStore(a.localPath).Saved; len(kept) != 0 {
		t.Errorf("playlists.json holds %+v, want none saved", kept)
	}
}

// A playlist saved on this computer is listed under the sidebar's saved
// ones signed out, and its 取消收藏 keeps NetEase out of it.
func TestSavedHereListedInSidebar(t *testing.T) {
	savePlaylist = func(int64, bool) error {
		t.Error("asked NetEase about a playlist saved on this computer")
		return nil
	}
	defer func() { savePlaylist = netease.SubscribePlaylist }()

	a := localTestApp(t)
	a.savedHere = []netease.Playlist{{ID: 20, Name: "飙升榜", Subscribed: true}}
	a.playlists[20] = &netease.Playlist{ID: 20, Name: "飙升榜", Subscribed: true}
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	if err := tt.RightClick("飙升榜"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("取消收藏"); err != nil {
		t.Fatalf("%v; the menu shows %q", err, tt.Menu())
	}
	tt.Frame()

	if len(a.savedHere) != 0 {
		t.Errorf("saved here are %+v, want none", a.savedHere)
	}
	if a.login.open {
		t.Error("asked the listener to sign in")
	}
}

// A playlist saved on this computer is written down with its name, cover
// and creator, without its songs, and read back as it was.
func TestSavedPlaylistsRoundTrip(t *testing.T) {
	a := newTestApp()
	a.localPath = filepath.Join(t.TempDir(), "playlists.json")
	pl := netease.Playlist{ID: 30, Name: "华语经典", CoverURL: "https://p1.music.126.net/x.jpg", TrackCount: 40}
	pl.Creator.Nickname = "云音乐"
	a.saveHere(pl)

	st := readLocalStore(a.localPath)
	if len(st.Saved) != 1 || st.Saved[0].ID != 30 || st.Saved[0].Name != "华语经典" {
		t.Fatalf("read %+v, want the playlist saved", st.Saved)
	}
	got := st.Saved[0]
	if got.Cover() == "" || got.Creator.Nickname != "云音乐" || got.TrackCount != 40 || !got.Subscribed {
		t.Errorf("the playlist came back as %+v", got)
	}
	if got.Tracks != nil || got.TrackIDs != nil {
		t.Errorf("the playlist kept its songs: %+v", got)
	}
}

// With NM_FRAMES set, saves the sidebar of someone signed out, with no
// playlist and then with the ones kept on this computer, and the page of
// one, to look at.
func TestLocalPlaylistFrames(t *testing.T) {
	dir := os.Getenv("NM_FRAMES")
	if dir == "" {
		t.Skip()
	}
	a := localTestApp(t)
	a.homeLoading = true
	songs := []netease.Track{
		{ID: 1, Name: "一张相片", Artists: []netease.Artist{{Name: "林宥嘉"}}, Duration: 200000},
		{ID: 2, Name: "宝船", Artists: []netease.Artist{{Name: "DOUDOU"}}, Duration: 200000},
	}
	a.local = []localPlaylist{
		{ID: -1, Name: "通勤", Tracks: songs},
		{ID: -2, Name: "睡前", Tracks: songs[:1]},
	}
	tt := ui.NewTester(a.view, 1200, 800)
	save := func(name string) {
		tt.Advance(400 * time.Millisecond)
		tt.Frame()
		f, _ := os.Create(filepath.Join(dir, name+".png"))
		defer f.Close()
		png.Encode(f, tt.Image())
	}
	for _, dark := range []bool{false, true} {
		suffix := "-light"
		if dark {
			suffix = "-dark"
		}
		tt.SetDark(dark)
		a.local, a.savedHere = nil, nil
		save("local-empty" + suffix)
		a.local = []localPlaylist{
			{ID: -1, Name: "通勤", Tracks: songs},
			{ID: -2, Name: "睡前", Tracks: songs[:1]},
		}
		a.show(route{page: pagePlaylist, id: -1})
		save("local-playlist" + suffix)
		// A playlist saved on this computer, signed out: off the network,
		// so its songs are put here.
		saved := &netease.Playlist{ID: 30, Name: "华语经典", Subscribed: true, Tracks: songs}
		saved.Creator.Nickname = "云音乐"
		a.savedHere = []netease.Playlist{*saved}
		a.playlists[30] = saved
		a.show(route{page: pagePlaylist, id: 30})
		save("saved-playlist" + suffix)
		// The songs liked here, as their page shows them.
		a.local = []localPlaylist{{ID: likesLocalID, Name: likesName, Special: 5, Tracks: songs}}
		a.seedLiked()
		a.show(route{page: pagePlaylist, id: likesLocalID})
		save("likes-playlist" + suffix)
		a.local, a.savedHere = nil, nil
		a.seedLiked()
		delete(a.playlists, 30)
		a.show(route{page: pageHome})
	}
}

// Saving a playlist keeps the ids of its songs, which outlive the app, so
// the cache's list knows which of its songs the playlist has; unsaving it
// forgets them.
func TestSavedPlaylistKeepsItsSongs(t *testing.T) {
	a := localTestApp(t)
	pl := netease.Playlist{ID: 42, Name: "睡前", Tracks: []netease.Track{{ID: 7}, {ID: 8}}}
	a.toggleSavedHere(pl)
	if got := a.savedSongs[42]; !slices.Equal(got, []int64{7, 8}) {
		t.Fatalf("kept %v, want the songs of the playlist", got)
	}
	if got := readLocalStore(a.localPath).Songs[42]; !slices.Equal(got, []int64{7, 8}) {
		t.Errorf("playlists.json keeps %v, want the songs of the playlist", got)
	}

	a.toggleSavedHere(pl)
	if _, known := a.savedSongs[42]; known {
		t.Error("kept the songs of a playlist no longer saved")
	}
	if _, known := readLocalStore(a.localPath).Songs[42]; known {
		t.Error("playlists.json keeps the songs of a playlist no longer saved")
	}
}

// The cache's list says a song is in the playlists that keep it: those saved
// here, whose songs were kept as they were saved, and those kept on this
// computer. The liked songs' own playlist says nothing of it, their hearts
// do.
func TestHeldByPlaylists(t *testing.T) {
	a := localTestApp(t)
	a.local = []localPlaylist{
		{ID: likesLocalID, Name: likesName, Special: 5, Tracks: []netease.Track{{ID: 1}}},
		{ID: -2, Name: "通勤", Tracks: []netease.Track{{ID: 2}}},
	}
	a.saved = []netease.Playlist{{ID: 42, Name: "睡前"}}
	a.savedSongs = map[int64][]int64{42: {3}}

	held := a.heldBy()
	if got := held[2]; !slices.Equal(got, []string{"通勤"}) {
		t.Errorf("song 2 is in %v, want 通勤", got)
	}
	if got := held[3]; !slices.Equal(got, []string{"睡前"}) {
		t.Errorf("song 3 is in %v, want 睡前", got)
	}
	if got, listed := held[1]; listed {
		t.Errorf("the liked songs are listed as a playlist of their own: %v", got)
	}
}
