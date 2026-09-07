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
	"sort"
)

const (
	blockSize  = 8
	maxPayload = 64
	headerSize = 8 // magic(2), version(1), length(1), crc32(4)
	tagSize    = 8
	frameSize  = headerSize + maxPayload + tagSize
	frameBits  = frameSize * 8
	maxSearchPixels = 50_000_000

	// Hamming(7,4) expands the fixed 640-bit frame to 1120 protected bits.
	eccBits    = frameBits / 4 * 7
	tileWidth  = 35
	tileHeight = 32
)

var (
	magic               = [2]byte{'P', 'S'}
	candidateBlockSizes = [...]int{8, 6, 4}
	normalizedScales    = [...]int{95, 90, 85, 80, 70, 65, 60, 55, 45, 40, 35, 30, 25}
	cosTable            [8][8]float64
	readCosTables       map[int][][]float64
)

type Options struct {
	Strength   float64
	Repetition int // Used when decoding legacy v1 watermarks.
}

func DefaultOptions() Options {
	return Options{Strength: 24, Repetition: 5}
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

func normalize(options Options) (Options, error) {
	if options.Strength == 0 {
		options.Strength = 24
	}
	if options.Repetition == 0 {
		options.Repetition = 5
	}
	if options.Strength < 4 || options.Strength > 120 {
		return options, errors.New("strength must be between 4 and 120")
	}
	if options.Repetition < 1 || options.Repetition > 31 || options.Repetition%2 == 0 {
		return options, errors.New("repetition must be an odd number between 1 and 31")
	}
	return options, nil
}

// Capacity reports the v2 payload capacity. A complete periodic tile is needed
// in both dimensions so every protected frame bit is represented at least once.
func Capacity(img image.Image, _ int) int {
	bounds := img.Bounds()
	if bounds.Dx()/blockSize < tileWidth || bounds.Dy()/blockSize < tileHeight {
		return 0
	}
	return maxPayload
}

// Embed writes a geometrically synchronized v2 watermark. The complete,
// error-corrected frame is repeated as a two-dimensional periodic tile.
func Embed(src image.Image, payload, key []byte, options Options) (*image.NRGBA, error) {
	options, err := normalize(options)
	if err != nil {
		return nil, err
	}
	if len(key) < 8 {
		return nil, errors.New("key must contain at least 8 bytes")
	}
	if len(payload) > maxPayload {
		return nil, fmt.Errorf("payload exceeds %d bytes", maxPayload)
	}
	if Capacity(src, options.Repetition) == 0 {
		return nil, fmt.Errorf("image must be at least %dx%d pixels for a v2 watermark", tileWidth*blockSize, tileHeight*blockSize)
	}

	frame := makeFrame(payload, key, 2)
	protected := hammingEncode(whiten(bytesToBits(frame), key, "pixseal-whiten-v2"))
	out := toNRGBA(src)
	bounds := out.Bounds()
	blocksWide := bounds.Dx() / blockSize
	blocksHigh := bounds.Dy() / blockSize

	for blockY := 0; blockY < blocksHigh; blockY++ {
		for blockX := 0; blockX < blocksWide; blockX++ {
			position := (blockY%tileHeight)*tileWidth + blockX%tileWidth
			embedBlock(out, point{blockX * blockSize, blockY * blockSize}, protected[position], options.Strength)
		}
	}
	return out, nil
}

// Extract first looks for the periodic v2 format across supported scales and
// block-grid offsets, then falls back to the original v1 decoder.
func Extract(src image.Image, key []byte, options Options) ([]byte, float64, error) {
	options, err := normalize(options)
	if err != nil {
		return nil, 0, err
	}
	if len(key) < 8 {
		return nil, 0, errors.New("key must contain at least 8 bytes")
	}

	if payload, confidence, err := extractV2(src, key); err == nil {
		return payload, confidence, nil
	}
	return extractLegacy(src, key, options)
}

type phaseCandidate struct {
	score int
	x     int
	y     int
}

type scaleCandidate struct {
	percent int
	score   int
}

func extractV2(src image.Image, key []byte) ([]byte, float64, error) {
	known := []byte{magic[0], magic[1], 2}
	syncBits := hammingEncode(whiten(bytesToBits(known), key, "pixseal-whiten-v2"))
	if payload, confidence, ok := searchV2(src, key, syncBits, candidateBlockSizes[:]); ok {
		return payload, confidence, nil
	}

	// First use an aligned fast path. A pure resize preserves the top-left grid
	// origin, so testing one offset avoids 63 unnecessary full-image scans for
	// every scale candidate.
	bounds := src.Bounds()
	neighborDeltas := [...]point{
		{-1, -1}, {0, -1}, {1, -1},
		{-1, 0}, {1, 0},
		{-1, 1}, {0, 1}, {1, 1},
	}
	scales := make([]scaleCandidate, 0, len(normalizedScales))
	for _, percent := range normalizedScales {
		width := int(math.Round(float64(bounds.Dx()) * 100 / float64(percent)))
		height := int(math.Round(float64(bounds.Dy()) * 100 / float64(percent)))
		if width < tileWidth*blockSize || height < tileHeight*blockSize || width*height > maxSearchPixels {
			continue
		}
		normalized := resizeBicubic(src, width, height)
		payload, confidence, score, ok := searchV2Aligned(normalized, key, syncBits)
		if ok {
			return payload, confidence, nil
		}
		scales = append(scales, scaleCandidate{percent: percent, score: score})
	}

	// Rounding can move either inverse dimension by one pixel. Try adjacent
	// dimensions only around the three nominal scales with the strongest sync
	// prefix, rather than expanding every scale into nine full candidates.
	sort.SliceStable(scales, func(i, j int) bool { return scales[i].score > scales[j].score })
	if len(scales) > 3 {
		scales = scales[:3]
	}
	for _, candidate := range scales {
		baseWidth := int(math.Round(float64(bounds.Dx()) * 100 / float64(candidate.percent)))
		baseHeight := int(math.Round(float64(bounds.Dy()) * 100 / float64(candidate.percent)))
		for _, delta := range neighborDeltas {
			width := baseWidth + delta.x
			height := baseHeight + delta.y
			if width < tileWidth*blockSize || height < tileHeight*blockSize || width*height > maxSearchPixels {
				continue
			}
			normalized := resizeBicubic(src, width, height)
			if payload, confidence, _, ok := searchV2Aligned(normalized, key, syncBits); ok {
				return payload, confidence, nil
			}
		}
	}
	return nil, 0, errors.New("v2 watermark not found")
}

func searchV2Aligned(src image.Image, key, syncBits []byte) ([]byte, float64, int, bool) {
	grid, ok := aggregateGrid(src, blockSize, 0, 0)
	if !ok {
		return nil, 0, 0, false
	}
	phases := strongestPhases(grid, syncBits, 4)
	score := 0
	if len(phases) > 0 {
		score = phases[0].score
	}
	payload, confidence, found := decodePhases(grid, key, syncBits, phases)
	return payload, confidence, score, found
}

func searchV2(src image.Image, key, syncBits []byte, sizes []int) ([]byte, float64, bool) {
	for _, size := range sizes {
		bounds := src.Bounds()
		if bounds.Dx() < tileWidth*size || bounds.Dy() < tileHeight*size {
			continue
		}
		for offsetY := 0; offsetY < size; offsetY++ {
			for offsetX := 0; offsetX < size; offsetX++ {
				grid, ok := aggregateGrid(src, size, offsetX, offsetY)
				if !ok {
					continue
				}
				if payload, confidence, ok := decodeGrid(grid, key, syncBits); ok {
					return payload, confidence, true
				}
			}
		}
	}
	return nil, 0, false
}

func decodeGrid(grid []float64, key, syncBits []byte) ([]byte, float64, bool) {
	return decodePhases(grid, key, syncBits, strongestPhases(grid, syncBits, 4))
}

func decodePhases(grid []float64, key, syncBits []byte, phases []phaseCandidate) ([]byte, float64, bool) {
	for _, phase := range phases {
		// Require at least 75% agreement with magic and version.
		if phase.score*4 < len(syncBits)*3 {
			continue
		}
		coded, margin := readPeriodicFrame(grid, phase.x, phase.y)
		decoded := hammingDecode(coded)
		raw := bitsToBytes(whiten(decoded, key, "pixseal-whiten-v2"))
		if payload, err := parseFrame(raw, key, 2); err == nil {
			return payload, margin, true
		}
	}
	return nil, 0, false
}

func aggregateGrid(src image.Image, size, offsetX, offsetY int) ([]float64, bool) {
	bounds := src.Bounds()
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

func strongestPhases(grid []float64, syncBits []byte, count int) []phaseCandidate {
	candidates := make([]phaseCandidate, 0, eccBits)
	for phaseY := 0; phaseY < tileHeight; phaseY++ {
		for phaseX := 0; phaseX < tileWidth; phaseX++ {
			score := 0
			for logicalPosition, expected := range syncBits {
				logicalX := logicalPosition % tileWidth
				logicalY := logicalPosition / tileWidth
				observedX := positiveMod(logicalX-phaseX, tileWidth)
				observedY := positiveMod(logicalY-phaseY, tileHeight)
				observed := byte(0)
				if grid[observedY*tileWidth+observedX] >= 0 {
					observed = 1
				}
				if observed == expected {
					score++
				}
			}
			candidates = append(candidates, phaseCandidate{score: score, x: phaseX, y: phaseY})
		}
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].score > candidates[j].score })
	if len(candidates) > count {
		candidates = candidates[:count]
	}
	return candidates
}

