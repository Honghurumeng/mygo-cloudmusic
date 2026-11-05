package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"

	"netease-music-mygo/internal/chksz"
	"netease-music-mygo/internal/netease"
	"netease-music-mygo/internal/player"
)

// ChKSz serves NetEase's songs without a session, which NetEase would
// otherwise want the listener to open by scanning a QR code. Its key is
// theirs: kept in the settings, or read from the environment or a key
// file, so that a build serves nobody else's.

// keyName is the name of the file a key may be kept in: in the app's data
// directory, the one it runs from, or the resources it ships with.
const keyName = "api.chksz.key"

// chkszKey returns the key to play through ChKSz with, or "" to play only
// as NetEase streams: what the listener entered, else one in the
// environment, else one in a key file.
func chkszKey(s settings) string {
	if s.NetEaseOnly {
		return ""
	}
	if k := strings.TrimSpace(s.ChKSzKey); k != "" {
		return k
	}
	if k := strings.TrimSpace(os.Getenv("CHKSZ_API_KEY")); k != "" {
		return k
	}
	for _, dir := range keyDirs() {
		if k := readKey(filepath.Join(dir, keyName)); k != "" {
			keepKey(dir, k)
			return k
		}
	}
	return ""
}

// keyDirs returns the directories a key file may be kept in: the app's
// data directory, the resources it ships with, the one it runs from, and
// the one its binary sits in. A build run from elsewhere than the
// directory it was built in finds nothing without the last two.
func keyDirs() []string {
	var dirs []string
	for _, name := range []mygo.PathName{mygo.PathUserData, mygo.PathResources} {
		if dir, err := mygo.App.Path(name); err == nil {
			dirs = append(dirs, dir)
		}
	}
	if exe, err := os.Executable(); err == nil {
		dirs = append(dirs, filepath.Dir(exe))
	}
	if dir, err := os.Getwd(); err == nil {
		dirs = append(dirs, dir)
	}
	return dirs
}

// keepKey copies a key found beside the binary or in the directory the app
// runs from into its data directory, where it is found however the app
// starts next time: a build launched from another directory would
// otherwise play only as NetEase streams, which is nothing without a
// session.
func keepKey(from, key string) {
	if from == "" {
		return
	}
	dir, err := mygo.App.Path(mygo.PathUserData)
	if err != nil || dir == from {
		return
	}
	path := filepath.Join(dir, keyName)
	if _, err := os.Stat(path); err == nil {
		return
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return
	}
	os.WriteFile(path, []byte(key+"\n"), 0o600)
}

// readKey returns the key kept in path, or "".
func readKey(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// newChKSz returns the client that resolves streams through ChKSz, or nil
// when it is off or there is no key, and songs stream as NetEase gives
// them.
func newChKSz(s settings) *chksz.Client {
	if key := chkszKey(s); key != "" {
		return chksz.New(key)
	}
	return nil
}

// errNoSource is why a song does not play when nobody is signed in and the
// ChKSz key is missing: NetEase streams nothing without a session, so the
// key is the only way, and the listener is told where it goes rather than
// that the song needs VIP, which is not why it did not play.
var errNoSource = errors.New("未登录，且没有 ChKSz 的 Key：在设置 → 音源里填一个，或扫码登录")

// streamURL returns where a track streams from at level, as the settings
// say. NetEase goes first, signed in or not, and what it won't play ChKSz
// is asked for, which spends the key's quota: the songs NetEase keeps for
// subscribers, or asks a QR code for, are the ones it answers.
func (a *app) streamURL(id int64, level string) (*netease.Stream, error) {
	st, err := netease.SongURL(id, level)
	if err == nil {
		return st, nil
	}
	if a.chksz == nil {
		return nil, noSourceWhy(err, a.profile != nil)
	}
	alt, played := a.chksz.SongURL(id, level)
	if played == nil {
		return alt, nil
	}
	var blocked *chksz.BlockedError
	if errors.As(played, &blocked) {
		// Nothing ChKSz plays would come of asking again for a while.
		// Signed in, NetEase still plays what it has, so the queue goes
		// on; signed out, every song needs ChKSz, so it stops.
		if a.profile == nil {
			return nil, &player.StopError{Err: played}
		}
		return nil, err
	}
	// ChKSz's reason is the truer one: NetEase says a song it keeps for
	// subscribers is unavailable, which is not why it did not play.
	return nil, played
}

// noSourceWhy returns why a song NetEase won't play has nowhere else to go,
// or NetEase's own reason when it has: a session, or ChKSz.
func noSourceWhy(err error, signedIn bool) error {
	if !signedIn && errors.Is(err, netease.ErrUnavailable) {
		return errNoSource
	}
	return err
}

// chkszKeyDesc says what the key field of the settings is for, and where
// the app reads a key from when it is empty, or that it has none.
func (a *app) chkszKeyDesc(s *settings) string {
	if s.NetEaseOnly {
		return "ChKSz 音源关着，用不到 Key"
	}
	if a.chksz == nil {
		return "还没有可用的 Key：在 api.chksz.com 申请后填在这里，或存成 api.chksz.key"
	}
	if strings.TrimSpace(s.ChKSzKey) == "" {
		return "已从环境变量或 api.chksz.key 读到 Key；填在这里的优先"
	}
	return "在 api.chksz.com 申请；留空时读取 CHKSZ_API_KEY 或 api.chksz.key 文件"
}
