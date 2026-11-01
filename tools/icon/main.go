package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	f, err := os.Open("assets/icon.png")
	if err != nil {
		return err
	}
	source, err := png.Decode(f)
	f.Close()
	if err != nil {
		return err
	}
	if source.Bounds().Dx() != source.Bounds().Dy() {
		return fmt.Errorf("icon source must be square")
	}
	sizes := []int{16, 20, 24, 32, 40, 48, 64, 128, 256}
	images := make([][]byte, len(sizes))
	for i, size := range sizes {
		var data bytes.Buffer
		if err := png.Encode(&data, downsample(source, size)); err != nil {
			return err
		}
		images[i] = data.Bytes()
	}
	var ico bytes.Buffer
	for _, value := range []uint16{0, 1, uint16(len(sizes))} {
		binary.Write(&ico, binary.LittleEndian, value)
	}
	offset := uint32(6 + 16*len(sizes))
	for i, size := range sizes {
		ico.Write([]byte{byte(size), byte(size), 0, 0})
		binary.Write(&ico, binary.LittleEndian, uint16(1))
		binary.Write(&ico, binary.LittleEndian, uint16(32))
		binary.Write(&ico, binary.LittleEndian, uint32(len(images[i])))
		binary.Write(&ico, binary.LittleEndian, offset)
		offset += uint32(len(images[i]))
	}
	for _, data := range images {
		ico.Write(data)
	}
	return os.WriteFile("assets/icon.ico", ico.Bytes(), 0644)
}

func downsample(source image.Image, size int) *image.RGBA {
	result := image.NewRGBA(image.Rect(0, 0, size, size))
	bounds := source.Bounds()
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			left, right := x*bounds.Dx()/size, (x+1)*bounds.Dx()/size
			top, bottom := y*bounds.Dy()/size, (y+1)*bounds.Dy()/size
			var r, g, b, a, count uint64
			for sy := top; sy < max(top+1, bottom); sy++ {
				for sx := left; sx < max(left+1, right); sx++ {
					pr, pg, pb, pa := source.At(bounds.Min.X+sx, bounds.Min.Y+sy).RGBA()
					r += uint64(pr)
					g += uint64(pg)
					b += uint64(pb)
					a += uint64(pa)
					count++
				}
			}
			result.SetRGBA(x, y, color.RGBA{uint8(r / count >> 8), uint8(g / count >> 8), uint8(b / count >> 8), uint8(a / count >> 8)})
		}
	}
	return result
}
