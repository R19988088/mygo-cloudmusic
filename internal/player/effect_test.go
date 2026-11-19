package player

import (
	"io"
	"math"
	"testing"
	"time"
)

// tone feeds a sine of hz at the gain given for frames frames, then silence,
// then nothing: a decoder with no download behind it.
type tone struct {
	hz      float64
	rate    int
	frames  int // the frames of the tone
	silence int // the frames of silence after them
	at      int
	gain    float64
}

func (t *tone) SampleRate() int           { return t.rate }
func (t *tone) Channels() int             { return 2 }
func (t *tone) Len() int64                { return int64(t.frames + t.silence) }
func (t *tone) SetPosition(f int64) error { t.at = int(f); return nil }

func (t *tone) Read(buf []float32) (int, error) {
	n := 0
	for ; n+2 <= len(buf); n += 2 {
		if t.at >= t.frames+t.silence {
			return n, io.EOF
		}
		v := 0.0
		if t.at < t.frames {
			v = t.gain * math.Sin(2*math.Pi*t.hz*float64(t.at)/float64(t.rate))
		}
		buf[n], buf[n+1] = float32(v), float32(v)
		t.at++
	}
	return n, nil
}

// rms is the level the chain settles at over the tone's first d, after a
// quarter of it.
func rms(t *testing.T, c *effectChain, d time.Duration) float64 {
	t.Helper()
	const rate = 44100
	buf := make([]float32, 1024)
	settled := int(0.25 * rate * d.Seconds())
	total := int(rate * d.Seconds())
	var sum float64
	var count, elapsed int
	for elapsed < total {
		n, _ := c.Read(buf)
		if n == 0 {
			break
		}
		for i := 0; i+1 < n; i += 2 {
			if elapsed+i/2 >= settled {
				sum += float64(buf[i]) * float64(buf[i])
				count++
			}
		}
		elapsed += n / 2
	}
	if count == 0 {
		return 0
	}
	return math.Sqrt(sum / float64(count))
}

// The lift a preset puts on its bands is heard: 40 Hz comes out louder than it
// went in, while 10 kHz, which the preset leaves alone, comes out as loud.
func TestGraphLift(t *testing.T) {
	const rate, frames = 44100, 44100
	effect := &Effect{Bands: [10]float64{6, 4.5, 3.2, 2.1}}
	level := func(hz float64, e *Effect) float64 {
		c := newEffectChain(&tone{hz: hz, rate: rate, frames: frames * 2, gain: 0.2}, e)
		return rms(t, c, time.Second)
	}
	low, lowPlain := level(40, effect), level(40, nil)
	high, highPlain := level(10000, effect), level(10000, nil)
	if low < lowPlain*1.5 {
		t.Fatalf("40 Hz at %.4f, %.4f without the effect", low, lowPlain)
	}
	if math.Abs(high-highPlain) > highPlain*0.1 {
		t.Fatalf("10 kHz at %.4f, %.4f without the effect", high, highPlain)
	}
}

// A preset that only fills a room with reverb still changes what plays: the
// sound rings on after the song stops.
func TestReverbRings(t *testing.T) {
	const rate = 44100
	e := &Effect{Reverb: &Reverb{Decay: 2.4, Damping: 0.02, Density: 60, Diffusion: 0.8, Wet: -20, Early: -15}}
	src := &tone{hz: 500, rate: rate, frames: rate / 4, silence: rate / 2, gain: 0.3}
	c := newEffectChain(src, e)
	buf := make([]float32, 1024)
	var rang bool
	for {
		n, _ := c.Read(buf)
		if n == 0 {
			break
		}
		if src.at > src.frames {
			for i := 0; i < n; i++ {
				if math.Abs(float64(buf[i])) > 1e-3 {
					rang = true
				}
			}
		}
	}
	if !rang {
		t.Fatal("nothing rang on after the song")
	}
}