func readPeriodicFrame(grid []float64, phaseX, phaseY int) ([]byte, float64) {
	coded := make([]byte, eccBits)
	margin := 0.0
	for logicalPosition := 0; logicalPosition < eccBits; logicalPosition++ {
		logicalX := logicalPosition % tileWidth
		logicalY := logicalPosition / tileWidth
		observedX := positiveMod(logicalX-phaseX, tileWidth)
		observedY := positiveMod(logicalY-phaseY, tileHeight)
		value := grid[observedY*tileWidth+observedX]
		if value >= 0 {
			coded[logicalPosition] = 1
		}
		margin += math.Abs(value)
	}
	return coded, margin / float64(eccBits)
}

func positiveMod(value, modulus int) int {
	value %= modulus
	if value < 0 {
		value += modulus
	}
	return value
}

func makeFrame(payload, key []byte, version byte) []byte {
	frame := make([]byte, frameSize)
	frame[0], frame[1], frame[2], frame[3] = magic[0], magic[1], version, byte(len(payload))
	binary.BigEndian.PutUint32(frame[4:8], crc32.ChecksumIEEE(payload))
	copy(frame[headerSize:], payload)
	mac := hmac.New(sha256.New, key)
	mac.Write(frame[:headerSize+len(payload)])
	copy(frame[headerSize+len(payload):], mac.Sum(nil)[:tagSize])
	return frame
}

