package app

import (
	"time"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/player"
)

// applyEffect puts the effect the listener chose over what plays, or takes it
// off.
func (a *app) applyEffect() {
	var e *player.Effect
	if a.settings.EffectsOn {
		e = effectPreset(a.settings.Effect)
	}
	a.pl.SetEffect(e)
}

// audioEffects are the effects the player offers, in the order the desktop
// client lists them: the six recommended ones first, then every other one.
var audioEffects = []string{
	"3D环绕", "清澈人声", "高解析人声", "360°环绕", "超重低音", "LiveHouse现场",
	"水晶立体声", "动感电音", "HiFi现场", "迷幻电音", "震撼全景", "婉约古风",
	"环绕立体声", "HiFi电子管", "教堂混响", "怀旧卡带机", "NINEONE#专属音效", "纯净ACG",
	"摇滚经典", "专辑定制音效", "复古收音机", "毛不易专属音效", "录音棚立体声", "音乐厅",
	"演唱会现场", "独享立体声", "狂嗨LIVE", "民谣音效", "极重低音", "嘻哈音效",
}

// effectNow says which effect plays, as the popover shows it beside its switch.
func (a *app) effectNow() string {
	switch {
	case !a.settings.EffectsOn:
		return "已关闭"
	case a.settings.Effect == "":
		return "未选择"
	}
	return a.settings.Effect
}

// effectsButton is the whale tail, which opens the effects in a popover beside
// it: the switch that turns them on, and the presets to pick from.
func (a *app) effectsButton(c *ui.Context, color ui.Color) {
	lit := color
	if a.settings.EffectsOn && a.settings.Effect != "" {
		lit = a.pal.accent
	}
	b := iconButton(c.Key("effects-button"), iconWhale, 16, lit, "音效")
	if b.Clicked() {
		a.fxOpen = !a.fxOpen
	}
	pop := popoverColors(a.pal.dark)
	ui.PopoverBase(c.Key("effects-popover"), b, &a.fxOpen, func(panel ui.Element) {
		panel.AttachTo(b, ui.AnchorTop, ui.AnchorBottom).Margin(0, 0, 11, 0).
			Width(348).Padding(13, 14, 14, 14).Radius(14).Background(pop.fill).Border(0.5, pop.edge).
			Shadow(0, 8, 24, -4, ui.RGBA(0, 0, 0, 0.2)).Gap(11).Label("音效")
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			sw := ui.Switch(c, &a.settings.EffectsOn).Label("音效开关")
			if sw.Changed() {
				a.settings.save()
				a.applyEffect()
			}
			ui.Text(c, "音效").FontSize(13).FontWeight(700).TextColor(a.pal.text)
			ui.Box(c).Grow(1)
			ui.Text(c, a.effectNow()).FontSize(12).TextColor(a.pal.muted).SingleLine()
		})
		ui.Grid(c).Columns(3).GapX(8).GapY(6).Children(func() {
			for _, name := range audioEffects {
				a.effectsCell(c, name)
			}
		})
		// The ones the client changes by convolution alone are a dot: the
		// desktop client's own sound, which we have no coefficients for.
		ui.Text(c, "· 的预设靠卷积音色变化，暂未复现，选它声音不变").
			FontSize(11).TextColor(a.pal.muted).SingleLine()
	})
}

// effectsCell is one preset in the effects popover: the chosen one carries the
// accent, the others only show it under the pointer, and all of them fade
// while the effects are off.
func (a *app) effectsCell(c *ui.Context, name string) {
	// A preset we have the parameters of; the others change the client's
	// sound through convolution, which leaves them their name alone.
	label := name
	if effectPreset(name) == nil {
		label += "·"
	}
	cell := ui.ButtonBase(c.Key("fx-" + name)).Height(30).PaddingX(8).Radius(8).Center().Label(name)
	on := a.settings.EffectsOn
	sel := a.settings.Effect == name
	fill, fg := ui.Transparent, a.pal.text
	if effectPreset(name) == nil {
		fg = a.pal.muted
	}
	if !on && !sel {
		fg = a.pal.faint
	}
	switch {
	case sel && on:
		fill, fg = a.pal.accent.Alpha(0.13), a.pal.accent
	case sel:
		fill, fg = a.pal.accent.Alpha(0.13), a.pal.muted
	case cell.Hovered():
		fill = a.pal.hover
	}
	cell.Background(cell.AnimateColor("bg", fill, ui.Animation{Duration: 120 * time.Millisecond}))
	pose := ui.Pose{}
	if cell.Pressed() {
		pose.Scale = 0.97
	}
	cell.AnimatePose(pose, ui.Snappy)
	cell.Children(func() { ui.Text(c, label).FontSize(12).FontWeight(600).TextColor(fg).SingleLine() })
	// Picking a preset turns the effects on, as the client does: one that
	// leaves them off would take the click and change nothing.
	if cell.Clicked() {
		a.settings.Effect, a.settings.EffectsOn = name, true
		a.settings.save()
		a.applyEffect()
	}
}
