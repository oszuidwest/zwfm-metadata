package web

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/png"
	"strings"

	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
)

const hubTemplate = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1024 1024"><rect width="1024" height="1024" fill="%[1]s"/><path d="M385.164 677a32 32 0 0 1 12.819 2.68L606.855 771h100.958c12.226-43.838 52.45-76 100.187-76 57.438 0 104 46.562 104 104s-46.562 104-104 104c-47.736 0-87.96-32.162-100.187-75.999L598.491 827a16 16 0 0 1-6.41-1.34L380.148 733H120a8 8 0 0 1-8-8v-40a8 8 0 0 1 8-8h265.164zM808 759c-22.091 0-40 17.909-40 40s17.909 40 40 40 40-17.909 40-40-17.909-40-40-40zm0-638c57.438 0 104 46.562 104 104s-46.562 104-104 104c-47.736 0-87.96-32.162-100.187-75.999L454 253v64h71.492c9.538-16.489 27.242-27.663 47.582-27.992L574 289c30.928 0 56 25.072 56 56s-25.072 56-56 56c-20.727 0-38.825-11.261-48.508-27.999L454 373v104c0 8.837-7.163 16-16 16H120a8 8 0 0 1-8-8v-40a8 8 0 0 1 8-8h278v-64H120a8 8 0 0 1-8-8v-40a8 8 0 0 1 8-8h278v-88c0-17.673 14.327-32 32-32h277.813c12.226-43.838 52.45-76 100.187-76zm0 64c-22.091 0-40 17.909-40 40s17.909 40 40 40 40-17.909 40-40-17.909-40-40-40z" fill="%[2]s"/><path d="M512 529c30.928 0 56 25.072 56 56s-25.072 56-56 56c-20.727 0-38.825-11.261-48.508-27.999L120 613a8 8 0 0 1-8-8v-40a8 8 0 0 1 8-8h343.492c9.682-16.738 27.78-28 48.508-28z" fill="%[2]s"/></svg>`

type asset struct {
	data        []byte
	contentType string
}

// iconAssets renders the hub icon in a light and a dark variant as SVG, ICO
// (16px and 32px layers) and the 180px PNG used by iOS, keyed by URL path.
func iconAssets(brandColor string) (map[string]asset, error) {
	variants := []struct{ suffix, glyphColor string }{
		{suffix: "", glyphColor: "#FFFFFF"},
		{suffix: "-dark", glyphColor: "#9CA3AF"},
	}

	assets := make(map[string]asset, 3*len(variants))
	for _, v := range variants {
		svg := fmt.Sprintf(hubTemplate, brandColor, v.glyphColor)

		ico, err := generateFaviconICO(svg)
		if err != nil {
			return nil, fmt.Errorf("generate favicon%s.ico: %w", v.suffix, err)
		}
		touchIcon, err := generateIconPNG(svg, 180)
		if err != nil {
			return nil, fmt.Errorf("generate apple-touch-icon%s.png: %w", v.suffix, err)
		}

		assets["/icon"+v.suffix+".svg"] = asset{data: []byte(svg), contentType: "image/svg+xml"}
		assets["/favicon"+v.suffix+".ico"] = asset{data: ico, contentType: "image/x-icon"}
		assets["/apple-touch-icon"+v.suffix+".png"] = asset{data: touchIcon, contentType: "image/png"}
	}

	return assets, nil
}

type icoDirEntry struct {
	Width, Height, Colors, Reserved uint8
	Planes, BitCount                uint16
	Size, Offset                    uint32
}

func generateFaviconICO(svg string) ([]byte, error) {
	sizes := []uint8{16, 32}

	layers := make([][]byte, 0, len(sizes))
	for _, size := range sizes {
		data, err := generateIconPNG(svg, int(size))
		if err != nil {
			return nil, err
		}
		layers = append(layers, data)
	}

	buf := &bytes.Buffer{}
	// ICONDIR: reserved, type 1 (icon), image count.
	if err := binary.Write(buf, binary.LittleEndian, []uint16{0, 1, 2}); err != nil {
		return nil, fmt.Errorf("write ico header: %w", err)
	}

	offset := 6 + 16*len(layers)
	for i, data := range layers {
		if err := binary.Write(buf, binary.LittleEndian, icoDirEntry{
			Width:    sizes[i],
			Height:   sizes[i],
			Planes:   1,
			BitCount: 32,
			Size:     uint32(len(data)), //nolint:gosec // PNG layers are a few KB
			Offset:   uint32(offset),    //nolint:gosec // header plus a few KB
		}); err != nil {
			return nil, fmt.Errorf("write ico directory: %w", err)
		}
		offset += len(data)
	}

	for _, data := range layers {
		buf.Write(data)
	}

	return buf.Bytes(), nil
}

func generateIconPNG(svg string, size int) ([]byte, error) {
	icon, err := oksvg.ReadIconStream(strings.NewReader(svg))
	if err != nil {
		return nil, err
	}

	icon.SetTarget(0, 0, float64(size), float64(size))

	img := image.NewRGBA(image.Rect(0, 0, size, size))
	scanner := rasterx.NewScannerGV(size, size, img, img.Bounds())
	icon.Draw(rasterx.NewDasher(size, size, scanner), 1.0)

	buf := &bytes.Buffer{}
	if err := png.Encode(buf, img); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
