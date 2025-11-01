package web

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"strings"

	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
)

const lightTemplate = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1024 1024"><rect width="1024" height="1024" fill="%s"/><path d="M385.164 677a32 32 0 0 1 12.819 2.68L606.855 771h100.958c12.226-43.838 52.45-76 100.187-76 57.438 0 104 46.562 104 104s-46.562 104-104 104c-47.736 0-87.96-32.162-100.187-75.999L598.491 827a16 16 0 0 1-6.41-1.34L380.148 733H120a8 8 0 0 1-8-8v-40a8 8 0 0 1 8-8h265.164zM808 759c-22.091 0-40 17.909-40 40s17.909 40 40 40 40-17.909 40-40-17.909-40-40-40zm0-638c57.438 0 104 46.562 104 104s-46.562 104-104 104c-47.736 0-87.96-32.162-100.187-75.999L454 253v64h71.492c9.538-16.489 27.242-27.663 47.582-27.992L574 289c30.928 0 56 25.072 56 56s-25.072 56-56 56c-20.727 0-38.825-11.261-48.508-27.999L454 373v104c0 8.837-7.163 16-16 16H120a8 8 0 0 1-8-8v-40a8 8 0 0 1 8-8h278v-64H120a8 8 0 0 1-8-8v-40a8 8 0 0 1 8-8h278v-88c0-17.673 14.327-32 32-32h277.813c12.226-43.838 52.45-76 100.187-76zm0 64c-22.091 0-40 17.909-40 40s17.909 40 40 40 40-17.909 40-40-17.909-40-40-40z" fill="#FFFFFF"/><path d="M512 529c30.928 0 56 25.072 56 56s-25.072 56-56 56c-20.727 0-38.825-11.261-48.508-27.999L120 613a8 8 0 0 1-8-8v-40a8 8 0 0 1 8-8h343.492c9.682-16.738 27.78-28 48.508-28z" fill="#FFFFFF"/></svg>`
const darkTemplate = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1024 1024"><rect width="1024" height="1024" fill="%s"/><path d="M385.164 677a32 32 0 0 1 12.819 2.68L606.855 771h100.958c12.226-43.838 52.45-76 100.187-76 57.438 0 104 46.562 104 104s-46.562 104-104 104c-47.736 0-87.96-32.162-100.187-75.999L598.491 827a16 16 0 0 1-6.41-1.34L380.148 733H120a8 8 0 0 1-8-8v-40a8 8 0 0 1 8-8h265.164zM808 759c-22.091 0-40 17.909-40 40s17.909 40 40 40 40-17.909 40-40-17.909-40-40-40zm0-638c57.438 0 104 46.562 104 104s-46.562 104-104 104c-47.736 0-87.96-32.162-100.187-75.999L454 253v64h71.492c9.538-16.489 27.242-27.663 47.582-27.992L574 289c30.928 0 56 25.072 56 56s-25.072 56-56 56c-20.727 0-38.825-11.261-48.508-27.999L454 373v104c0 8.837-7.163 16-16 16H120a8 8 0 0 1-8-8v-40a8 8 0 0 1 8-8h278v-64H120a8 8 0 0 1-8-8v-40a8 8 0 0 1 8-8h278v-88c0-17.673 14.327-32 32-32h277.813c12.226-43.838 52.45-76 100.187-76zm0 64c-22.091 0-40 17.909-40 40s17.909 40 40 40 40-17.909 40-40-17.909-40-40-40z" fill="#9CA3AF"/><path d="M512 529c30.928 0 56 25.072 56 56s-25.072 56-56 56c-20.727 0-38.825-11.261-48.508-27.999L120 613a8 8 0 0 1-8-8v-40a8 8 0 0 1 8-8h343.492c9.682-16.738 27.78-28 48.508-28z" fill="#9CA3AF"/></svg>`

// buildHubSVG returns the static hub icon SVG with the provided brand color.
func buildHubSVG(brandColor string) string {
	return fmt.Sprintf(lightTemplate, brandColor)
}

func buildHubSVGDark(brandColor string) string {
	return fmt.Sprintf(darkTemplate, brandColor)
}

// generateFaviconSVG returns the SVG favicon string.
func generateFaviconSVG(brandColor string) string {
	return buildHubSVG(brandColor)
}

func generateFaviconSVGDark(brandColor string) string {
	return buildHubSVGDark(brandColor)
}

// generateFaviconICO produces an ICO container with 16px and 32px PNG layers.
func generateFaviconICO(brandColor string) ([]byte, error) {
	return generateFaviconICOFromSVG(buildHubSVG(brandColor))
}

func generateFaviconICODark(brandColor string) ([]byte, error) {
	return generateFaviconICOFromSVG(buildHubSVGDark(brandColor))
}

func generateFaviconICOFromSVG(svg string) ([]byte, error) {
	sizes := []int{16, 32}
	type iconLayer struct {
		size int
		data []byte
	}

	layers := make([]iconLayer, 0, len(sizes))
	for _, size := range sizes {
		data, err := generateIconPNGFromSVG(svg, size)
		if err != nil {
			return nil, err
		}
		layers = append(layers, iconLayer{size: size, data: data})
	}

	buf := &bytes.Buffer{}

	buf.Write([]byte{0x00, 0x00})              // Reserved
	buf.Write([]byte{0x01, 0x00})              // Type (icon)
	buf.Write([]byte{byte(len(layers)), 0x00}) // Image count
	dataOffset := 6 + 16*len(layers)
	for _, layer := range layers {
		size := layer.size
		width := byte(size)
		height := byte(size)
		if size >= 256 {
			width = 0
			height = 0
		}

		buf.WriteByte(width)
		buf.WriteByte(height)
		buf.WriteByte(0x00)
		buf.WriteByte(0x00)
		buf.Write([]byte{0x01, 0x00})
		buf.Write([]byte{0x20, 0x00})

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
	return generateIconPNGFromSVG(buildHubSVG(brandColor), 180)
}

func generateAppleTouchIconPNGD(brandColor string) ([]byte, error) {
	return generateIconPNGFromSVG(buildHubSVGDark(brandColor), 180)
}

func generateIconPNG(brandColor string, size int) ([]byte, error) {
	return generateIconPNGFromSVG(buildHubSVG(brandColor), size)
}

func generateIconPNGDark(brandColor string, size int) ([]byte, error) {
	return generateIconPNGFromSVG(buildHubSVGDark(brandColor), size)
}

func generateIconPNGFromSVG(svg string, size int) ([]byte, error) {
	img, err := renderIconImage(svg, size)
	if err != nil {
		return nil, err
	}

	buf := &bytes.Buffer{}
	if err := png.Encode(buf, img); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func renderIconImage(svg string, size int) (*image.RGBA, error) {
	if size <= 0 {
		return nil, fmt.Errorf("invalid icon size: %d", size)
	}

	icon, err := oksvg.ReadIconStream(strings.NewReader(svg))
	if err != nil {
		return nil, err
	}

	icon.SetTarget(0, 0, float64(size), float64(size))

	img := image.NewRGBA(image.Rect(0, 0, size, size))
	scanner := rasterx.NewScannerGV(size, size, img, img.Bounds())
	raster := rasterx.NewDasher(size, size, scanner)

	icon.Draw(raster, 1.0)

	return img, nil
}
