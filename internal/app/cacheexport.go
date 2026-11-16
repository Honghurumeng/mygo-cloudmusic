package app

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"unicode/utf8"

	"github.com/egoist/mygo"

	"netease-music-mygo/internal/player"
)

// saveSongAs asks where to export a song, through a sheet on the settings
// window. A test swaps it out.
var saveSongAs = func(parent *mygo.Window, name, ext string) (string, error) {
	dir, _ := mygo.App.Path(mygo.PathMusic)
	return mygo.Dialog.Save(mygo.SaveDialogOptions{
		Parent:            parent,
		Title:             "导出歌曲",
		DefaultPath:       filepath.Join(dir, name),
		ButtonLabel:       "导出",
		Filters:           []mygo.FileFilter{{Name: strings.ToUpper(ext) + " 音频", Extensions: []string{ext}}},
		CreateDirectories: true,
	})
}

// exportCached asks where to save s, then writes it there with its name
// and tags, on a goroutine, and says where it went.
func (a *app) exportCached(s player.CachedSong) {
	cache := a.pl.Cache
	src := cache.SongFile(s)
	if src == "" {
		a.cacheToast = "这首歌已经不在缓存里了"
		return
	}
	win := a.settingsWin
	name := cachedFileName(s)
	go func() {
		dst, err := saveSongAs(win, name, s.Type)
		if err == nil && dst != "" {
			if filepath.Ext(dst) == "" {
				dst += "." + s.Type
			}
			err = exportSong(src, dst, s)
		}
		if err != nil {
			a.post(func() { a.cacheToast = "导出失败，请稍后再试" })
			return
		}
		if dst != "" {
			a.post(func() { a.cacheToast = "已导出 " + filepath.Base(dst) })
		}
	}()
}

// cachedFileName is what an exported song is called: its artists and its
// name, without what a file name may not hold.
func cachedFileName(s player.CachedSong) string {
	base := sanitizeFileName(cachedArtist(s) + " - " + cachedTitle(s))
	if base == "" {
		base = fmt.Sprintf("歌曲 %d", s.ID)
	}
	return truncateName(base, 180) + "." + s.Type
}

func cachedTitle(s player.CachedSong) string {
	if name := strings.TrimSpace(s.Name); name != "" {
		return name
	}
	return fmt.Sprintf("歌曲 %d", s.ID)
}

func cachedArtist(s player.CachedSong) string {
	if artists := strings.TrimSpace(s.Artists); artists != "" {
		return artists
	}
	return "未知歌手"
}

// fileNameReplacer makes characters a file name may not hold on some
// system full-width, which reads better than dropping them.
var fileNameReplacer = strings.NewReplacer(
	"/", "／", "\\", "＼", ":", "：", "*", "＊", "?", "？",
	"\"", "＂", "<", "＜", ">", "＞", "|", "｜", "\x00", "",
)

func sanitizeFileName(s string) string {
	s = fileNameReplacer.Replace(s)
	s = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, s)
	return strings.Trim(s, " .")
}

// truncateName cuts s to at most n bytes, without cutting a character,
// and without leaving a dot or a space where the name ends.
func truncateName(s string, n int) string {
	if len(s) <= n {
		return s
	}
	s = s[:n]
	for !utf8.ValidString(s) {
		s = s[:len(s)-1]
	}
	return strings.TrimRight(s, " .")
}

// exportSong writes the cached audio at src to dst, tagged with what the
// song is. MP3 and FLAC streams are copied as they are; only their tags
// are written again.
func exportSong(src, dst string, s player.CachedSong) error {
	if src == "" {
		return errors.New("the song is gone")
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	tmp, err := os.CreateTemp(filepath.Dir(dst), ".mygo-export-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	switch s.Type {
	case "mp3":
		err = copyMP3(tmp, in, s)
	case "flac":
		err = copyFLAC(tmp, in, s)
	default:
		err = fmt.Errorf("unsupported type %q", s.Type)
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), dst); err != nil {
		// Windows will not replace a file that is there: the new one has
		// all come in, so the old one goes.
		if runtime.GOOS != "windows" {
			return err
		}
		if err := os.Remove(dst); err != nil && !os.IsNotExist(err) {
			return err
		}
		return os.Rename(tmp.Name(), dst)
	}
	return nil
}

