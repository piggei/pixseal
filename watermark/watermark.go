package watermark

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"image"
	"image/color"
	"math"
	"math/rand"
)

const (
	blockSize  = 8
	maxPayload = 64
	headerSize = 8 // magic(2), version(1), length(1), crc32(4)
	tagSize    = 8
)

var magic = [2]byte{'P', 'S'}

type Options struct {
	Strength   float64
	Repetition int
}

func DefaultOptions() Options { return Options{Strength: 34, Repetition: 5} }

func normalize(o Options) (Options, error) {
	if o.Strength == 0 { o.Strength = 34 }
	if o.Repetition == 0 { o.Repetition = 5 }
	if o.Strength < 4 || o.Strength > 120 { return o, errors.New("strength must be between 4 and 120") }
	if o.Repetition < 1 || o.Repetition > 31 || o.Repetition%2 == 0 { return o, errors.New("repetition must be an odd number between 1 and 31") }
	return o, nil
}

func Capacity(img image.Image, repetition int) int {
	if repetition < 1 { repetition = 5 }
	b := img.Bounds()
	blocks := (b.Dx()/blockSize)*(b.Dy()/blockSize)
	bytes := blocks/(8*repetition) - headerSize - tagSize
	if bytes < 0 { return 0 }
	if bytes > maxPayload { return maxPayload }
	return bytes
}

func Embed(src image.Image, payload, key []byte, opts Options) (*image.NRGBA, error) {
	opts, err := normalize(opts); if err != nil { return nil, err }
	if len(key) < 8 { return nil, errors.New("key must contain at least 8 bytes") }
	if len(payload) > maxPayload { return nil, fmt.Errorf("payload exceeds %d bytes", maxPayload) }
	if len(payload) > Capacity(src, opts.Repetition) { return nil, fmt.Errorf("image capacity is %d bytes with repetition %d", Capacity(src, opts.Repetition), opts.Repetition) }
	packet := makePacket(payload, key)
	bits := bytesToBits(packet)
	bits = whiten(bits, key)
	order := blockOrder(src.Bounds(), key)
	needed := len(bits)*opts.Repetition
	if needed > len(order) { return nil, errors.New("image does not contain enough complete 8x8 blocks") }

	out := toNRGBA(src)
	for i := 0; i < needed; i++ {
		bit := bits[i/opts.Repetition]
		embedBlock(out, order[i], bit, opts.Strength)
	}
	return out, nil
}

func Extract(src image.Image, key []byte, opts Options) ([]byte, float64, error) {
	opts, err := normalize(opts); if err != nil { return nil, 0, err }
	if len(key) < 8 { return nil, 0, errors.New("key must contain at least 8 bytes") }
	order := blockOrder(src.Bounds(), key)
	minBits := (headerSize+tagSize)*8
	if len(order) < minBits*opts.Repetition { return nil, 0, errors.New("image is too small") }

	// Decode enough for the maximum packet; unused tail bits are harmless.
	nbits := len(order)/opts.Repetition
	maxBits := (headerSize+maxPayload+tagSize)*8
	if nbits > maxBits { nbits = maxBits }
	bits := make([]byte, nbits)
	confidenceSum := 0.0
	for i := 0; i < nbits; i++ {
		votes, margin := 0, 0.0
		for r := 0; r < opts.Repetition; r++ {
			v := readBlock(src, order[i*opts.Repetition+r])
			if v >= 0 { votes++ } else { votes-- }
			margin += math.Abs(v)
		}
		if votes > 0 { bits[i] = 1 }
		confidenceSum += margin/float64(opts.Repetition)
	}
	bits = whiten(bits, key)
	raw := bitsToBytes(bits)
	if len(raw) < headerSize+tagSize || raw[0] != magic[0] || raw[1] != magic[1] || raw[2] != 1 {
		return nil, confidenceSum/float64(nbits), errors.New("watermark not found or key/repetition is incorrect")
	}
	n := int(raw[3])
	packetLen := headerSize+n+tagSize
	if n > maxPayload || packetLen > len(raw) { return nil, 0, errors.New("invalid watermark length") }
	packet := raw[:packetLen]
	payload := packet[headerSize:headerSize+n]
	if crc32.ChecksumIEEE(payload) != binary.BigEndian.Uint32(packet[4:8]) { return nil, 0, errors.New("watermark damaged: CRC mismatch") }
	mac := hmac.New(sha256.New, key); mac.Write(packet[:headerSize+n])
	if !hmac.Equal(packet[headerSize+n:], mac.Sum(nil)[:tagSize]) { return nil, 0, errors.New("watermark authentication failed") }
	return append([]byte(nil), payload...), confidenceSum/float64(nbits), nil
}

func makePacket(payload, key []byte) []byte {
	p := make([]byte, headerSize+len(payload)+tagSize)
	p[0], p[1], p[2], p[3] = magic[0], magic[1], 1, byte(len(payload))
	binary.BigEndian.PutUint32(p[4:8], crc32.ChecksumIEEE(payload))
	copy(p[headerSize:], payload)
	mac := hmac.New(sha256.New, key); mac.Write(p[:headerSize+len(payload)])
	copy(p[headerSize+len(payload):], mac.Sum(nil)[:tagSize])
	return p
}

