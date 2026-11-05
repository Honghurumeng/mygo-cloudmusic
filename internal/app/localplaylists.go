package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"

	"github.com/egoist/mygo"

	"netease-music-mygo/internal/netease"
)

// The listener's own playlists kept on this computer, which need no
// account: they live in playlists.json in the user data directory, their
// songs with the names, artists and covers the app shows of them.
//
// They have negative ids, which NetEase's never are, so a page, a row of
// the sidebar or a menu tells them apart and edits them here rather than
// there.

// localPlaylist is a playlist kept on this computer.
type localPlaylist struct {
	ID     int64           `json:"id"`
	Name   string          `json:"name"`
	Tracks []netease.Track `json:"tracks"`
	// Special is 5 for the songs the listener likes here, as NetEase marks
	// their liked songs' playlist: it is their hearts', not a list they
	// name, fill and delete.
	Special int `json:"special,omitempty"`
}

// likesLocalID names the playlist of the songs liked here: below zero, as
// every playlist kept here is, and below those made, so one made now never
// takes its id.
const likesLocalID int64 = -1 << 40

// likesName is what that playlist is called, as NetEase calls it.
const likesName = "我喜欢的音乐"

// localNote is what a local playlist's page says under its name.
const localNote = "保存在本机，不需要登录"

// listed returns the playlist as the sidebar and the menus show it: named,
// with its first song's cover, and without its songs.
func (l *localPlaylist) listed() netease.Playlist {
	pl := netease.Playlist{ID: l.ID, Name: l.Name, Description: localNote, SpecialType: l.Special}
	if len(l.Tracks) > 0 {
		pl.CoverURL = l.Tracks[0].Album.PicURL
	}
	return pl
}

// view returns the playlist as its page shows it: with its first song's
// cover, and its songs.
func (l *localPlaylist) view() netease.Playlist {
	pl := l.listed()
	pl.Tracks = slices.Clone(l.Tracks)
	return pl
}

// localStore is what playlists.json keeps: the playlists made here, and
// those saved here.
type localStore struct {
	Playlists []localPlaylist    `json:"playlists"`
	Saved     []netease.Playlist `json:"saved"`
}

func localPlaylistsPath() (string, error) {
	dir, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "playlists.json"), nil
}

// loadLocalPlaylists reads the playlists kept on this computer. It must run
// before the pages show.
func (a *app) loadLocalPlaylists() {
	path, err := localPlaylistsPath()
	if err != nil {
		return
	}
	a.localPath = path
	st := readLocalStore(path)
	a.local, a.savedHere = st.Playlists, st.Saved
	a.seedLiked()
}

// readLocalStore reads what is kept at path, or nothing.
func readLocalStore(path string) localStore {
	b, err := os.ReadFile(path)
	if err != nil {
		return localStore{}
	}
	var st localStore
	if json.Unmarshal(b, &st) != nil {
		return localStore{}
	}
	return st
}

// readLocalPlaylists reads the playlists made on this computer, or none.
func readLocalPlaylists(path string) []localPlaylist {
	return readLocalStore(path).Playlists
}

// saveLocalPlaylists keeps the playlists made and saved on this computer,
// where they were loaded from; those never loaded, as in tests, are not
// saved.
func (a *app) saveLocalPlaylists() {
	path := a.localPath
	if path == "" {
		return
	}
	b, err := json.MarshalIndent(localStore{Playlists: a.local, Saved: a.savedHere}, "", "  ")
	if err != nil {
		return
	}
	if os.MkdirAll(filepath.Dir(path), 0o700) != nil {
		return
	}
	if os.WriteFile(path+".part", b, 0o600) == nil {
		os.Rename(path+".part", path)
	}
}

// isLocal reports whether id names a playlist kept on this computer rather
// than one NetEase keeps.
func (a *app) isLocal(id int64) bool { return id < 0 }

// localIndex returns where the playlist id is kept, or -1.
func (a *app) localIndex(id int64) int {
	if !a.isLocal(id) {
		return -1
	}
	for i := range a.local {
		if a.local[i].ID == id {
			return i
		}
	}
	return -1
}

// allMine lists the listener's playlists: those NetEase keeps, then those
// kept on this computer.
func (a *app) allMine() []netease.Playlist {
	all := make([]netease.Playlist, 0, len(a.mine)+len(a.local))
	all = append(all, a.mine...)
	for i := range a.local {
		all = append(all, a.local[i].listed())
	}
	return all
}

// nextLocalID is the id the next playlist made takes: below every one kept,
// and below zero. The songs liked here keep an id of their own, which a
// playlist made never takes.
func (a *app) nextLocalID() int64 {
	id := int64(0)
	for i := range a.local {
		if a.local[i].ID != likesLocalID {
			id = min(id, a.local[i].ID)
		}
	}
	return id - 1
}

// likesIndex returns where the songs liked on this computer are kept, or -1.
func (a *app) likesIndex() int { return a.localIndex(likesLocalID) }

// likesList is 我喜欢的音乐 as this computer keeps it, made empty if it is
// not there yet.
func (a *app) likesList() *localPlaylist {
	if i := a.likesIndex(); i >= 0 {
		return &a.local[i]
	}
	a.local = slices.Insert(a.local, 0, localPlaylist{ID: likesLocalID, Name: likesName, Special: 5})
	return &a.local[0]
}

