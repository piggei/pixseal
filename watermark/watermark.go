package watermark

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/color"
	"math"
)

const (
	blockSize        = 8
	maxPayload       = 64
	headerSize       = 8 // magic(2), version(1), length(1), crc32(4)
	tagSize          = 8
	maxFrameSize     = headerSize + maxPayload + tagSize
	maxFrameBits     = maxFrameSize * 8
	maxSearchPixels  = 50_000_000
	maxWorkingPixels = 250_000_000

	// Hamming(7,4) expands the 80-byte capacity-profile frame to 1120 protected bits.
	eccBits    = maxFrameBits / 4 * 7
	tileWidth  = 35
	tileHeight = 32
)

func exceedsPixelLimit(width, height, limit int) bool {
	if width <= 0 || height <= 0 || limit < 0 {
		return false
	}
	return int64(width) > int64(limit)/int64(height)
}

var (
	magic               = [2]byte{'P', 'S'}
	candidateBlockSizes = [...]int{8, 6, 4}
	normalizedScales    = [...]int{95, 90, 85, 80, 70, 65, 60, 55, 45, 40, 35, 30, 25}
	cosTable            [8][8]float64
	readCosTables       map[int][][]float64
)

type Options struct {
	Strength float64
	Profile  Profile // v3 embed profile; empty is treated as auto.
}

func DefaultOptions() Options {
	return Options{Strength: 24, Profile: ProfileAuto}
}

func init() {
	for u := 0; u < blockSize; u++ {
		for x := 0; x < blockSize; x++ {
			cosTable[u][x] = math.Cos(float64(2*x+1) * float64(u) * math.Pi / 16)
		}
	}
	readCosTables = make(map[int][][]float64, len(candidateBlockSizes))
	for _, size := range candidateBlockSizes {
		table := make([][]float64, 4)
		for frequency := 0; frequency < 4; frequency++ {
			table[frequency] = make([]float64, size)
			for position := 0; position < size; position++ {
				table[frequency][position] = math.Cos(
					float64(2*position+1) * float64(frequency) * math.Pi / float64(2*size),
				)
			}
		}
		readCosTables[size] = table
	}
}

func normalizeEmbedOptions(options Options) (Options, error) {
	if options.Strength == 0 {
		options.Strength = 24
	}
	if math.IsNaN(options.Strength) || math.IsInf(options.Strength, 0) {
		return options, errors.New("strength must be a finite number")
	}
	if options.Strength < 4 || options.Strength > 120 {
		return options, errors.New("strength must be between 4 and 120")
	}
	return options, nil
}

func validateWorkingImageSize(img image.Image) error {
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	if width <= 0 || height <= 0 {
		return fmt.Errorf("invalid image dimensions %dx%d", width, height)
	}
	if int64(width) > (1<<63-1)/int64(height) {
		return errors.New("image dimensions overflow the PixSeal pixel-count calculation")
	}
	pixels := int64(width) * int64(height)
	if pixels > maxWorkingPixels {
		return fmt.Errorf("image has %d pixels; PixSeal working-image limit is %d pixels", pixels, maxWorkingPixels)
	}
	maxInt := int64(^uint(0) >> 1)
	if pixels > maxInt/3 {
		return errors.New("image is too large for PixSeal pixel-plane allocation on this architecture")
	}
	return nil
}

// Capacity reports the maximum v3 payload capacity. A complete periodic tile is
// needed in both dimensions so every protected capacity-profile bit is present.
func Capacity(img image.Image) int {
	bounds := img.Bounds()
	if bounds.Dx()/blockSize < tileWidth || bounds.Dy()/blockSize < tileHeight {
		return 0
	}
	return maxPayload
}

// Embed hides an authenticated v3 payload. Auto selects the most robust profile
// that can hold the payload. Use EmbedWithInfo when the resolved profile is needed.
func Embed(src image.Image, payload, key []byte, options Options) (*image.NRGBA, error) {
	out, _, err := EmbedWithInfo(src, payload, key, options)
	return out, err
}