func parseFrame(frame, key []byte, version byte) ([]byte, error) {
	if len(frame) < frameSize || frame[0] != magic[0] || frame[1] != magic[1] || frame[2] != version {
		return nil, errors.New("watermark header mismatch")
	}
	payloadLength := int(frame[3])
	if payloadLength > maxPayload {
		return nil, errors.New("invalid watermark length")
	}
	payload := frame[headerSize : headerSize+payloadLength]
	if crc32.ChecksumIEEE(payload) != binary.BigEndian.Uint32(frame[4:8]) {
		return nil, errors.New("watermark damaged: CRC mismatch")
	}
	mac := hmac.New(sha256.New, key)
	mac.Write(frame[:headerSize+payloadLength])
	if !hmac.Equal(frame[headerSize+payloadLength:headerSize+payloadLength+tagSize], mac.Sum(nil)[:tagSize]) {
		return nil, errors.New("watermark authentication failed")
	}
	return append([]byte(nil), payload...), nil
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

// readBlockSized evaluates only the two DCT coefficients used by PixSeal. It
// supports the 8, 6 and 4 pixel grids produced by 100%, 75% and 50% scaling.
func readBlockSized(img image.Image, position point, size int) float64 {
	table := readCosTables[size]
	coefficient23 := 0.0
	coefficient32 := 0.0
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			r, g, b, _ := img.At(position.x+x, position.y+y).RGBA()
			luminance := .299*float64(r>>8) + .587*float64(g>>8) + .114*float64(b>>8) - 128
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

func toNRGBA(src image.Image) *image.NRGBA {
	bounds := src.Bounds()
	output := image.NewNRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	for y := 0; y < bounds.Dy(); y++ {
		for x := 0; x < bounds.Dx(); x++ {
			pixel := color.NRGBAModel.Convert(src.At(bounds.Min.X+x, bounds.Min.Y+y)).(color.NRGBA)
			if pixel.A < 255 {
				alpha := float64(pixel.A) / 255
				pixel.R = uint8(float64(pixel.R)*alpha + 255*(1-alpha))
				pixel.G = uint8(float64(pixel.G)*alpha + 255*(1-alpha))
				pixel.B = uint8(float64(pixel.B)*alpha + 255*(1-alpha))
				pixel.A = 255
			}
			output.SetNRGBA(x, y, pixel)
		}
	}
	return output
}

func resizeBicubic(src image.Image, width, height int) *image.NRGBA {
	bounds := src.Bounds()
	output := image.NewNRGBA(image.Rect(0, 0, width, height))
	scaleX := float64(bounds.Dx()) / float64(width)
	scaleY := float64(bounds.Dy()) / float64(height)

	for y := 0; y < height; y++ {
		sourceY := (float64(y)+.5)*scaleY - .5
		baseY := int(math.Floor(sourceY))
		for x := 0; x < width; x++ {
			sourceX := (float64(x)+.5)*scaleX - .5
			baseX := int(math.Floor(sourceX))
			red, green, blue, weightSum := 0.0, 0.0, 0.0, 0.0

			for sampleY := baseY - 1; sampleY <= baseY+2; sampleY++ {
				weightY := cubicWeight(sourceY - float64(sampleY))
				pixelY := clampCoordinate(sampleY, bounds.Dy()) + bounds.Min.Y
				for sampleX := baseX - 1; sampleX <= baseX+2; sampleX++ {
					weight := weightY * cubicWeight(sourceX-float64(sampleX))
					pixelX := clampCoordinate(sampleX, bounds.Dx()) + bounds.Min.X
					r, g, b, _ := src.At(pixelX, pixelY).RGBA()
					red += float64(r>>8) * weight
					green += float64(g>>8) * weight
					blue += float64(b>>8) * weight
					weightSum += weight
				}
			}
			output.SetNRGBA(x, y, color.NRGBA{
				R: clamp(red / weightSum),
				G: clamp(green / weightSum),
				B: clamp(blue / weightSum),
				A: 255,
			})
		}
	}
	return output
}

func cubicWeight(value float64) float64 {
	value = math.Abs(value)
	if value <= 1 {
		return 1.5*value*value*value - 2.5*value*value + 1
	}
	if value < 2 {
		return -.5*value*value*value + 2.5*value*value - 4*value + 2
	}
	return 0
}

func clampCoordinate(value, length int) int {
	if value < 0 {
		return 0
	}
	if value >= length {
		return length - 1
	}
	return value
}
