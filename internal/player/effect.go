package player

import (
	"math"
	"sync/atomic"

	"github.com/egoist/mygo/audio"
)

// An Effect is a sound effect applied to what plays, as the presets of the
// client are: a ten-band graphic EQ, a bass and a treble shelf, a wider stereo
// image and a room. A preset is nothing but which of them it uses, and how
// much.
type Effect struct {
	// Bands are the graphic EQ's gains, in dB, on bandHz; zero leaves a band
	// out.
	Bands [10]float64
	// Bass and Treble are the shelves' gains, in dB, at their frequencies;
	// zero leaves a shelf out.
	Bass, Treble     float64
	BassHz, TrebleHz float64
	// Width widens the stereo image, as the client's stereoizer does: 1
	// leaves it as it is, 2 doubles the sides.
	Width float64
	// Reverb adds the room a preset asks for, if any.
	Reverb *Reverb
}

// A Reverb is the room a preset puts a song in, as far as this app can make
// one: the decay, how the highs die in it, and how loud its two parts play.
type Reverb struct {
	// Decay is the seconds it takes to fall 60 dB.
	Decay float64
	// Damping is how much the highs die first, from 0 to 1.
	Damping float64
	// Density is how many echoes of the room there are, from 0 to 100.
	Density float64
	// Diffusion is how smeared they are, from 0 to 1.
	Diffusion float64
	// Wet is the tail's level, and Early the reflections' before it, in dB:
	// 0 is as loud as the song.
	Wet, Early float64
}

// bandHz are the graphic EQ's bands, an octave apart, as the client's are.
var bandHz = [10]float64{31.5, 63, 125, 250, 500, 1000, 2000, 4000, 8000, 16000}

// bandQ widens each band to about an octave, so neighbours overlap the way a
// graphic EQ's do.
const bandQ = 1.0

// biquad is a second-order section, its coefficients divided through by a0.
type biquad struct {
	b0, b1, b2, a1, a2 float64
}

// state is the two past samples a biquad keeps, of a channel.
type state struct {
	x1, x2, y1, y2 float64
}

func (q biquad) run(s *state, x float64) float64 {
	y := q.b0*x + q.b1*s.x1 + q.b2*s.x2 - q.a1*s.y1 - q.a2*s.y2
	s.x2, s.x1 = s.x1, x
	s.y2, s.y1 = s.y1, y
	return y
}

// peaking is a bell of gain db at hz, q samples wide.
func peaking(hz, db, q, rate float64) biquad {
	a := math.Pow(10, db/40)
	w := 2 * math.Pi * hz / rate
	alpha := math.Sin(w) / (2 * q)
	a0 := 1 + alpha/a
	return biquad{
		b0: (1 + alpha*a) / a0,
		b1: (-2 * math.Cos(w)) / a0,
		b2: (1 - alpha*a) / a0,
		a1: (-2 * math.Cos(w)) / a0,
		a2: (1 - alpha/a) / a0,
	}
}

// lowShelf lifts or cuts what is below hz by db, its slope a shelf's.
func lowShelf(hz, db, rate float64) biquad {
	a := math.Pow(10, db/40)
	w := 2 * math.Pi * hz / rate
	alpha := math.Sin(w) / 2 * math.Sqrt2
	c, s := math.Cos(w), 2*math.Sqrt(a)*alpha
	a0 := (a + 1) + (a-1)*c + s
	return biquad{
		b0: a * ((a + 1) - (a-1)*c + s) / a0,
		b1: 2 * a * ((a - 1) - (a+1)*c) / a0,
		b2: a * ((a + 1) - (a-1)*c - s) / a0,
		a1: -2 * ((a - 1) + (a+1)*c) / a0,
		a2: ((a + 1) + (a-1)*c - s) / a0,
	}
}

// highShelf lifts or cuts what is above hz by db.
func highShelf(hz, db, rate float64) biquad {
	a := math.Pow(10, db/40)
	w := 2 * math.Pi * hz / rate
	alpha := math.Sin(w) / 2 * math.Sqrt2
	c, s := math.Cos(w), 2*math.Sqrt(a)*alpha
	a0 := (a + 1) - (a-1)*c + s
	return biquad{
		b0: a * ((a + 1) + (a-1)*c + s) / a0,
		b1: -2 * a * ((a - 1) + (a+1)*c) / a0,
		b2: a * ((a + 1) + (a-1)*c - s) / a0,
		a1: 2 * ((a - 1) - (a+1)*c) / a0,
		a2: ((a + 1) - (a-1)*c - s) / a0,
	}
}