// Extract recovers an authenticated v3 payload and returns its confidence margin.
// Use ExtractWithInfo to inspect the recovered adaptive profile.
func Extract(src image.Image, key []byte) ([]byte, float64, error) {
	payload, info, err := ExtractWithInfo(src, key)
	if err != nil {
		return nil, 0, err
	}
	return payload, info.Confidence, nil
}

func aggregateGrid(src *pixelPlane, size, offsetX, offsetY int) ([]float64, bool) {
	bounds := src.bounds
	blocksWide := (bounds.Dx() - offsetX) / size
	blocksHigh := (bounds.Dy() - offsetY) / size
	if blocksWide < tileWidth || blocksHigh < tileHeight {
		return nil, false
	}

	grid := make([]float64, eccBits)
	for blockY := 0; blockY < blocksHigh; blockY++ {
		for blockX := 0; blockX < blocksWide; blockX++ {
			position := (blockY%tileHeight)*tileWidth + blockX%tileWidth
			grid[position] += readBlockSized(src, point{
				x: bounds.Min.X + offsetX + blockX*size,
				y: bounds.Min.Y + offsetY + blockY*size,
			}, size)
		}
	}
	return grid, true
}

func positiveMod(value, modulus int) int {
	value %= modulus
	if value < 0 {
		value += modulus
	}
	return value
}

func bytesToBits(input []byte) []byte {
	bits := make([]byte, len(input)*8)
	for i, value := range input {
		for bit := 0; bit < 8; bit++ {
			bits[i*8+bit] = (value >> uint(7-bit)) & 1
		}
	}
	return bits
}

func bitsToBytes(bits []byte) []byte {
	output := make([]byte, len(bits)/8)
	for i := range output {
		for bit := 0; bit < 8; bit++ {
			output[i] |= bits[i*8+bit] << uint(7-bit)
		}
	}
	return output
}

func whiten(bits, key []byte, label string) []byte {
	output := append([]byte(nil), bits...)
	counter := uint64(0)
	position := 0
	for position < len(output) {
		hash := sha256.New()
		hash.Write([]byte(label))
		hash.Write(key)
		var encodedCounter [8]byte
		binary.BigEndian.PutUint64(encodedCounter[:], counter)
		hash.Write(encodedCounter[:])
		for _, value := range hash.Sum(nil) {
			for bit := 0; bit < 8 && position < len(output); bit++ {
				output[position] ^= (value >> uint(7-bit)) & 1
				position++
			}
		}
		counter++
	}
	return output
}

func hammingEncode(input []byte) []byte {
	output := make([]byte, len(input)/4*7)
	for source, destination := 0, 0; source < len(input); source, destination = source+4, destination+7 {
		d1, d2, d3, d4 := input[source], input[source+1], input[source+2], input[source+3]
		output[destination] = d1 ^ d2 ^ d4
		output[destination+1] = d1 ^ d3 ^ d4
		output[destination+2] = d1
		output[destination+3] = d2 ^ d3 ^ d4
		output[destination+4] = d2
		output[destination+5] = d3
		output[destination+6] = d4
	}
	return output
}

func hammingDecode(input []byte) []byte {
	output := make([]byte, len(input)/7*4)
	for source, destination := 0, 0; source < len(input); source, destination = source+7, destination+4 {
		word := [7]byte{}
		copy(word[:], input[source:source+7])
		s1 := word[0] ^ word[2] ^ word[4] ^ word[6]
		s2 := word[1] ^ word[2] ^ word[5] ^ word[6]
		s4 := word[3] ^ word[4] ^ word[5] ^ word[6]
		errorPosition := int(s1 + 2*s2 + 4*s4)
		if errorPosition > 0 {
			word[errorPosition-1] ^= 1
		}
		output[destination] = word[2]
		output[destination+1] = word[4]
		output[destination+2] = word[5]
		output[destination+3] = word[6]
	}
	return output
}