// songTags are what an exported song says it is.
func songTags(s player.CachedSong) [][2]string {
	tags := [][2]string{{"TITLE", cachedTitle(s)}, {"ARTIST", cachedArtist(s)}}
	if album := strings.TrimSpace(s.Album); album != "" {
		tags = append(tags, [2]string{"ALBUM", album})
	}
	return tags
}

// copyMP3 writes an ID3v2.4 tag of what the song is, then the audio of
// src from after its own tag.
func copyMP3(dst io.Writer, src *os.File, s player.CachedSong) error {
	off, err := skipID3(src)
	if err != nil {
		return err
	}
	if _, err := dst.Write(id3Tag(s)); err != nil {
		return err
	}
	if _, err := src.Seek(off, io.SeekStart); err != nil {
		return err
	}
	_, err = io.Copy(dst, src)
	return err
}

// skipID3 returns where the audio begins, after an ID3v2 tag at the start
// of f if it has one.
func skipID3(f *os.File) (int64, error) {
	var h [10]byte
	_, err := io.ReadFull(f, h[:])
	if err == io.EOF || err == io.ErrUnexpectedEOF {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	if string(h[:3]) != "ID3" {
		return 0, nil
	}
	// The size excludes the header, and includes a footer if there is one.
	return 10 + syncsafe(h[6:10]), nil
}

func id3Tag(s player.CachedSong) []byte {
	var frames []byte
	add := func(id, value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		// 3 is UTF-8, which ID3v2.4 takes.
		text := append([]byte{3}, value...)
		var h [10]byte
		copy(h[:4], id)
		putSyncsafe(h[4:8], len(text))
		frames = append(frames, h[:]...)
		frames = append(frames, text...)
	}
	add("TIT2", cachedTitle(s))
	add("TPE1", cachedArtist(s))
	add("TALB", s.Album)

	tag := make([]byte, 10, 10+len(frames))
	copy(tag, "ID3")
	tag[3], tag[4] = 4, 0 // version 2.4, no flags
	putSyncsafe(tag[6:10], len(frames))
	return append(tag, frames...)
}

func syncsafe(b []byte) int64 {
	return int64(b[0]&0x7f)<<21 | int64(b[1]&0x7f)<<14 | int64(b[2]&0x7f)<<7 | int64(b[3]&0x7f)
}

func putSyncsafe(b []byte, n int) {
	b[0], b[1], b[2], b[3] = byte(n>>21)&0x7f, byte(n>>14)&0x7f, byte(n>>7)&0x7f, byte(n)&0x7f
}

// flacBlock is a metadata block of the FLAC stream: its type and length,
// where its body is in the file, and its body, if it is to be written
// rather than copied.
type flacBlock struct {
	typ  byte
	size int64
	at   int64
	body []byte
}

// copyFLAC writes the STREAMINFO block of src, a Vorbis comment of what
// the song is, the other metadata blocks, and the audio.
func copyFLAC(dst io.Writer, src *os.File, s player.CachedSong) error {
	blocks, audio, vendor, old, err := readFLACMeta(src)
	if err != nil {
		return err
	}
	if len(blocks) == 0 || blocks[0].typ != 0 || blocks[0].size != 34 {
		return errors.New("not a FLAC stream")
	}
	body := vorbisBody(vendor, mergeTags(old, songTags(s)))
	if len(body) > 0xffffff {
		return errors.New("the tags are too long")
	}
	out := []flacBlock{
		{typ: 0, size: blocks[0].size, at: blocks[0].at},
		{typ: 4, size: int64(len(body)), body: body},
	}
	// What says the song is says it once: the old comment goes, and the
	// stream info stays first.
	for _, b := range blocks[1:] {
		if b.typ != 4 {
			out = append(out, b)
		}
	}
	if _, err := dst.Write([]byte("fLaC")); err != nil {
		return err
	}
	for i, b := range out {
		h := [4]byte{b.typ, byte(b.size >> 16), byte(b.size >> 8), byte(b.size)}
		if i == len(out)-1 {
			h[0] |= 0x80
		}
		if _, err := dst.Write(h[:]); err != nil {
			return err
		}
		if b.body != nil {
			if _, err := dst.Write(b.body); err != nil {
				return err
			}
			continue
		}
		if _, err := src.Seek(b.at, io.SeekStart); err != nil {
			return err
		}
		if _, err := io.CopyN(dst, src, b.size); err != nil {
			return err
		}
	}
	if _, err := src.Seek(audio, io.SeekStart); err != nil {
		return err
	}
	_, err = io.Copy(dst, src)
	return err
}

// readFLACMeta reads the metadata of a FLAC stream, and returns its blocks,
// where the audio begins, and what its Vorbis comment says.
func readFLACMeta(f *os.File) (blocks []flacBlock, audio int64, vendor string, tags [][2]string, err error) {
	if _, err = f.Seek(0, io.SeekStart); err != nil {
		return
	}
	var magic [4]byte
	if _, err = io.ReadFull(f, magic[:]); err != nil {
		return
	}
	if string(magic[:]) != "fLaC" {
		err = errors.New("not a FLAC stream")
		return
	}
	for i := 0; ; i++ {
		if i > 1024 {
			err = errors.New("too many metadata blocks")
			return
		}
		var h [4]byte
		if _, err = io.ReadFull(f, h[:]); err != nil {
			return
		}
		last := h[0]&0x80 != 0
		size := int64(h[1])<<16 | int64(h[2])<<8 | int64(h[3])
		if size > 1<<26 {
			err = errors.New("a metadata block is too big")
			return
		}
		at, _ := f.Seek(0, io.SeekCurrent)
		blocks = append(blocks, flacBlock{typ: h[0] & 0x7f, size: size, at: at})
		if typ := h[0] & 0x7f; typ == 4 && size <= 1<<20 {
			b := make([]byte, size)
			if _, err = io.ReadFull(f, b); err != nil {
				return
			}
			if v, t, ok := parseVorbis(b); ok {
				if vendor == "" {
					vendor = v
				}
				tags = append(tags, t...)
			}
		} else if _, err = f.Seek(size, io.SeekCurrent); err != nil {
			return
		}
		if last {
			break
		}
	}
	audio, err = f.Seek(0, io.SeekCurrent)
	return
}

// parseVorbis reads a Vorbis comment's vendor and tags, as FLAC keeps them.
func parseVorbis(b []byte) (vendor string, tags [][2]string, ok bool) {
	read := func() (string, bool) {
		if len(b) < 4 {
			return "", false
		}
		n := int(binary.LittleEndian.Uint32(b))
		b = b[4:]
		if n < 0 || n > len(b) {
			return "", false
		}
		s := string(b[:n])
		b = b[n:]
		return s, true
	}
	v, ok := read()
	if !ok || len(b) < 4 {
		return "", nil, false
	}
	count := binary.LittleEndian.Uint32(b)
	b = b[4:]
	if count > 50000 {
		return "", nil, false
	}
	for i := uint32(0); i < count; i++ {
		s, ok := read()
		if !ok {
			return "", nil, false
		}
		if j := strings.IndexByte(s, '='); j >= 0 {
			tags = append(tags, [2]string{s[:j], s[j+1:]})
		}
	}
	return v, tags, true
}

// vorbisBody writes a vendor and tags as a Vorbis comment's body.
func vorbisBody(vendor string, tags [][2]string) []byte {
	if vendor == "" {
		vendor = "MyGo 云音乐"
	}
	var b bytes.Buffer
	put := func(s string) {
		var n [4]byte
		binary.LittleEndian.PutUint32(n[:], uint32(len(s)))
		b.Write(n[:])
		b.WriteString(s)
	}
	put(vendor)
	var n [4]byte
	binary.LittleEndian.PutUint32(n[:], uint32(len(tags)))
	b.Write(n[:])
	for _, t := range tags {
		put(t[0] + "=" + t[1])
	}
	return b.Bytes()
}

// mergeTags keeps what old says that new doesn't, then adds new.
func mergeTags(old, new [][2]string) [][2]string {
	out := make([][2]string, 0, len(old)+len(new))
	for _, t := range old {
		replaced := false
		for _, n := range new {
			if strings.EqualFold(t[0], n[0]) {
				replaced = true
				break
			}
		}
		if !replaced {
			out = append(out, t)
		}
	}
	return append(out, new...)
}