// chainPlan is what an effect does to the samples: built away from the audio
// thread, then swapped in whole, so picking a preset never waits on the buffer
// playing.
type chainPlan struct {
	sect  []biquad
	width float64
	rvb   *reverbUnit
}

// effectChain decodes the song and puts the effect over its samples, as they
// play: it is the decoder package audio reads.
type effectChain struct {
	dec  audio.Decoder
	ch   int
	rate float64

	cur atomic.Pointer[chainPlan]
	// eff is the effect the plan was built from: picking the same preset
	// again builds nothing.
	eff *Effect
	// last is the plan the states belong to, and state the sections' memory
	// of the past frames, a channel's after another's. Only the audio thread
	// touches them.
	last  *chainPlan
	state []state
}

func newEffectChain(dec audio.Decoder, e *Effect) *effectChain {
	c := &effectChain{dec: dec, ch: max(dec.Channels(), 1), rate: float64(dec.SampleRate())}
	c.set(e)
	return c
}

// set puts the effect e over what plays from the next buffer on, and reports
// whether it changed: it builds the plan off the audio thread and swaps it in
// at once. The sections keep nothing of what played before them.
func (c *effectChain) set(e *Effect) bool {
	if e == c.eff {
		return false
	}
	c.eff = e
	c.cur.Store(c.plan(e))
	return true
}

func (c *effectChain) plan(e *Effect) *chainPlan {
	p := &chainPlan{}
	if e == nil {
		return p
	}
	for i, db := range e.Bands {
		if db != 0 {
			p.sect = append(p.sect, peaking(bandHz[i], db, bandQ, c.rate))
		}
	}
	if e.Bass != 0 {
		p.sect = append(p.sect, lowShelf(orDefault(e.BassHz, 120), e.Bass, c.rate))
	}
	if e.Treble != 0 {
		p.sect = append(p.sect, highShelf(orDefault(e.TrebleHz, 6000), e.Treble, c.rate))
	}
	if e.Width > 1 {
		p.width = e.Width
	}
	if e.Reverb != nil && e.Reverb.Decay > 0 {
		p.rvb = newReverb(e.Reverb, c.rate, min(c.ch, 2))
	}
	return p
}

func orDefault(v, d float64) float64 {
	if v <= 0 {
		return d
	}
	return v
}

func (c *effectChain) SampleRate() int { return c.dec.SampleRate() }
func (c *effectChain) Channels() int   { return c.dec.Channels() }
func (c *effectChain) Len() int64      { return c.dec.Len() }

func (c *effectChain) SetPosition(frame int64) error {
	c.last = nil // the sections start again from silence
	return c.dec.SetPosition(frame)
}

func (c *effectChain) Read(dst []float32) (int, error) {
	n, err := c.dec.Read(dst)
	if n > 0 {
		c.process(dst[:n])
	}
	return n, err
}

// process runs every frame through the sections, widens it, puts the room
// around it, and keeps it under the rails.
func (c *effectChain) process(buf []float32) {
	p := c.cur.Load()
	if p == nil || (len(p.sect) == 0 && p.width == 0 && p.rvb == nil) {
		return
	}
	ch := c.ch
	if need := len(p.sect) * ch; len(c.state) < need {
		c.state = make([]state, need)
	} else if p != c.last {
		clear(c.state)
	}
	c.last = p

	width, rvb := p.width, p.rvb
	for i := 0; i+ch <= len(buf); i += ch {
		for k := 0; k < ch && k < 2; k++ {
			x := float64(buf[i+k])
			for j := range p.sect {
				x = p.sect[j].run(&c.state[j*ch+k], x)
			}
			buf[i+k] = float32(x)
		}
		if width != 0 && ch >= 2 {
			l, r := float64(buf[i]), float64(buf[i+1])
			mid, side := (l+r)/2, (l-r)/2*width
			buf[i], buf[i+1] = float32(mid+side), float32(mid-side)
		}
		if rvb != nil && ch >= 2 {
			l := float64(buf[i]) + rvb.run(0, float64(buf[i]))
			r := float64(buf[i+1]) + rvb.run(1, float64(buf[i+1]))
			buf[i], buf[i+1] = float32(softClip(l)), float32(softClip(r))
			continue
		}
		for k := 0; k < ch && k < 2; k++ {
			buf[i+k] = float32(softClip(float64(buf[i+k])))
		}
	}
}

