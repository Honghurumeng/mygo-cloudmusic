package player

import (
	"testing"

	"netease-music-mygo/internal/netease"
)

// One button, three modes: the list looping, this song looping, the list
// shuffled, and round again.
func TestModesCycle(t *testing.T) {
	p := &Player{at: -1}
	p.Cue([]netease.Track{{ID: 1, Name: "一"}, {ID: 2, Name: "二"}, {ID: 3, Name: "三"}})
	// A player starts with the list looping, though the field behind it
	// still holds the zero value.
	if got := p.Mode(); got != ModeRepeatAll {
		t.Fatalf("a new player is in mode %v, want the list looping", got)
	}
	for _, want := range []PlayMode{ModeRepeatOne, ModeShuffle, ModeRepeatAll, ModeRepeatOne} {
		p.CycleMode()
		if got := p.Mode(); got != want {
			t.Fatalf("cycled to %v, want %v", got, want)
		}
	}
}

// A list cannot both shuffle and repeat one song, so turning one on lets
// go of the other: there is no fourth mode behind the button.
func TestOneModeAtATime(t *testing.T) {
	p := &Player{at: -1}
	p.Cue([]netease.Track{{ID: 1, Name: "一"}, {ID: 2, Name: "二"}})
	p.SetMode(ModeRepeatOne)
	p.SetShuffle(true)
	if p.Mode() != ModeShuffle || p.repeat != RepeatAll {
		t.Fatalf("shuffle with repeat %v, mode %v", p.repeat, p.Mode())
	}
	p.SetMode(ModeRepeatOne)
	if p.shuffle || p.repeat != RepeatOne {
		t.Fatalf("single repeat with shuffle %v, repeat %v", p.shuffle, p.repeat)
	}
}

// Choosing the mode it is already in changes nothing, so that picking
// "随机播放" again from the menu does not reshuffle what is queued.
func TestSettingTheSameModeChangesNothing(t *testing.T) {
	p := &Player{at: -1}
	p.Cue([]netease.Track{{ID: 1}, {ID: 2}, {ID: 3}, {ID: 4}, {ID: 5}, {ID: 6}})
	p.SetMode(ModeShuffle)
	order := append([]int(nil), p.order...)
	at := p.at
	p.SetMode(ModeShuffle)
	if p.at != at {
		t.Fatalf("at moved from %d to %d", at, p.at)
	}
	for i := range order {
		if p.order[i] != order[i] {
			t.Fatalf("the order was shuffled again: %v, was %v", p.order, order)
		}
	}
}

// Changing the mode keeps the song playing the one playing, wherever it
// moves in the order.
func TestModesKeepTheSong(t *testing.T) {
	p := &Player{at: -1}
	p.Cue([]netease.Track{{ID: 1, Name: "一"}, {ID: 2, Name: "二"}, {ID: 3, Name: "三"}})
	// The third plays, as if the listener had picked it.
	p.at = 2
	p.track = &p.queue[2]

	p.SetMode(ModeShuffle)
	if p.order[p.at] != 2 {
		t.Fatalf("shuffling left %d playing, want 2", p.order[p.at])
	}
	p.SetMode(ModeRepeatOne)
	if p.order[p.at] != 2 || p.at != 2 {
		t.Fatalf("unshuffling left %d at %d playing, want 2 at 2", p.order[p.at], p.at)
	}
}
