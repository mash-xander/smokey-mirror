package main

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
)

func main() {
	const size = 256
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			fx, fy := float64(x), float64(y)
			if rounded(fx, fy, 8, 8, 248, 248, 52) {
				img.SetNRGBA(x, y, color.NRGBA{29, 34, 51, 255})
			}
			if rounded(fx, fy, 58, 54, 117, 202, 29) {
				img.SetNRGBA(x, y, color.NRGBA{173, 163, 255, 255})
			}
			if rounded(fx, fy, 134, 54, 193, 202, 29) {
				img.SetNRGBA(x, y, color.NRGBA{221, 217, 255, 255})
			}
			if math.Abs(fx-199)+math.Abs(fy-56) < 17 {
				img.SetNRGBA(x, y, color.NRGBA{249, 250, 253, 255})
			}
		}
	}
	var pngData bytes.Buffer
	if err := png.Encode(&pngData, img); err != nil {
		panic(err)
	}
	var ico bytes.Buffer
	for _, value := range []uint16{0, 1, 1} {
		binary.Write(&ico, binary.LittleEndian, value)
	}
	ico.Write([]byte{0, 0, 0, 0})
	binary.Write(&ico, binary.LittleEndian, uint16(1))
	binary.Write(&ico, binary.LittleEndian, uint16(32))
	binary.Write(&ico, binary.LittleEndian, uint32(pngData.Len()))
	binary.Write(&ico, binary.LittleEndian, uint32(22))
	ico.Write(pngData.Bytes())
	if err := os.WriteFile("assets/icon.ico", ico.Bytes(), 0644); err != nil {
		panic(err)
	}
	if err := os.WriteFile("assets/icon.png", pngData.Bytes(), 0644); err != nil {
		panic(err)
	}
}

func rounded(x, y, left, top, right, bottom, r float64) bool {
	if x < left || x > right || y < top || y > bottom {
		return false
	}
	cx := math.Max(left+r, math.Min(x, right-r))
	cy := math.Max(top+r, math.Min(y, bottom-r))
	return (x-cx)*(x-cx)+(y-cy)*(y-cy) <= r*r
}
