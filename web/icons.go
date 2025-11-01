package web

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"strconv"
	"strings"

	"golang.org/x/image/vector"
)

// generateFaviconSVG returns an SVG favicon that reflects the configured brand color.
func generateFaviconSVG(brandColor string) string {
	var sb strings.Builder
	for _, band := range flowBands {
		if path := polygonToSVGPath(band); path != "" {
			fmt.Fprintf(&sb, `<path d='%s' fill='white'/>`, path)
		}
	}

	sb.WriteString(`<circle cx='50' cy='50' r='18' fill='none' stroke='white' stroke-width='6' stroke-linecap='round'/>`)
	sb.WriteString(`<circle cx='50' cy='50' r='7' fill='white' fill-opacity='0.9'/>`)

	return fmt.Sprintf(
		`<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 100 100'><circle cx='50' cy='50' r='45' fill='%s'/>%s</svg>`,
		brandColor,
		sb.String(),
	)
}

// generateFaviconICO produces an ICO container with 16px and 32px PNG layers.
func generateFaviconICO(brandColor string) ([]byte, error) {
	sizes := []int{16, 32}
	type iconLayer struct {
		size int
		data []byte
	}

	layers := make([]iconLayer, 0, len(sizes))
	for _, size := range sizes {
		data, err := generateIconPNG(brandColor, size)
		if err != nil {
			return nil, err
		}
		layers = append(layers, iconLayer{size: size, data: data})
	}

	buf := &bytes.Buffer{}

	// ICONDIR header
	buf.Write([]byte{0x00, 0x00})              // Reserved
	buf.Write([]byte{0x01, 0x00})              // Type (icon)
	buf.Write([]byte{byte(len(layers)), 0x00}) // Image count
	dataOffset := 6 + 16*len(layers)           // Initial offset after header and entries
	for _, layer := range layers {
		size := layer.size
		width := byte(size)
		height := byte(size)
		if size >= 256 {
			width = 0
			height = 0
		}

		buf.WriteByte(width)          // Width
		buf.WriteByte(height)         // Height
		buf.WriteByte(0x00)           // Color palette count (none)
		buf.WriteByte(0x00)           // Reserved
		buf.Write([]byte{0x01, 0x00}) // Color planes
		buf.Write([]byte{0x20, 0x00}) // Bits per pixel (32)

		length := uint32(len(layer.data))
		buf.Write([]byte{byte(length), byte(length >> 8), byte(length >> 16), byte(length >> 24)})
		offset := uint32(dataOffset)
		buf.Write([]byte{byte(offset), byte(offset >> 8), byte(offset >> 16), byte(offset >> 24)})

		dataOffset += len(layer.data)
	}

	for _, layer := range layers {
		if _, err := buf.Write(layer.data); err != nil {
			return nil, err
		}
	}

	return buf.Bytes(), nil
}

// generateAppleTouchIconPNG produces the 180×180 PNG used by iOS devices.
func generateAppleTouchIconPNG(brandColor string) ([]byte, error) {
	return generateIconPNG(brandColor, 180)
}

