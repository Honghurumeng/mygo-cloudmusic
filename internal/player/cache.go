package player

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"netease-music-mygo/internal/netease"
)

// Cache keeps the songs played on disk, so that they play again at once,
// and without the network: each as <id>-<level>.mp3 or .flac, with what
// its stream was, and what the song is, beside it in <id>-<level>.json.
// Once they take more than the limit, those played longest ago go first.
type Cache struct {
	dir string

	mu    sync.Mutex
	limit int64 // in bytes; 0 keeps nothing
}

// NewCache returns a cache in dir of limit bytes.
func NewCache(dir string, limit int64) *Cache {
	return &Cache{dir: dir, limit: limit}
}

// SetLimit changes how much the cache may take, and drops what is over.
func (c *Cache) SetLimit(limit int64) {
	c.mu.Lock()
	c.limit = limit
	c.mu.Unlock()
	c.prune("")
}

func cacheKey(id int64, level string) string { return fmt.Sprintf("%d-%s", id, level) }

// keepable reports whether a song of level and type ext may be kept: they
// name its file, so only those known, which keep it in the cache's folder.
func keepable(level, ext string) bool {
	return slices.Contains(netease.Levels, level) && (ext == "mp3" || ext == "flac")
}

// songMeta is what the cache keeps beside a song's audio: its stream, which
// plays it again without the network, and what the song is, which lists it
// without asking NetEase again. Files kept before the cache kept what a
// song is read with those fields empty.
type songMeta struct {
	netease.Stream
	Name     string `json:"name,omitempty"`
	Artists  string `json:"artists,omitempty"`
	Album    string `json:"album,omitempty"`
	Cover    string `json:"cover,omitempty"`
	Duration int64  `json:"duration,omitempty"` // milliseconds
}

// CachedSong is a song the cache keeps, as listing it shows it.
type CachedSong struct {
	ID       int64
	Level    string // the stream level kept
	Type     string // mp3, or flac
	Size     int64  // in bytes
	Kept     time.Time
	Name     string
	Artists  string
	Album    string
	Cover    string
	Duration time.Duration
}

// lookup returns a song kept at level or a better one, open to read, with
// its stream.
func (c *Cache) lookup(id int64, level string) (*download, *netease.Stream, bool) {
	if c == nil {
		return nil, nil, false
	}
	at := max(slices.Index(netease.Levels, level), 0)
	for _, lv := range netease.Levels[at:] {
		key := cacheKey(id, lv)
		b, err := os.ReadFile(filepath.Join(c.dir, key+".json"))
		if err != nil {
			continue
		}
		var m songMeta
		if json.Unmarshal(b, &m) != nil || !keepable(lv, m.Type) {
			continue
		}
		path := filepath.Join(c.dir, key+"."+m.Type)
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		if info, err := f.Stat(); err != nil || info.Size() != m.Size {
			f.Close()
			continue
		}
		// Played now: the last to go.
		now := time.Now()
		os.Chtimes(path, now, now)
		return opened(f, m.Size), &m.Stream, true
	}
	return nil, nil, false
}

// download fetches the song t of the stream st into a file, to keep once
// it has all come in; or into memory, if the cache would not keep it.
func (c *Cache) download(t netease.Track, st *netease.Stream) *download {
	part := c.part(st)
	if part == nil {
		return startDownload(st.URL, st.Size, nil, nil)
	}
	return startDownload(st.URL, st.Size, part, func(path string, n int64) (string, bool) {
		return c.keep(t, st, path, n)
	})
}

// part makes a file for a song of the stream st to come into, or returns
// nil if the cache would not keep it. Each has a name of its own: the same
// song played again while it still comes in has another.
func (c *Cache) part(st *netease.Stream) *os.File {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	limit := c.limit
	c.mu.Unlock()
	// A song of more than a quarter of it would push out too many.
	if limit <= 0 || st.Size > limit/4 {
		return nil
	}
	if os.MkdirAll(c.dir, 0o700) != nil {
		return nil
	}
	f, err := os.CreateTemp(c.dir, "*.part")
	if err != nil {
		return nil
	}
	return f
}

// keep keeps the song t of the stream st, n bytes, from the file part,
// which is closed, and drops what is over the limit then. It returns where
// the song is: kept, or still at part.
func (c *Cache) keep(t netease.Track, st *netease.Stream, part string, n int64) (string, bool) {
	c.mu.Lock()
	limit := c.limit
	c.mu.Unlock()
	if n > limit/4 {
		return part, false
	}
	m := songMeta{
		Stream:   *st,
		Name:     t.Name,
		Artists:  t.ArtistNames(),
		Album:    t.Album.Name,
		Cover:    t.Album.PicURL,
		Duration: t.Duration,
	}
	if m.Type == "" {
		m.Type = "mp3"
	}
	m.Size = n
	if !keepable(m.Level, m.Type) {
		return part, false
	}
	key := cacheKey(t.ID, m.Level)
	b, _ := json.Marshal(m)
	path := filepath.Join(c.dir, key+"."+m.Type)
	if os.Rename(part, path) != nil {
		return part, false
	}
	os.WriteFile(filepath.Join(c.dir, key+".json"), b, 0o600)
	c.prune(path)
	return path, true
}

