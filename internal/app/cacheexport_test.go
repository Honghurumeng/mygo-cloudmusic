package app

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"netease-music-mygo/internal/player"
)

// An exported file is named after its artists and its name, without what a
// file name may not hold, and says what a song with nothing known is.
func TestCachedFileName(t *testing.T) {
	s := player.CachedSong{ID: 7, Type: "mp3", Name: `夜/曲: 1`, Artists: `A\B`}
	if got := cachedFileName(s); got != "A＼B - 夜／曲： 1.mp3" {
		t.Errorf("name %q", got)
	}
	if got := cachedFileName(player.CachedSong{ID: 7, Type: "flac"}); got != "未知歌手 - 歌曲 7.flac" {
		t.Errorf("empty name %q", got)
	}
	for _, bad := range []string{"/", "\\", ":", "*", "?", `"`, "<", ">", "|"} {
		if strings.ContainsAny(cachedFileName(s), bad) {
			t.Errorf("name keeps %q", bad)
		}
	}
}

// Exporting an MP3 writes what the song is as an ID3v2.4 tag, replacing
// the tag it had, and copies the audio after it unchanged.
func TestExportMP3(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "1-exhigh.mp3")
	// An old tag of four bytes, then the audio-frame bytes.
	if err := os.WriteFile(src, []byte("ID3\x04\x00\x00\x00\x00\x00\x04OLD!audio-bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(dir, "out.mp3")
	s := player.CachedSong{ID: 1, Level: "exhigh", Type: "mp3", Name: "宝船", Artists: "歌手", Album: "专辑"}
	if err := exportSong(src, dst, s); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	tags, audio := parseID3(t, b)
	if got := tags["TIT2"]; got != "宝船" {
		t.Errorf("title %q", got)
	}
	if got := tags["TPE1"]; got != "歌手" {
		t.Errorf("artist %q", got)
	}
	if got := tags["TALB"]; got != "专辑" {
		t.Errorf("album %q", got)
	}
	if string(audio) != "audio-bytes" {
		t.Errorf("audio %q", audio)
	}
	if bytes.Contains(b, []byte("OLD!")) {
		t.Error("the old tag is still there")
	}
}

// Exporting a FLAC keeps its stream info, padding and audio as they were,
// and replaces its Vorbis comment with one that says what the song is.
func TestExportFLAC(t *testing.T) {
	dir := t.TempDir()
	streamInfo := bytes.Repeat([]byte{0x5a}, 34)
	comment := vorbisBody("Old", [][2]string{{"TITLE", "Old title"}, {"GENRE", "Pop"}})
	var src []byte
	src = append(src, "fLaC"...)
	src = append(src, flacBlockBytes(0, false, streamInfo)...)
	src = append(src, flacBlockBytes(4, false, comment)...)
	src = append(src, flacBlockBytes(1, true, []byte("pad!"))...)
	src = append(src, "audio-bytes"...)
	srcPath := filepath.Join(dir, "1-lossless.flac")
	if err := os.WriteFile(srcPath, src, 0o600); err != nil {
		t.Fatal(err)
	}

	dst := filepath.Join(dir, "out.flac")
	s := player.CachedSong{ID: 1, Level: "lossless", Type: "flac", Name: "宝船", Artists: "歌手", Album: "专辑"}
	if err := exportSong(srcPath, dst, s); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	blocks, audio := parseFLAC(t, b)
	if len(blocks) != 3 || blocks[0].typ != 0 || blocks[1].typ != 4 || blocks[2].typ != 1 {
		t.Fatalf("blocks %+v", blocks)
	}
	if blocks[0].last || !blocks[2].last {
		t.Errorf("last flags %+v", blocks)
	}
	if !bytes.Equal(blocks[0].body, streamInfo) {
		t.Error("the stream info changed")
	}
	if string(blocks[2].body) != "pad!" {
		t.Errorf("padding %q", blocks[2].body)
	}
	if string(audio) != "audio-bytes" {
		t.Errorf("audio %q", audio)
	}
	_, tags, ok := parseVorbis(blocks[1].body)
	if !ok {
		t.Fatal("the comment does not parse")
	}
	got := map[string]string{}
	for _, tag := range tags {
		got[tag[0]] = tag[1]
	}
	if got["TITLE"] != "宝船" || got["ARTIST"] != "歌手" || got["ALBUM"] != "专辑" {
		t.Errorf("tags %v", got)
	}
	if got["GENRE"] != "Pop" {
		t.Errorf("the old genre is gone: %v", got)
	}
}

type flacTestBlock struct {
	typ  byte
	last bool
	body []byte
}

func flacBlockBytes(typ byte, last bool, body []byte) []byte {
	h := []byte{typ, byte(len(body) >> 16), byte(len(body) >> 8), byte(len(body))}
	if last {
		h[0] |= 0x80
	}
	return append(h, body...)
}

func parseID3(t *testing.T, b []byte) (map[string]string, []byte) {
	t.Helper()
	if len(b) < 10 || string(b[:3]) != "ID3" || b[3] != 4 {
		t.Fatalf("not an ID3v2.4 tag: % x", b[:min(len(b), 10)])
	}
	n := int(b[6]&0x7f)<<21 | int(b[7]&0x7f)<<14 | int(b[8]&0x7f)<<7 | int(b[9]&0x7f)
	if 10+n > len(b) {
		t.Fatalf("tag of %d bytes in %d", n, len(b))
	}
	tags := map[string]string{}
	for frames := b[10 : 10+n]; len(frames) > 0; {
		if len(frames) < 10 {
			t.Fatalf("short frame header: % x", frames)
		}
		size := int(frames[4]&0x7f)<<21 | int(frames[5]&0x7f)<<14 | int(frames[6]&0x7f)<<7 | int(frames[7]&0x7f)
		if 10+size > len(frames) || size < 2 {
			t.Fatalf("frame of %d bytes in %d", size, len(frames))
		}
		if frames[10] != 3 {
			t.Fatalf("frame %q is not UTF-8", frames[:4])
		}
		tags[string(frames[:4])] = string(frames[11 : 10+size])
		frames = frames[10+size:]
	}
	return tags, b[10+n:]
}

func parseFLAC(t *testing.T, b []byte) ([]flacTestBlock, []byte) {
	t.Helper()
	if len(b) < 4 || string(b[:4]) != "fLaC" {
		t.Fatal("not a FLAC stream")
	}
	b = b[4:]
	var blocks []flacTestBlock
	for {
		if len(b) < 4 {
			t.Fatal("short metadata header")
		}
		last := b[0]&0x80 != 0
		n := int(b[1])<<16 | int(b[2])<<8 | int(b[3])
		if 4+n > len(b) {
			t.Fatalf("metadata block of %d bytes in %d", n, len(b))
		}
		blocks = append(blocks, flacTestBlock{typ: b[0] & 0x7f, last: last, body: b[4 : 4+n]})
		b = b[4+n:]
		if last {
			return blocks, b
		}
	}
}