// A combLine is one echo of a room: a delay it feeds back into itself, the
// highs gone a little more each pass.
type combLine struct {
	buf  []float64
	at   int
	fb   float64
	damp float64
	low  float64
}

func (c *combLine) run(x float64) float64 {
	y := c.buf[c.at]
	c.low = y*(1-c.damp) + c.low*c.damp
	c.buf[c.at] = x + c.low*c.fb
	if c.at++; c.at == len(c.buf) {
		c.at = 0
	}
	return y
}

// allpassLine smears an echo of a room, as a Schroeder allpass does.
type allpassLine struct {
	buf []float64
	at  int
	g   float64
}

func (a *allpassLine) run(x float64) float64 {
	buf := a.buf[a.at]
	y := buf - a.g*x
	a.buf[a.at] = x + a.g*y
	if a.at++; a.at == len(a.buf) {
		a.at = 0
	}
	return y
}

// roomChannel is one side of a room: its own echoes, allpasses and walls.
type roomChannel struct {
	combs []combLine
	aps   []allpassLine
	line  []float64
	at    int
	taps  []int
}

// reverbUnit is a room, each side of it fed its own channel: the two sides
// share the decay but not the echoes, so the tail is around the song rather
// than summed down its middle.
type reverbUnit struct {
	chans      []roomChannel
	wet, early float64
}

// newReverb builds the room rv asks for, at rate hz, for ch channels.
func newReverb(rv *Reverb, rate float64, ch int) *reverbUnit {
	scale := rate / 44100
	u := &reverbUnit{wet: math.Pow(10, rv.Wet/20), early: math.Pow(10, rv.Early/20)}
	combs := []int{1116, 1188, 1277, 1356}
	if rv.Density < 50 {
		combs = combs[:3]
	}
	g := min(0.2+0.5*rv.Diffusion, 0.7)
	// The walls before the tail: 8 to 31 ms.
	wallMs := []float64{8, 11, 14, 19, 24, 31}
	for c := 0; c < max(ch, 1); c++ {
		// The right side's echoes come a little later, as a room's do.
		offset := 0
		if c == 1 {
			offset = 23
		}
		var room roomChannel
		for _, d := range combs {
			delay := max(int(float64(d+offset)*scale), 8)
			passes := rv.Decay / (float64(delay) / rate)
			fb := min(math.Pow(10, -3/max(passes, 1)), 0.97)
			room.combs = append(room.combs, combLine{buf: make([]float64, delay), fb: fb, damp: rv.Damping})
		}
		for i, d := range []int{556, 441} {
			delay := max(int(float64(d+offset*(i+1))*scale), 8)
			room.aps = append(room.aps, allpassLine{buf: make([]float64, delay), g: g})
		}
		longest := 0
		for i, ms := range wallMs {
			d := int(ms/1000*rate) + offset*(i%3)
			room.taps = append(room.taps, d)
			longest = max(longest, d)
		}
		room.line = make([]float64, longest+1)
		u.chans = append(u.chans, room)
	}
	return u
}

func (u *reverbUnit) run(side int, x float64) float64 {
	c := &u.chans[min(side, len(u.chans)-1)]
	var reflections float64
	for _, d := range c.taps {
		i := c.at - d
		if i < 0 {
			i += len(c.line)
		}
		reflections += c.line[i]
	}
	c.line[c.at] = x
	if c.at++; c.at == len(c.line) {
		c.at = 0
	}
	var tail float64
	for i := range c.combs {
		tail += c.combs[i].run(x)
	}
	tail /= float64(len(c.combs))
	for i := range c.aps {
		tail = c.aps[i].run(tail)
	}
	return u.early*reflections/float64(len(c.taps)) + u.wet*tail
}

// softClip is the last guard at 0 dB: what stays under the rails is untouched,
// and only what a lift pushed over them is bent back.
func softClip(x float64) float64 {
	const knee = 0.98
	if x > knee {
		return knee + (1-knee)*math.Tanh((x-knee)/(1-knee))
	}
	if x < -knee {
		return -knee - (1-knee)*math.Tanh((-x-knee)/(1-knee))
	}
	return x
}