type point struct {
	x int
	y int
}

func embedBlock(img *image.NRGBA, position point, bit byte, strength float64) {
	var luminance, cb, cr [8][8]float64
	for y := 0; y < blockSize; y++ {
		for x := 0; x < blockSize; x++ {
			r, g, b, _ := img.At(position.x+x, position.y+y).RGBA()
			rf, gf, bf := float64(r>>8), float64(g>>8), float64(b>>8)
			luminance[y][x] = .299*rf + .587*gf + .114*bf - 128
			cb[y][x] = -.168736*rf - .331264*gf + .5*bf
			cr[y][x] = .5*rf - .418688*gf - .081312*bf
		}
	}

	coefficients := dct(luminance)
	a := math.Abs(coefficients[2][3])
	b := math.Abs(coefficients[3][2])
	if bit == 1 {
		if a-b < strength {
			coefficients[2][3] = math.Copysign(b+strength, coefficients[2][3])
		}
	} else if b-a < strength {
		coefficients[3][2] = math.Copysign(a+strength, coefficients[3][2])
	}

	luminance = idct(coefficients)
	for y := 0; y < blockSize; y++ {
		for x := 0; x < blockSize; x++ {
			yValue := luminance[y][x] + 128
			r := yValue + 1.402*cr[y][x]
			g := yValue - .344136*cb[y][x] - .714136*cr[y][x]
			b := yValue + 1.772*cb[y][x]
			img.SetNRGBA(position.x+x, position.y+y, color.NRGBA{clamp(r), clamp(g), clamp(b), 255})
		}
	}
}

type pixelPlane struct {
	bounds image.Rectangle
	rgb    []uint8
}

// newPixelPlane converts each source pixel once into compact 8-bit RGB storage.
// Offset searches can then avoid repeatedly traversing image.Image and invoking
// color-model conversion for every candidate grid.
func newPixelPlane(img image.Image) *pixelPlane {
	bounds := img.Bounds()
	pixelCount := bounds.Dx() * bounds.Dy()
	plane := &pixelPlane{
		bounds: bounds,
		rgb:    make([]uint8, pixelCount*3),
	}
	for y := 0; y < bounds.Dy(); y++ {
		for x := 0; x < bounds.Dx(); x++ {
			r, g, b, _ := img.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
			index := (y*bounds.Dx() + x) * 3
			plane.rgb[index], plane.rgb[index+1], plane.rgb[index+2] = uint8(r>>8), uint8(g>>8), uint8(b>>8)
		}
	}
	return plane
}

// readBlockSized evaluates only the two DCT coefficients used by PixSeal. It
// supports the 8, 6 and 4 pixel grids produced by 100%, 75% and 50% scaling.
func readBlockSized(plane *pixelPlane, position point, size int) float64 {
	table := readCosTables[size]
	coefficient23 := 0.0
	coefficient32 := 0.0
	stride := plane.bounds.Dx()
	startX := position.x - plane.bounds.Min.X
	startY := position.y - plane.bounds.Min.Y
	for y := 0; y < size; y++ {
		row := (startY + y) * stride
		for x := 0; x < size; x++ {
			index := (row + startX + x) * 3
			luminance := .299*float64(plane.rgb[index]) + .587*float64(plane.rgb[index+1]) + .114*float64(plane.rgb[index+2]) - 128
			coefficient23 += luminance * table[3][x] * table[2][y]
			coefficient32 += luminance * table[2][x] * table[3][y]
		}
	}
	return math.Abs(coefficient23) - math.Abs(coefficient32)
}

func alpha(frequency int) float64 {
	if frequency == 0 {
		return 1 / math.Sqrt2
	}
	return 1
}