// Size returns how many bytes the songs kept take.
func (c *Cache) Size() int64 {
	var n int64
	for _, f := range c.songs() {
		n += f.size
	}
	return n
}

// Clear drops every song kept.
func (c *Cache) Clear() {
	for _, f := range c.songs() {
		c.remove(f.path)
	}
}

// Songs returns the songs the cache keeps, the one played last first, with
// what each is as far as it was kept with it.
func (c *Cache) Songs() []CachedSong {
	if c == nil {
		return nil
	}
	var out []CachedSong
	for _, f := range c.songs() {
		name := filepath.Base(f.path)
		ext := filepath.Ext(name)
		base := strings.TrimSuffix(name, ext)
		i := strings.Index(base, "-")
		if i < 0 {
			continue
		}
		id, err := strconv.ParseInt(base[:i], 10, 64)
		if err != nil {
			continue
		}
		s := CachedSong{
			ID:    id,
			Level: base[i+1:],
			Type:  strings.TrimPrefix(ext, "."),
			Size:  f.size,
			Kept:  f.used,
		}
		if b, err := os.ReadFile(strings.TrimSuffix(f.path, ext) + ".json"); err == nil {
			var m songMeta
			if json.Unmarshal(b, &m) == nil {
				s.Name, s.Artists, s.Album, s.Cover = m.Name, m.Artists, m.Album, m.Cover
				s.Duration = time.Duration(m.Duration) * time.Millisecond
				if m.Level != "" {
					s.Level = m.Level
				}
			}
		}
		out = append(out, s)
	}
	slices.SortFunc(out, func(a, b CachedSong) int { return b.Kept.Compare(a.Kept) })
	return out
}

// Remove drops the song of id kept at level, and what is kept of it.
func (c *Cache) Remove(id int64, level string) {
	if c == nil {
		return
	}
	key := cacheKey(id, level)
	for _, ext := range []string{"mp3", "flac"} {
		os.Remove(filepath.Join(c.dir, key+"."+ext))
	}
	os.Remove(filepath.Join(c.dir, key+".json"))
}

// SetInfo writes what a song is to what the cache keeps of id at level, for
// songs kept before the cache kept what a song is. It reports whether the
// cache keeps the song then.
func (c *Cache) SetInfo(id int64, level string, t netease.Track) bool {
	if c == nil {
		return false
	}
	path := filepath.Join(c.dir, cacheKey(id, level)+".json")
	b, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var m songMeta
	if json.Unmarshal(b, &m) != nil {
		return false
	}
	m.Name, m.Artists, m.Album = t.Name, t.ArtistNames(), t.Album.Name
	m.Cover, m.Duration = t.Album.PicURL, t.Duration
	b, err = json.Marshal(m)
	if err != nil {
		return false
	}
	return os.WriteFile(path, b, 0o600) == nil
}

type cachedSong struct {
	path string
	size int64
	used time.Time
}

func (c *Cache) songs() []cachedSong {
	entries, _ := os.ReadDir(c.dir)
	var out []cachedSong
	for _, e := range entries {
		ext := filepath.Ext(e.Name())
		if ext != ".mp3" && ext != ".flac" {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		out = append(out, cachedSong{filepath.Join(c.dir, e.Name()), info.Size(), info.ModTime()})
	}
	return out
}

func (c *Cache) remove(path string) {
	os.Remove(path)
	os.Remove(strings.TrimSuffix(path, filepath.Ext(path)) + ".json")
}

// prune drops the songs played longest ago until the rest fit the limit,
// but keep, the song just kept.
func (c *Cache) prune(keep string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.dropParts()
	songs := c.songs()
	slices.SortFunc(songs, func(a, b cachedSong) int { return a.used.Compare(b.used) })
	var total int64
	for _, f := range songs {
		total += f.size
	}
	for _, f := range songs {
		if total <= c.limit {
			return
		}
		if f.path != keep {
			c.remove(f.path)
			total -= f.size
		}
	}
}

// dropParts removes the files of songs that never all came in, left by an
// app that quit as they came: one coming in is written to every moment.
func (c *Cache) dropParts() {
	parts, _ := filepath.Glob(filepath.Join(c.dir, "*.part"))
	for _, path := range parts {
		if info, err := os.Stat(path); err == nil && time.Since(info.ModTime()) > time.Hour {
			os.Remove(path)
		}
	}
}
