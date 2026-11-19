package app

import "netease-music-mygo/internal/player"

// effectPresets are the effects the player puts over a song, from the bands,
// shelves, stereo width and reverb the client's own presets carry. Their
// convolution and ambience parts are not here: a convolution preset, whose
// impulse is in its own file, plays plain.
var effectPresets = map[string]*player.Effect{
	"3D环绕": &player.Effect{
		Bands: [10]float64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		Width: 1.4,
	},
	"清澈人声": &player.Effect{
		Bands: [10]float64{-1.5, -1, 0, 1.79, 4, 3.79, 1.79, 0.5, -1, 1},
	},
	"LiveHouse现场": &player.Effect{
		Bands: [10]float64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		Reverb: &player.Reverb{Decay: 1.5, Damping: 0.03, Density: 60, Diffusion: 0.8, Wet: -20, Early: -23},
	},
	"动感电音": &player.Effect{
		Bands: [10]float64{4, 4, 3, 1, -1, -2, -1, 2, 4, 4},
		Reverb: &player.Reverb{Decay: 2.5, Damping: 0.07, Density: 50, Diffusion: 0.85, Wet: -24, Early: -22},
	},
	"迷幻电音": &player.Effect{
		Bands: [10]float64{5, 6.09, 2.2, 0, -1.6, 1.5, 0, 1, 2.57, 3.77},
		Bass: 3, BassHz: 120,
		Treble: 2, TrebleHz: 6000,
	},
	"婉约古风": &player.Effect{
		Bands: [10]float64{4, 2, 0, -0.5, -1, 3.5, 4.5, 1, -1, 3},
		Bass: 2, BassHz: 120,
		Treble: 2, TrebleHz: 6000,
	},
	"教堂混响": &player.Effect{
		Bands: [10]float64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		Reverb: &player.Reverb{Decay: 2.4, Damping: 0.02, Density: 60, Diffusion: 0.8, Wet: -20, Early: -15},
	},
	"NINEONE#专属音效": &player.Effect{
		Bands: [10]float64{0, 0, -1, -1, -2, 1, 2, 2, 0, 0},
		Reverb: &player.Reverb{Decay: 1.2, Damping: 0.04, Density: 35, Diffusion: 1, Wet: -22, Early: -28},
	},
	"纯净ACG": &player.Effect{
		Bands: [10]float64{4, 6.19, 1.5, -0.5, -0.8, 2, 5.19, 1, -1.5, 4},
		Bass: 2, BassHz: 120,
		Treble: 3, TrebleHz: 6000,
	},
	"摇滚经典": &player.Effect{
		Bands: [10]float64{-5.7, 0, 1.5, -3.81, -0.2, -1.1, 0.6, 1.5, 1.91, 1.91},
		Width: 1.5,
	},
	"专辑定制音效": &player.Effect{
		Bands: [10]float64{-1, -1, -2, -3, -2, -2, -1, -1, -1, -1},
		Width: 1.15,
	},
	"毛不易专属音效": &player.Effect{
		Bands: [10]float64{-5, -4, -2, -0.5, 1.5, 2.5, 0, -3, -5, -5},
		Reverb: &player.Reverb{Decay: 1, Damping: 0.02, Density: 80, Diffusion: 1, Wet: -80, Early: -14},
	},
	"演唱会现场": &player.Effect{
		Bands: [10]float64{5.59, 5, 3, 0, -1, 0, 3.09, 4.5, 4, 5},
		Bass: 3, BassHz: 120,
		Treble: 2, TrebleHz: 6000,
	},
	"狂嗨LIVE": &player.Effect{
		Bands: [10]float64{-6, -6.5, -3.7, -9.93, -12, -12, -12, -12, -12, -12},
		Bass: 12, BassHz: 120,
		Treble: -12, TrebleHz: 6000,
		Width: 2,
		Reverb: &player.Reverb{Decay: 3.59, Damping: 0.02, Density: 60, Diffusion: 0.4, Wet: -20, Early: -3},
	},
	"民谣音效": &player.Effect{
		Bands: [10]float64{0, 3, 0, -0.8, 1.5, 4.8, 5.8, 3.2, -0.5, 2},
		Treble: 1, TrebleHz: 6000,
	},
	"极重低音": &player.Effect{
		Bands: [10]float64{6, 4.5, 3.2, 2.09, 0, 0, 0, 0, 0, 0},
		Bass: 3, BassHz: 120,
	},
	"嘻哈音效": &player.Effect{
		Bands: [10]float64{5.6, 5.09, 2, 0.5, -2, 1.2, 3.79, 0.5, 1.5, 4},
		Bass: 2, BassHz: 120,
		Treble: 2, TrebleHz: 6000,
	},
}

// effectPreset is the effect a preset applies, if this app can play it. It
// returns the same one for the same name, so picking it again builds nothing.
func effectPreset(name string) *player.Effect {
	return effectPresets[name]
}