func generateIconPNG(brandColor string, size int) ([]byte, error) {
	img, err := renderIconImage(brandColor, size)
	if err != nil {
		return nil, err
	}

	buf := &bytes.Buffer{}
	if err := png.Encode(buf, img); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func renderIconImage(brandColor string, size int) (*image.NRGBA, error) {
	if size <= 0 {
		return nil, fmt.Errorf("invalid icon size: %d", size)
	}

	brandColorRGBA, err := parseHexColor(brandColor)
	if err != nil {
		return nil, err
	}

	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	scaleX := float32(size) / canonicalSize
	scaleY := float32(size) / canonicalSize

	brandSrc := image.NewUniform(brandColorRGBA)
	drawPolygon(img, brandSrc, scaleX, scaleY, backgroundCircle)

	whiteSrc := image.NewUniform(color.RGBA{255, 255, 255, 255})
	for _, band := range flowBands {
		drawPolygon(img, whiteSrc, scaleX, scaleY, band)
	}

	drawPolygon(img, whiteSrc, scaleX, scaleY, outerRingCircle)
	drawPolygon(img, brandSrc, scaleX, scaleY, innerRingCircle)

	coreSrc := image.NewUniform(color.RGBA{255, 255, 255, 230})
	drawPolygon(img, coreSrc, scaleX, scaleY, coreCircle)

	return img, nil
}

func parseHexColor(hex string) (color.RGBA, error) {
	h := strings.TrimPrefix(hex, "#")
	if len(h) != 6 {
		return color.RGBA{}, fmt.Errorf("invalid hex color: %s", hex)
	}

	r, err := strconv.ParseUint(h[0:2], 16, 8)
	if err != nil {
		return color.RGBA{}, err
	}
	g, err := strconv.ParseUint(h[2:4], 16, 8)
	if err != nil {
		return color.RGBA{}, err
	}
	b, err := strconv.ParseUint(h[4:6], 16, 8)
	if err != nil {
		return color.RGBA{}, err
	}

	return color.RGBA{uint8(r), uint8(g), uint8(b), 255}, nil
}

const (
	canonicalSize  float32 = 100.0
	circleSegments         = 256
	centerX        float32 = 50
	centerY        float32 = 50
)

type canonicalPoint struct {
	x float32
	y float32
}

var (
	backgroundCircle = buildCirclePoints(circleSegments, centerX, centerY, 45)
	outerRingCircle  = buildCirclePoints(circleSegments, centerX, centerY, 18)
	innerRingCircle  = buildCirclePoints(circleSegments, centerX, centerY, 12.5)
	coreCircle       = buildCirclePoints(circleSegments, centerX, centerY, 7)
	flowBands        = [][]canonicalPoint{
		buildFlowBand(-118, -22),
		buildFlowBand(62, 158),
	}
)

func buildCirclePoints(segments int, centerX, centerY, radius float32) []canonicalPoint {
	points := make([]canonicalPoint, segments)
	for i := 0; i < segments; i++ {
		angle := 2 * math.Pi * float64(i) / float64(segments)
		points[i] = canonicalPoint{
			x: centerX + radius*float32(math.Cos(angle)),
			y: centerY + radius*float32(math.Sin(angle)),
		}
	}
	return points
}

func drawPolygon(dst draw.Image, src image.Image, scaleX, scaleY float32, points []canonicalPoint) {
	if len(points) < 3 {
		return
	}

	r := vector.NewRasterizer(dst.Bounds().Dx(), dst.Bounds().Dy())
	r.MoveTo(points[0].x*scaleX, points[0].y*scaleY)
	for _, p := range points[1:] {
		r.LineTo(p.x*scaleX, p.y*scaleY)
	}
	r.ClosePath()
	r.Draw(dst, dst.Bounds(), src, image.Point{})
}

func buildFlowBand(startDeg, endDeg float64) []canonicalPoint {
	const (
		outerRadius = 41.0
		innerRadius = 29.0
		gapDeg      = 14.0
		steps       = 160
	)

	start := degreesToRadians(startDeg)
	end := degreesToRadians(endDeg)

	points := make([]canonicalPoint, 0, 2*steps+2)

	outerStart := start + degreesToRadians(gapDeg)
	outerEnd := end - degreesToRadians(gapDeg)
	if outerEnd <= outerStart {
		outerEnd = outerStart + 0.01
	}

	for i := 0; i <= steps; i++ {
		t := float64(i) / float64(steps)
		angle := outerStart + (outerEnd-outerStart)*t
		points = append(points, canonicalPoint{
			x: centerX + float32(outerRadius*math.Cos(angle)),
			y: centerY + float32(outerRadius*math.Sin(angle)),
		})
	}

	for i := 0; i <= steps; i++ {
		t := float64(i) / float64(steps)
		angle := outerEnd - (outerEnd-outerStart)*t
		points = append(points, canonicalPoint{
			x: centerX + float32(innerRadius*math.Cos(angle)),
			y: centerY + float32(innerRadius*math.Sin(angle)),
		})
	}

	return points
}

func degreesToRadians(deg float64) float64 {
	return deg * math.Pi / 180.0
}

func polygonToSVGPath(points []canonicalPoint) string {
	if len(points) == 0 {
		return ""
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "M%.2f %.2f", points[0].x, points[0].y)
	for _, p := range points[1:] {
		fmt.Fprintf(&sb, "L%.2f %.2f", p.x, p.y)
	}
	sb.WriteString("Z")
	return sb.String()
}
