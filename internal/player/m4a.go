package player

import (
	"encoding/binary"
	"errors"
	"io"
	"sync"

	"github.com/egoist/mygo/audio"
	aacpcm "github.com/tphakala/go-aac/pcm"

	"netease-music-mygo/internal/mp4"
)

func init() {
	mp4.RegisterCodec("mp4a", func(config []byte) (mp4.Codec, error) { return newAACCodec(config) })
}

// m4aStream decodes the sound of an MP4, which is the only way to play
// what the cloud holds: NetEase serves an upload as the .m4a it was, AAC
// or Apple Lossless, and the player itself decodes MP3 and FLAC alone. Its
// packets are placed by the movie header, which a file may keep after its
// sound, so it opens once the song has all come in.
type m4aStream struct {
	dl   *download
	dec  audio.Decoder
	pos  int64
	spec *spectrum
	once sync.Once
}

func openM4AStream(dl *download) (*m4aStream, error) {
	if err := dl.waitAll(); err != nil {
		return nil, err
	}
	dec, err := mp4.NewDecoder(&downloadSeeker{d: dl})
	if err != nil {
		return nil, err
	}
	rate := dec.SampleRate()
	if rate <= 0 {
		return nil, errors.New("m4a: no sample rate")
	}
	s := &m4aStream{dl: dl, dec: dec}
	s.spec = newSpectrum(rate, max(dec.Len(), 0))
	return s, nil
}

func (s *m4aStream) SampleRate() int { return s.dec.SampleRate() }
func (s *m4aStream) Channels() int   { return s.dec.Channels() }
func (s *m4aStream) Len() int64      { return s.dec.Len() }

func (s *m4aStream) Read(dst []float32) (int, error) {
	n, err := s.dec.Read(dst)
	if n > 0 {
		s.spec.feed(s.pos, dst[:n])
		s.pos += int64(n / 2)
	}
	return n, err
}

func (s *m4aStream) SetPosition(frame int64) error {
	if err := s.dec.SetPosition(frame); err != nil {
		return err
	}
	s.pos = frame
	return nil
}

func (s *m4aStream) source() (*download, *spectrum) { return s.dl, s.spec }

func (s *m4aStream) Close() error {
	s.once.Do(s.dl.Close)
	return nil
}

// aacCodec decodes the raw AAC packets of an MP4 track, configured by its
// AudioSpecificConfig, to stereo. HE-AAC, whose SBR the decoder does not,
// fails to open.
type aacCodec struct {
	fd  *aacpcm.FrameDecoder
	pcm []byte
}

func newAACCodec(asc []byte) (*aacCodec, error) {
	fd, err := aacpcm.NewRawDecoder(asc)
	if err != nil {
		return nil, err
	}
	return &aacCodec{fd: fd}, nil
}

func (c *aacCodec) SampleRate() int   { return c.fd.SampleRate() }
func (c *aacCodec) Reset()            { c.fd.Reset() }
func (c *aacCodec) Preroll() int      { return 1 }
func (c *aacCodec) PacketFrames() int { return 1024 }

func (c *aacCodec) Decode(dst []float32, packet []byte) ([]float32, error) {
	pcm, n, err := c.fd.DecodeFrame(c.pcm[:0], packet)
	if err != nil {
		return dst, err
	}
	c.pcm = pcm
	ch := c.fd.Channels()
	right := 0
	if ch > 1 {
		right = 2
	}
	for i := range n {
		f := pcm[i*ch*2:]
		dst = append(dst,
			float32(int16(binary.LittleEndian.Uint16(f)))/(1<<15),
			float32(int16(binary.LittleEndian.Uint16(f[right:])))/(1<<15))
	}
	return dst, nil
}

// A downloadSeeker reads a download all in, as the MP4 decoder does: from
// its end for the size, and at offsets for its header and packets.
type downloadSeeker struct {
	d   *download
	pos int64
}

func (r *downloadSeeker) Read(p []byte) (int, error) {
	d := r.d
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.n <= r.pos {
		if d.err != nil {
			return 0, d.err
		}
		return 0, io.EOF
	}
	n, err := d.readAt(p, r.pos)
	if n == 0 {
		return 0, err
	}
	r.pos += int64(n)
	return n, nil
}

func (r *downloadSeeker) ReadAt(p []byte, off int64) (int, error) {
	d := r.d
	d.mu.Lock()
	defer d.mu.Unlock()
	n, err := d.readAt(p, off)
	if n == len(p) {
		return n, nil
	}
	if err == nil {
		err = io.ErrUnexpectedEOF
	}
	return n, err
}

func (r *downloadSeeker) Seek(off int64, whence int) (int64, error) {
	d := r.d
	d.mu.Lock()
	at := off
	switch whence {
	case io.SeekCurrent:
		at += r.pos
	case io.SeekEnd:
		at += d.n
	case io.SeekStart:
	default:
		d.mu.Unlock()
		return 0, errors.New("m4a: bad whence")
	}
	d.mu.Unlock()
	pos := d.size
	if pos <= 0 {
		pos = d.n
	}
	if at < 0 || at > pos {
		return 0, errors.New("m4a: out of range")
	}
	r.pos = at
	return at, nil
}
