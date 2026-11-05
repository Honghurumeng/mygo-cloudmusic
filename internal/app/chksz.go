package app

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"

	"netease-music-mygo/internal/chksz"
	"netease-music-mygo/internal/netease"
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
	for _, name := range []mygo.PathName{mygo.PathUserData, mygo.PathResources} {
		if dir, err := mygo.App.Path(name); err == nil {
			if k := readKey(filepath.Join(dir, keyName)); k != "" {
				return k
			}
		}
	}
	if dir, err := os.Getwd(); err == nil {
		return readKey(filepath.Join(dir, keyName))
	}
	return ""
}

// readKey returns the key kept in path, or "".
func readKey(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// streamURL returns where a track streams from at level, as the settings
// say. Nobody signed in, ChKSz plays it, which NetEase would only play a
// preview of, or nothing. Signed in, NetEase plays what it may, and ChKSz
// plays what it keeps for subscribers, where it can.
func (a *app) streamURL(id int64, level string) (*netease.Stream, error) {
	if a.chksz == "" {
		return netease.SongURL(id, level)
	}
	if a.profile == nil {
		return chksz.SongURL(id, level, a.chksz)
	}
	st, err := netease.SongURL(id, level)
	if err == nil {
		return st, nil
	}
	if st, played := chksz.SongURL(id, level, a.chksz); played == nil {
		return st, nil
	}
	return nil, err
}

// canPlay reports whether the listener may play tr now: as NetEase lets
// them, or through ChKSz, which plays what NetEase keeps from them, and
// says nothing about what it may not play until it is asked. NetEase
// marks songs nobody is signed in may not play as taken down, which ChKSz
// plays all the same: they are not dimmed.
func (a *app) canPlay(tr *netease.Track) bool {
	return tr.Playable() || a.chksz != ""
}

// chkszKeyDesc says what the key field of the settings is for, and where
// the app reads a key from when it is empty, or that it has none.
func (a *app) chkszKeyDesc(s settings) string {
	if s.NetEaseOnly {
		return "ChKSz 音源关着，用不到 Key"
	}
	if a.chksz == "" {
		return "还没有可用的 Key：在 api.chksz.com 申请后填在这里，或存成 api.chksz.key"
	}
	if strings.TrimSpace(s.ChKSzKey) == "" {
		return "已从环境变量或 api.chksz.key 读到 Key；填在这里的优先"
	}
	return "在 api.chksz.com 申请；留空时读取 CHKSZ_API_KEY 或 api.chksz.key 文件"
}
