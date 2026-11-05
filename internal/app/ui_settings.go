package app

import (
	"fmt"
	"runtime"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// openSettings shows the settings window, opening it the first time.
func (a *app) openSettings() {
	if a.settingsWin != nil {
		a.settingsWin.Show()
		a.settingsWin.Focus()
		a.measureCache()
		return
	}
	a.measureCache()
	a.settingsWin = mygo.NewWindow(mygo.WindowOptions{
		Title:     "设置",
		Width:     760,
		Height:    580,
		MinWidth:  680,
		MinHeight: 460,
		// A key of its own: the size saved for the old one-scroll window
		// would leave the rail too little room.
		StateKey:        "settings-rail",
		DisableMaximize: true,
		DisableMinimize: true,
		// The app's menu has nothing for this window: on Linux and
		// Windows it stays out of sight until Alt shows it.
		AutoHideMenuBar: true,
		BackgroundColor: "light-dark(#f4f4f6, #1c1c1f)",
		Content:         ui.View(a.settingsView),
	})
	a.settingsWin.OnClosed(func() {
		a.settingsWin = nil
		a.syncDock()
	})
	a.syncDock()
}

// changed applies the settings everywhere and keeps them.
func (a *app) changed() {
	a.settings.save()
	a.settings.applyAppearance()
	a.chksz = newChKSz(a.settings)
	if a.notch != nil {
		a.notch.setEnabled(a.settings.Notch)
	}
	a.redraw()
}

// settingsSection is a category of the settings window, as the rail down
// its left lists it.
type settingsSection struct {
	label string
	icon  *ui.SVG
}

// settingsSections are the categories the window shows, top to bottom:
// those this build has something to put in.
func (a *app) settingsSections() []settingsSection {
	var out []settingsSection
	// The icon in the menu bar, and the player at the notch, are macOS's.
	if runtime.GOOS == "darwin" || a.notch != nil {
		out = append(out, settingsSection{"通用", iconSettings})
	}
	out = append(out, settingsSection{"播放", iconPlay})
	if a.captionsShown() {
		out = append(out, settingsSection{"播客", iconPodcast})
	}
	return append(out,
		settingsSection{"音源", iconRadio},
		settingsSection{"外观", iconPalette},
		settingsSection{"缓存", iconDownload},
		settingsSection{"账号", iconUser},
	)
}

// settingsGroup and settingsRow lay out a page of the settings: a card of
// rows, each a label and what it says beside its control.
type settingsGroup func(title string, rows func())
type settingsRow func(label, desc string, control func())

// settingsView shows the categories down the left, and the page of the one
// picked on the right: a window of short pages, not one scroll through all
// of them.
func (a *app) settingsView(c *ui.Context) {
	t := *c.Theme()
	p := paletteFor(&t)
	t.Accent, t.AccentText = p.accent, p.onAccent
	c.SetTheme(&t)
	c.Root().Background(p.sidebar)
	c.OnShortcut(ui.Cmd, ui.KeyW, func() { a.settingsWin.Close() })
	if a.cacheOpen {
		a.cacheSongsView(c, p)
		return
	}
	s := &a.settings
	sections := a.settingsSections()
	at := sections[0]
	for _, sec := range sections {
		if sec.label == a.settingsTab {
			at = sec
		}
	}
	a.settingsTab = at.label

	ui.Row(c).Fill().AlignItems(ui.Stretch).Children(func() {
		a.settingsRail(c, p, sections, at)
		// Keyed by the category: another page starts at its top.
		ui.Scroll(c.Key("settings-"+at.label)).Grow(1).Basis(0).MinWidth(0).
			Padding(20, 26, 30, 26).Gap(10).Children(func() {
			ui.Text(c, at.label).FontSize(15).FontWeight(600).Margin(2, 0, 0, 0)
			group := func(title string, rows func()) {
				if title != "" {
					ui.Text(c, title).FontSize(12).FontWeight(600).TextColor(p.muted).Margin(10, 4, 4, 4)
				}
				ui.Column(c).Radius(12).Background(p.bg).Border(1, p.hairline).Dividers(1, p.hairline).Children(rows)
			}
			row := func(label, desc string, control func()) {
				ui.Row(c).Padding(12, 16).Gap(16).AlignItems(ui.Center).MinHeight(48).Children(func() {
					ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(3).Children(func() {
						ui.Text(c, label).FontSize(13.5).FontWeight(500)
						if desc != "" {
							ui.Text(c, desc).FontSize(12).TextColor(p.muted).LineHeight(1.4)
						}
					})
					control()
				})
			}

			switch at.label {
			case "通用":
				a.generalSettings(c, s, group, row)
			case "播放":
				a.playbackSettings(c, s, group, row)
			case "播客":
				group("", func() { a.captionSettings(row, c) })
			case "音源":
				a.sourceSettings(c, s, group, row)
			case "外观":
				a.appearanceSettings(c, s, group, row)
			case "缓存":
				a.cacheSettings(c, s, group, row)
			case "账号":
				a.accountSettings(c, p, s, group, row)
			}
		})
	})
}

// settingsRail lists the categories down the window's left, picking out
// the one whose page shows.
func (a *app) settingsRail(c *ui.Context, p palette, sections []settingsSection, at settingsSection) {
	ui.Column(c).Width(178).Padding(18, 10, 14, 10).Gap(2).
		BorderWidth(0, 1, 0, 0).BorderColor(p.hairline).Children(func() {
		for _, sec := range sections {
			b := ui.ButtonBase(c).Height(34).PaddingX(10).Gap(9).Radius(8).
				AlignItems(ui.Center).Justify(ui.Start).FocusRing(false).Label(sec.label)
			color := p.text
			bg := ui.Transparent
			switch {
			case sec.label == at.label:
				bg, color = p.press, p.accent
			case b.Hovered():
				bg = p.hover
			}
			b.Background(b.AnimateColor("bg", bg, ui.Animation{Duration: 110 * time.Millisecond}))
			b.Children(func() {
				ui.Icon(c, sec.icon).Size(16, 16).TextColor(color)
				ui.Text(c, sec.label).FontSize(13).FontWeight(500).TextColor(color)
			})
			if b.Clicked() {
				a.settingsTab = sec.label
				if a.settingsWin != nil {
					a.settingsWin.Invalidate()
				}
			}
		}
	})
}

// generalSettings is the icon the app shows, and the player at the notch:
// what macOS gives.
func (a *app) generalSettings(c *ui.Context, s *settings, group settingsGroup, row settingsRow) {
	if runtime.GOOS == "darwin" {
		group("", func() {
			row("应用图标", "显示在菜单栏时，关闭窗口后可从菜单栏图标打开", func() {
				labels := make([]string, len(appIcons))
				chosen := appIcons[0].label
				for i, ic := range appIcons {
					labels[i] = ic.label
					if ic.id == s.AppIcon {
						chosen = ic.label
					}
				}
				if ui.Select(c.Key("app-icon"), &chosen, labels).Label("应用图标").Width(150).Changed() {
					for _, ic := range appIcons {
						if ic.label == chosen {
							s.AppIcon = ic.id
						}
					}
					a.changed()
					a.changeAppIcon()
				}
			})
		})
	}
	if a.notch != nil {
		group("刘海播放器", func() {
			row("显示刘海播放器", "在屏幕顶部的刘海处显示迷你播放器，鼠标悬停即可展开", func() {
				if ui.Switch(c.Key("notch"), &s.Notch).Label("显示刘海播放器").Changed() {
					a.changed()
				}
			})
			row("播放时显示实时活动", "在刘海两侧显示封面与音频波形，可能遮挡刘海旁的菜单栏图标", func() {
				if ui.Switch(c.Key("activity"), &s.NotchActivity).Label("播放时显示实时活动").Disabled(!s.Notch).Changed() {
					a.changed()
				}
			})
			row("在无刘海的屏幕上常驻显示", "关闭时，将鼠标移至屏幕顶部中央即可唤出，不会遮挡菜单栏", func() {
				if ui.Switch(c.Key("virtual"), &s.NotchVirtual).Label("在无刘海的屏幕上常驻显示").Disabled(!s.Notch).Changed() {
					a.changed()
				}
			})
		})
	}
}

// playbackSettings is how songs play: their quality, their loudness, and
// what the lyrics show between them.
func (a *app) playbackSettings(c *ui.Context, s *settings, group settingsGroup, row settingsRow) {
	group("", func() {
		row("音质", "无损与 Hi-Res 需要黑胶 VIP，没有时播放所能播放的最高音质；下一首歌曲开始生效", func() {
			labels := make([]string, len(qualityLevels))
			chosen := qualityLevels[2].label
			for i, q := range qualityLevels {
				labels[i] = q.label
				if q.level == s.Quality {
					chosen = q.label
				}
			}
			if ui.Select(c.Key("quality"), &chosen, labels).Label("音质").Width(150).Changed() {
				for _, q := range qualityLevels {
					if q.label == chosen {
						s.Quality = q.level
					}
				}
				a.changed()
			}
		})
		row("间奏时显示热评", "前奏、间奏与尾奏时，歌词处换成这首歌的热门评论", func() {
			if ui.Switch(c.Key("hot-comments"), &s.HotComments).Label("间奏时显示热评").Changed() {
				a.changed()
			}
		})
		row("音量均衡", "按网易云音乐测得的响度调整每首歌的音量，让歌曲听起来一样响", func() {
			if ui.Switch(c.Key("normalize"), &s.Normalize).Label("音量均衡").Changed() {
				a.pl.SetVolume(a.volume)
				a.changed()
			}
		})
	})
}

// sourceSettings is where songs play from when NetEase won't play them.
func (a *app) sourceSettings(c *ui.Context, s *settings, group settingsGroup, row settingsRow) {
	group("", func() {
		row("ChKSz 音源", "网易云播放不了时，再由 ChKSz 解析播放地址：它按次数限额，只有真的播不了才会用到", func() {
			on := !s.NetEaseOnly
			if ui.Switch(c.Key("chksz"), &on).Label("使用 ChKSz 音源").Changed() {
				s.NetEaseOnly = !on
				a.changed()
			}
		})
		row("API Key", a.chkszKeyDesc(s), func() {
			in := ui.TextInput(c.Key("chksz-key"), &s.ChKSzKey).Label("ChKSz API Key").
				Placeholder("chksz_…").Width(210).Disabled(s.NetEaseOnly)
			if in.Changed() {
				a.chksz = newChKSz(a.settings)
				a.settings.save()
			}
		})
	})
}

// appearanceSettings is how the app looks, and the effects it may draw.
func (a *app) appearanceSettings(c *ui.Context, s *settings, group settingsGroup, row settingsRow) {
	group("", func() {
		row("主题", "跟随系统时随系统切换，也可以自己定浅色或深色", func() {
			modes := []string{"system", "light", "dark"}
			i := 0
			for k, m := range modes {
				if m == s.Appearance {
					i = k
				}
			}
			if segmented(c.Key("appearance"), &i, "跟随系统", "浅色", "深色") {
				s.Appearance = modes[i]
				a.changed()
			}
		})
	})
	group("实验性功能", func() {
		row("进度条烟雾特效", "播放时进度条化作拖着烟雾的光点，随音乐起伏；会多用一些电量", func() {
			if ui.Switch(c.Key("smoke"), &s.Smoke).Label("进度条烟雾特效").Changed() {
				a.changed()
			}
		})
		row("烟雾背景", "播放页的背景化作封面色调的烟雾，缓缓升起，随低音起伏；仍在打磨，会多用一些电量", func() {
			if ui.Switch(c.Key("fog"), &s.Fog).Label("烟雾背景").Changed() {
				a.changed()
			}
		})
	})
}

// cacheSettings is what the app keeps of the songs played, and how much.
func (a *app) cacheSettings(c *ui.Context, s *settings, group settingsGroup, row settingsRow) {
	group("", func() {
		row("歌曲缓存", "听过的歌曲保存在本机，再次播放无需等待，也不耗流量；"+a.cacheUsage(), func() {
			labels := make([]string, len(cacheLimits))
			chosen := cacheLimits[2].label
			for i, l := range cacheLimits {
				labels[i] = l.label
				if l.mb == s.CacheMB {
					chosen = l.label
				}
			}
			if ui.Select(c.Key("cache"), &chosen, labels).Label("歌曲缓存").Width(150).Changed() {
				for _, l := range cacheLimits {
					if l.label == chosen {
						s.CacheMB = l.mb
					}
				}
				a.changed()
				a.applyCacheLimit()
			}
		})
		row("缓存明细", a.cacheCount()+"，可逐首查看与删除", func() {
			if ui.Button(c.Key("cache-songs"), "管理").Clicked() {
				a.openCacheSongs()
			}
		})
	})
}

// accountSettings is who listens: signed in to NetEase, or not.
func (a *app) accountSettings(c *ui.Context, p palette, s *settings, group settingsGroup, row settingsRow) {
	group("", func() {
		if a.profile == nil {
			row("未登录", "登录后，每日推荐会按你的口味生成，歌单与喜欢同步到云端；本机也能建歌单", func() {
				if ui.PrimaryButton(c.Key("login"), "扫码登录").Clicked() {
					a.showMain()
					a.openLogin()
					a.win.Invalidate()
				}
			})
			return
		}
		prof := a.profile
		ui.Row(c).Padding(12, 16).Gap(12).AlignItems(ui.Center).Children(func() {
			a.avatar(c, prof.AvatarURL, 36, p.press)
			ui.Column(c).Grow(1).Gap(2).Children(func() {
				ui.Text(c, prof.Nickname).FontSize(13.5).FontWeight(600)
				ui.Text(c, "已登录网易云音乐").FontSize(12).TextColor(p.muted)
			})
			if ui.Button(c.Key("profile"), "我的主页").Clicked() {
				a.showMain()
				a.open(route{page: pageUser, id: prof.UserID})
				a.settingsWin.Close()
			}
			if ui.Button(c.Key("logout"), "退出登录").Clicked() {
				a.signOut()
			}
		})
	})
}

// openCacheSongs shows the songs the cache keeps, in the settings window.
func (a *app) openCacheSongs() {
	// Back from the list returns to the page it was opened from.
	a.settingsTab = "缓存"
	a.cacheOpen, a.cacheList = true, ui.ListState{}
	a.listCache()
	a.redraw()
}

// cacheSongsView lists the songs the cache keeps, with how much each takes
// and when it played last: the back button returns to the settings.
func (a *app) cacheSongsView(c *ui.Context, p palette) {
	a.applyCacheActions()
	ui.Column(c).Fill().Children(func() {
		ui.Row(c).Height(56).PaddingX(16).Gap(12).AlignItems(ui.Center).
			BorderWidth(0, 0, 1, 0).BorderColor(p.hairline).Children(func() {
			if iconButton(c.Key("back"), iconBack, 17, p.muted, "返回设置").Clicked() {
				a.cacheOpen = false
			}
			ui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(1).Children(func() {
				ui.Text(c, "歌曲缓存").FontSize(15).FontWeight(600)
				ui.Text(c, a.cacheCount()).FontSize(12).TextColor(p.muted).SingleLine()
			})
			if len(a.cacheSongs) > 0 {
				if ui.Button(c.Key("clear-all"), "清空").Clicked() {
					a.cacheClearing = true
				}
			}
		})
		if len(a.cacheSongs) == 0 {
			ui.Column(c).Grow(1).Center().Gap(7).Children(func() {
				ui.Icon(c, iconNote).Size(30, 30).TextColor(p.faint)
				ui.Text(c, "还没有缓存的歌曲").FontSize(13).FontWeight(500).TextColor(p.muted)
				ui.Text(c, "播放过的歌曲会自动保存在本机").FontSize(12).TextColor(p.faint)
			})
			return
		}
		a.cacheList.Key = func(row int) any {
			if row >= len(a.cacheSongs) {
				return int64(-row - 1)
			}
			return a.cacheSongs[row].ID
		}
		ui.List(c.Key("cache-songs"), &a.cacheList, len(a.cacheSongs), func(i int) {
			a.cachedSongRow(c, i, p)
		}).Grow(1)
	})
}

// cachedSongRow shows a song the cache keeps, and drops it when its trash
// is clicked.
func (a *app) cachedSongRow(c *ui.Context, i int, p palette) {
	s := &a.cacheSongs[i]
	row := ui.Row(c).Height(58).PaddingX(16).Gap(12).AlignItems(ui.Center)
	bg := ui.Transparent
	if row.Hovered() {
		bg = p.hover
	}
	row.Background(row.AnimateColor("bg", bg, ui.Animation{Duration: 100 * time.Millisecond}))
	row.Children(func() {
		a.coverPal(c, p, s.Cover, 72, 38, 6)
		name := s.Name
		if name == "" {
			name = fmt.Sprintf("歌曲 %d", s.ID)
		}
		ui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(3).Children(func() {
			ui.Text(c, name).FontSize(13.5).FontWeight(500).SingleLine()
			artists := s.Artists
			if artists == "" {
				artists = "未知歌手"
			}
			ui.Text(c, artists).FontSize(12).TextColor(p.muted).SingleLine()
		})
		ui.Column(c).AlignItems(ui.End).Gap(3).Children(func() {
			ui.Text(c, cachedLevel(s.Level)+" · "+cachedSize(s.Size)).FontSize(11.5).TextColor(p.muted).SingleLine()
			ui.Text(c, cachedAt(s.Kept)).FontSize(11.5).TextColor(p.faint).SingleLine()
		})
		if iconButton(c.Key("remove"), iconTrash, 15, p.muted, "删除 "+name).Clicked() {
			a.cacheDropping = s.ID
		}
	})
}