// seedLiked fills the hearts with the songs liked on this computer, for
// when there is no account to say which they are.
func (a *app) seedLiked() {
	a.liked = nil
	i := a.likesIndex()
	if i < 0 {
		return
	}
	a.liked = make(map[int64]bool, len(a.local[i].Tracks))
	for _, tr := range a.local[i].Tracks {
		a.liked[tr.ID] = true
	}
}

// likeHere likes tr on this computer, or unlikes it: it goes to the top of
// 我喜欢的音乐, or leaves it, and the hearts follow.
func (a *app) likeHere(tr netease.Track) {
	if a.liked == nil {
		a.liked = map[int64]bool{}
	}
	if a.liked[tr.ID] {
		if i := a.likesIndex(); i >= 0 {
			a.local[i].Tracks = slices.DeleteFunc(slices.Clone(a.local[i].Tracks), func(t netease.Track) bool { return t.ID == tr.ID })
			a.changedLocal(likesLocalID)
		}
		delete(a.liked, tr.ID)
		return
	}
	l := a.likesList()
	l.Tracks = slices.Insert(l.Tracks, 0, stored(tr))
	a.liked[tr.ID] = true
	a.changedLocal(likesLocalID)
}

// savedHereIndex returns where the playlist id is among those saved on this
// computer, or -1.
func (a *app) savedHereIndex(id int64) int {
	return slices.IndexFunc(a.savedHere, func(pl netease.Playlist) bool { return pl.ID == id })
}

// allSaved lists the playlists the listener saved: those NetEase keeps,
// then those kept on this computer. A playlist in both is listed once.
func (a *app) allSaved() []netease.Playlist {
	all := make([]netease.Playlist, 0, len(a.saved)+len(a.savedHere))
	all = append(all, a.saved...)
	for _, pl := range a.savedHere {
		if !slices.ContainsFunc(all, func(s netease.Playlist) bool { return s.ID == pl.ID }) {
			all = append(all, pl)
		}
	}
	return all
}

// saveHere keeps playlist pl on this computer, the newest first, as the
// sidebar lists them.
func (a *app) saveHere(pl netease.Playlist) {
	pl = savedCopy(pl)
	if i := a.savedHereIndex(pl.ID); i >= 0 {
		a.savedHere = slices.Delete(slices.Clone(a.savedHere), i, i+1)
	}
	a.savedHere = slices.Insert(a.savedHere, 0, pl)
	a.saveLocalPlaylists()
}

// unsaveHere stops keeping playlist id on this computer.
func (a *app) unsaveHere(id int64) {
	i := a.savedHereIndex(id)
	if i < 0 {
		return
	}
	a.savedHere = slices.Delete(slices.Clone(a.savedHere), i, i+1)
	a.saveLocalPlaylists()
}

// savedCopy returns pl as the store keeps it: named and covered for the
// sidebar, without its songs, and marked saved.
func savedCopy(pl netease.Playlist) netease.Playlist {
	pl.Tracks, pl.TrackIDs, pl.Preview = nil, nil, nil
	pl.Subscribed = true
	return pl
}

// newLocalPlaylist makes a playlist named name on this computer, with tr in
// it if set, and opens it.
func (a *app) newLocalPlaylist(name string, tr *netease.Track) {
	l := localPlaylist{ID: a.nextLocalID(), Name: name}
	if tr != nil {
		l.Tracks = []netease.Track{stored(*tr)}
	}
	a.local = slices.Insert(a.local, 0, l)
	a.saveLocalPlaylists()
	a.open(route{page: pagePlaylist, id: l.ID})
}

// stored returns tr as a playlist keeps it: its privilege is what NetEase
// said for whoever asked then, and would be wrong later.
func stored(tr netease.Track) netease.Track {
	tr.Privilege = nil
	tr.Reason = ""
	return tr
}

// addLocal puts tr at the top of the local playlist id, as NetEase puts a
// song added to one of its own, and reports whether it was not there
// already. A song added twice is listed once.
func (a *app) addLocal(id int64, tr *netease.Track) bool {
	i := a.localIndex(id)
	if i < 0 {
		return false
	}
	if slices.ContainsFunc(a.local[i].Tracks, func(t netease.Track) bool { return t.ID == tr.ID }) {
		return false
	}
	a.local[i].Tracks = slices.Insert(a.local[i].Tracks, 0, stored(*tr))
	a.changedLocal(id)
	return true
}

// removeLocal takes the song out of the local playlist id.
func (a *app) removeLocal(id, song int64) {
	i := a.localIndex(id)
	if i < 0 {
		return
	}
	a.local[i].Tracks = slices.DeleteFunc(slices.Clone(a.local[i].Tracks), func(t netease.Track) bool { return t.ID == song })
	a.changedLocal(id)
}

// dropLocal deletes the local playlist id for good: its songs with it, and
// its page leaves for the one before.
func (a *app) dropLocal(id int64) {
	i := a.localIndex(id)
	if i < 0 {
		return
	}
	a.local = slices.Delete(slices.Clone(a.local), i, i+1)
	delete(a.playlists, id)
	a.leavePlaylist(id)
	a.saveLocalPlaylists()
	a.toast = "已删除歌单"
}

// changedLocal keeps the local playlist id as it now is, and shows it so
// where its page may be reading it.
func (a *app) changedLocal(id int64) {
	if i := a.localIndex(id); i >= 0 && a.playlists[id] != nil {
		view := a.local[i].view()
		a.playlists[id] = &view
	}
	a.saveLocalPlaylists()
}