func bytesToBits(in []byte) []byte { out:=make([]byte,len(in)*8); for i,b:=range in { for j:=0;j<8;j++ { out[i*8+j]=(b>>uint(7-j))&1 } }; return out }
func bitsToBytes(in []byte) []byte { out:=make([]byte,len(in)/8); for i:=range out { for j:=0;j<8;j++ { out[i]|=in[i*8+j]<<uint(7-j) } }; return out }

func whiten(bits []byte, key []byte) []byte {
	out:=append([]byte(nil),bits...); counter:=uint64(0); pos:=0
	for pos<len(out) { h:=sha256.New(); h.Write([]byte("pixseal-whiten-v1")); h.Write(key); var c [8]byte; binary.BigEndian.PutUint64(c[:],counter); h.Write(c[:]); sum:=h.Sum(nil); for _,b:=range sum { for j:=0;j<8 && pos<len(out);j++ { out[pos]^=(b>>uint(7-j))&1; pos++ } }; counter++ }
	return out
}

type point struct{ x,y int }
func blockOrder(bounds image.Rectangle, key []byte) []point {
	w,h:=bounds.Dx()/8,bounds.Dy()/8; out:=make([]point,0,w*h)
	for y:=0;y<h;y++ { for x:=0;x<w;x++ { out=append(out,point{bounds.Min.X+x*8,bounds.Min.Y+y*8}) } }
	hash:=sha256.Sum256(append(append([]byte("pixseal-order-v1"),key...), byte(w),byte(w>>8),byte(h),byte(h>>8)))
	r:=rand.New(rand.NewSource(int64(binary.LittleEndian.Uint64(hash[:8])))); r.Shuffle(len(out),func(i,j int){out[i],out[j]=out[j],out[i]}); return out
}

var cosTable [8][8]float64
func init(){ for u:=0;u<8;u++ { for x:=0;x<8;x++ { cosTable[u][x]=math.Cos((float64(2*x+1)*float64(u)*math.Pi)/16) } } }
func alpha(k int) float64 { if k==0{return 1/math.Sqrt2}; return 1 }

func embedBlock(img *image.NRGBA, p point, bit byte, strength float64) {
	var y,cb,cr [8][8]float64
	for yy:=0;yy<8;yy++ { for xx:=0;xx<8;xx++ { r,g,b,_:=img.At(p.x+xx,p.y+yy).RGBA(); rf,gf,bf:=float64(r>>8),float64(g>>8),float64(b>>8); y[yy][xx]=.299*rf+.587*gf+.114*bf-128; cb[yy][xx]=-.168736*rf-.331264*gf+.5*bf; cr[yy][xx]=.5*rf-.418688*gf-.081312*bf } }
	c:=dct(y); a,b:=math.Abs(c[2][3]),math.Abs(c[3][2]); target:=strength
	if bit==1 { if a-b<target { c[2][3]=math.Copysign(b+target,c[2][3]) } } else if b-a<target { c[3][2]=math.Copysign(a+target,c[3][2]) }
	y=idct(c)
	for yy:=0;yy<8;yy++ { for xx:=0;xx<8;xx++ { yyv:=y[yy][xx]+128; r:=yyv+1.402*cr[yy][xx]; g:=yyv-.344136*cb[yy][xx]-.714136*cr[yy][xx]; b:=yyv+1.772*cb[yy][xx]; img.SetNRGBA(p.x+xx,p.y+yy,color.NRGBA{clamp(r),clamp(g),clamp(b),255}) } }
}
func readBlock(img image.Image,p point) float64 { var y [8][8]float64; for yy:=0;yy<8;yy++ { for xx:=0;xx<8;xx++ { r,g,b,_:=img.At(p.x+xx,p.y+yy).RGBA(); y[yy][xx]=.299*float64(r>>8)+.587*float64(g>>8)+.114*float64(b>>8)-128 } }; c:=dct(y); return math.Abs(c[2][3])-math.Abs(c[3][2]) }
func dct(in [8][8]float64)(out [8][8]float64){ for v:=0;v<8;v++ { for u:=0;u<8;u++ { s:=0.0; for y:=0;y<8;y++ { for x:=0;x<8;x++ { s+=in[y][x]*cosTable[u][x]*cosTable[v][y] } }; out[v][u]=.25*alpha(u)*alpha(v)*s } }; return }
func idct(in [8][8]float64)(out [8][8]float64){ for y:=0;y<8;y++ { for x:=0;x<8;x++ { s:=0.0; for v:=0;v<8;v++ { for u:=0;u<8;u++ { s+=alpha(u)*alpha(v)*in[v][u]*cosTable[u][x]*cosTable[v][y] } }; out[y][x]=.25*s } }; return }
func clamp(v float64) uint8 { if v<0{return 0}; if v>255{return 255}; return uint8(math.Round(v)) }
func toNRGBA(src image.Image)*image.NRGBA { b:=src.Bounds(); out:=image.NewNRGBA(image.Rect(0,0,b.Dx(),b.Dy())); for y:=0;y<b.Dy();y++ { for x:=0;x<b.Dx();x++ { c:=color.NRGBAModel.Convert(src.At(b.Min.X+x,b.Min.Y+y)).(color.NRGBA); if c.A<255 { a:=float64(c.A)/255; c.R=uint8(float64(c.R)*a+255*(1-a)); c.G=uint8(float64(c.G)*a+255*(1-a)); c.B=uint8(float64(c.B)*a+255*(1-a)); c.A=255 }; out.SetNRGBA(x,y,c) } }; return out }
