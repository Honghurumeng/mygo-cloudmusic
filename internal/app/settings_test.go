package app

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// The settings window shows one category at a time, picked in the rail on
// its left: not one scroll through all of them.
func TestSettingsRailPicksThePage(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	tt := ui.NewTester(a.settingsView, 760, 580)
	tt.Frame()

	sections := a.settingsSections()
	if a.settingsTab != sections[0].label {
		t.Fatalf("shows %q, want %q first", a.settingsTab, sections[0].label)
	}
	for _, sec := range sections {
		if !tt.HasText(sec.label) {
			t.Errorf("the rail does not list %q", sec.label)
		}
	}

	if err := tt.Click("音源"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.settingsTab != "音源" {
		t.Fatalf("picked %q, want 音源", a.settingsTab)
	}
	if !tt.HasText("ChKSz 音源") {
		t.Error("the page of 音源 does not show")
	}
	if tt.HasText("音量均衡") {
		t.Error("the page of 播放 shows beside 音源")
	}

	if err := tt.Click("播放"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("音量均衡") {
		t.Error("the page of 播放 does not show")
	}
	if tt.HasText("ChKSz 音源") {
		t.Error("the page of 音源 shows beside 播放")
	}
}
