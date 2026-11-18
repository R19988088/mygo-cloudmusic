package player

import (
	"math"
	"os"
	"testing"
)

// TestM4ADecode decodes the file $NM_M4A_FILE, a cloud upload's .m4a, for
// the decoder to be tried on real AAC.
func TestM4ADecode(t *testing.T) {
	path := os.Getenv("NM_M4A_FILE")
	if path == "" {
		t.Skip()
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		t.Fatal(err)
	}
	s, err := openM4AStream(opened(f, info.Size()))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	t.Logf("rate=%d len=%d (~%.1fs)", s.SampleRate(), s.Len(), float64(s.Len())/float64(s.SampleRate()))

	dst := make([]float32, 4800*2) // 100ms at 48k, or a little more at 44.1k
	var peak float64
	var frames int64
	for frames < int64(s.SampleRate())*2 { // two seconds
		n, err := s.Read(dst)
		if err != nil || n == 0 {
			t.Logf("read ended: %v", err)
		}
		if n == 0 {
			break
		}
		for _, v := range dst[:n] {
			peak = math.Max(peak, math.Abs(float64(v)))
		}
		frames += int64(n / 2)
	}
	if peak < 0.01 {
		t.Fatalf("silent: peak %f", peak)
	}
	t.Logf("first 2s peak=%f frames=%d", peak, frames)

	// From the middle, as a seek: the sound there is the song's.
	at := s.Len() / 2
	if err := s.SetPosition(at); err != nil {
		t.Fatal(err)
	}
	peak, frames = 0, 0
	for frames < int64(s.SampleRate()) { // a second
		n, err := s.Read(dst)
		if err != nil || n == 0 {
			break
		}
		for _, v := range dst[:n] {
			peak = math.Max(peak, math.Abs(float64(v)))
		}
		frames += int64(n / 2)
	}
	t.Logf("middle 1s peak=%f at=%d", peak, at)
	if peak < 0.01 {
		t.Fatalf("silent after seek: peak %f", peak)
	}
}
