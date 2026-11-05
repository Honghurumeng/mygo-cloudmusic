package app

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/egoist/mygo"

	"netease-music-mygo/internal/netease"
	"netease-music-mygo/internal/player"
)

// newSongCache returns the cache of songs played, in the app's cache
// directory, or nil without one.
func newSongCache(mb int) *player.Cache {
	dir, err := mygo.App.Path(mygo.PathCache)
	if err != nil {
		return nil
	}
	return player.NewCache(filepath.Join(dir, "songs"), int64(mb)<<20)
}

// applyCacheLimit gives the cache the limit the settings say.
func (a *app) applyCacheLimit() {
	if cache := a.pl.Cache; cache != nil {
		limit := int64(a.settings.CacheMB) << 20
		go func() {
			cache.SetLimit(limit)
			a.measureCache()
			// What it drops over the lower limit goes from the list too.
			a.post(func() {
				if a.cacheOpen {
					a.listCache()
				}
			})
		}()
	}
}

// measureCache finds how much the cache takes, on a goroutine, for the
// settings to show.
func (a *app) measureCache() {
	cache := a.pl.Cache
	if cache == nil {
		return
	}
	go func() {
		n := cache.Size()
		a.post(func() {
			a.cacheUsed = n
			if a.settingsWin != nil {
				a.settingsWin.Invalidate()
			}
		})
	}()
}

// listCache reads the songs the cache keeps, on a goroutine, for the
// settings to list: what a song is shows as soon as it is read, and the
// ones kept before the cache kept it fill in once NetEase says.
func (a *app) listCache() {
	cache := a.pl.Cache
	if cache == nil {
		return
	}
	a.cacheBusy = true
	go func() {
		songs := cache.Songs()
		a.post(func() {
			a.cacheSongs, a.cacheBusy = songs, false
		})
		if !fillCacheInfo(cache, songs) {
			return
		}
		songs = cache.Songs()
		a.post(func() { a.cacheSongs = songs })
	}()
}

// fillCacheInfo asks NetEase what the songs kept before the cache kept what
// a song is are, and writes it beside them, so it is asked once. It reports
// whether it wrote any.
func fillCacheInfo(cache *player.Cache, songs []player.CachedSong) bool {
	var ids []int64
	for _, s := range songs {
		if s.Name == "" {
			ids = append(ids, s.ID)
		}
	}
	if len(ids) == 0 {
		return false
	}
	tracks, err := netease.SongDetails(ids)
	if err != nil {
		return false
	}
	byID := make(map[int64]*netease.Track, len(tracks))
	for i := range tracks {
		byID[tracks[i].ID] = &tracks[i]
	}
	var wrote bool
	for _, s := range songs {
		if s.Name != "" {
			continue
		}
		if t := byID[s.ID]; t != nil && cache.SetInfo(s.ID, s.Level, *t) {
			wrote = true
		}
	}
	return wrote
}

// applyCacheActions drops what the list asked for as it last built, before
// it builds again: a row builds while it shows, so dropping its song then
// would leave the list reading a shorter slice than it builds by.
func (a *app) applyCacheActions() {
	if a.cacheClearing {
		a.cacheClearing = false
		a.clearCache()
		return
	}
	id := a.cacheDropping
	if id == 0 {
		return
	}
	a.cacheDropping = 0
	for i := range a.cacheSongs {
		if a.cacheSongs[i].ID == id {
			a.removeCached(i)
			return
		}
	}
}

// removeCached drops the song the cache lists at i.
func (a *app) removeCached(i int) {
	cache := a.pl.Cache
	if cache == nil || i < 0 || i >= len(a.cacheSongs) {
		return
	}
	s := a.cacheSongs[i]
	a.cacheSongs = append(a.cacheSongs[:i:i], a.cacheSongs[i+1:]...)
	if a.cacheUsed >= s.Size {
		a.cacheUsed -= s.Size
	}
	go func() {
		cache.Remove(s.ID, s.Level)
		a.measureCache()
	}()
}

func (a *app) clearCache() {
	if cache := a.pl.Cache; cache != nil {
		a.cacheUsed, a.cacheSongs = 0, nil
		go func() {
			cache.Clear()
			a.measureCache()
		}()
	}
}

// cacheUsage says how much the cache takes, once measured.
func (a *app) cacheUsage() string {
	switch n := a.cacheUsed; {
	case n < 0:
		return "正在计算已用空间…"
	case n >= 1<<30:
		return fmt.Sprintf("已用 %.1f GB", float64(n)/(1<<30))
	default:
		return fmt.Sprintf("已用 %d MB", n>>20)
	}
}

// cacheCount says how many songs the cache keeps.
func (a *app) cacheCount() string {
	switch {
	case a.cacheBusy && len(a.cacheSongs) == 0:
		return "正在读取…"
	case len(a.cacheSongs) == 0:
		return "还没有缓存的歌曲"
	}
	return fmt.Sprintf("%d 首 · %s", len(a.cacheSongs), a.cacheUsage())
}

// cachedLevel names the stream level a cached song was kept at.
func cachedLevel(level string) string {
	for _, q := range qualityLevels {
		if q.level == level {
			return q.label
		}
	}
	return level
}

// cachedSize formats how much a song takes.
func cachedSize(n int64) string {
	if n >= 1<<20 {
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	}
	return fmt.Sprintf("%d KB", (n+1023)>>10)
}

// cachedAt says when a song played last, as people read it.
func cachedAt(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "刚刚播放"
	case d < time.Hour:
		return fmt.Sprintf("%d 分钟前播放", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%d 小时前播放", int(d.Hours()))
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%d 天前播放", int(d.Hours()/24))
	}
	return t.Format("2006年1月2日")
}