func dct(input [8][8]float64) (output [8][8]float64) {
	for v := 0; v < blockSize; v++ {
		for u := 0; u < blockSize; u++ {
			sum := 0.0
			for y := 0; y < blockSize; y++ {
				for x := 0; x < blockSize; x++ {
					sum += input[y][x] * cosTable[u][x] * cosTable[v][y]
				}
			}
			output[v][u] = .25 * alpha(u) * alpha(v) * sum
		}
	}
	return output
}

func idct(input [8][8]float64) (output [8][8]float64) {
	for y := 0; y < blockSize; y++ {
		for x := 0; x < blockSize; x++ {
			sum := 0.0
			for v := 0; v < blockSize; v++ {
				for u := 0; u < blockSize; u++ {
					sum += alpha(u) * alpha(v) * input[v][u] * cosTable[u][x] * cosTable[v][y]
				}
			}
			output[y][x] = .25 * sum
		}
	}
	return output
}

func clamp(value float64) uint8 {
	if value < 0 {
		return 0
	}
	if value > 255 {
		return 255
	}
	return uint8(math.Round(value))
}

func flattenedNRGBA(value color.Color) color.NRGBA {
	pixel := color.NRGBAModel.Convert(value).(color.NRGBA)
	if pixel.A < 255 {
		alpha := float64(pixel.A) / 255
		pixel.R = uint8(float64(pixel.R)*alpha + 255*(1-alpha))
		pixel.G = uint8(float64(pixel.G)*alpha + 255*(1-alpha))
		pixel.B = uint8(float64(pixel.B)*alpha + 255*(1-alpha))
		pixel.A = 255
	}
	return pixel
}

func toNRGBA(src image.Image) *image.NRGBA {
	bounds := src.Bounds()
	output := image.NewNRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	for y := 0; y < bounds.Dy(); y++ {
		for x := 0; x < bounds.Dx(); x++ {
			output.SetNRGBA(x, y, flattenedNRGBA(src.At(bounds.Min.X+x, bounds.Min.Y+y)))
		}
	}
	return output
}

func resizePixelPlaneBilinear(src *pixelPlane, width, height int) *pixelPlane {
	sourceWidth := src.bounds.Dx()
	sourceHeight := src.bounds.Dy()
	output := &pixelPlane{
		bounds: image.Rect(0, 0, width, height),
		rgb:    make([]uint8, width*height*3),
	}
	scaleX := float64(sourceWidth) / float64(width)
	scaleY := float64(sourceHeight) / float64(height)

	for y := 0; y < height; y++ {
		sourceY := (float64(y)+.5)*scaleY - .5
		y0 := int(math.Floor(sourceY))
		fy := sourceY - float64(y0)
		if y0 < 0 {
			y0, fy = 0, 0
		}
		y1 := y0 + 1
		if y1 >= sourceHeight {
			y1 = sourceHeight - 1
		}
		for x := 0; x < width; x++ {
			sourceX := (float64(x)+.5)*scaleX - .5
			x0 := int(math.Floor(sourceX))
			fx := sourceX - float64(x0)
			if x0 < 0 {
				x0, fx = 0, 0
			}
			x1 := x0 + 1
			if x1 >= sourceWidth {
				x1 = sourceWidth - 1
			}
			row0 := y0 * sourceWidth
			row1 := y1 * sourceWidth
			outIndex := (y*width + x) * 3
			for channel := 0; channel < 3; channel++ {
				c00 := float64(src.rgb[(row0+x0)*3+channel])
				c10 := float64(src.rgb[(row0+x1)*3+channel])
				c01 := float64(src.rgb[(row1+x0)*3+channel])
				c11 := float64(src.rgb[(row1+x1)*3+channel])
				top := c00*(1-fx) + c10*fx
				bottom := c01*(1-fx) + c11*fx
				output.rgb[outIndex+channel] = clamp(top*(1-fy) + bottom*fy)
			}
		}
	}
	return output
}
