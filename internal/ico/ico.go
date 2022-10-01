// Package ico implements a minimal ICO image decoder
//
// References:
// - http://www.digicamsoft.com/bmp/bmp.html
// - https://en.wikipedia.org/wiki/ICO_(file_format)
//
// Note:
// - DecodeConfig is not implemented
package ico

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/png"
	"io"

	"golang.org/x/image/bmp"
)

func init() {
	image.RegisterFormat("ico", icoHeader, Decode, DecodeConfig)
}

const (
	bmpFileHeaderSize uint32 = 14
	bmpInfoHeaderSize uint32 = 40
	icoHeader                = "\x00\x00\x01\x00"
	iconHeaderSize    uint32 = 6
	iconEntrieSize    uint32 = 16
)

func Decode(r io.Reader) (image.Image, error) {
	iconHeader := IconHeader{}
	if err := binary.Read(r, binary.LittleEndian, &iconHeader); err != nil {
		return nil, fmt.Errorf("could not read file header: %s", err)
	}

	if iconHeader.Reserved != 0 {
		return nil, fmt.Errorf("invalid signature")
	}

	// ico type we support only the icon (.ICO): value 1
	if iconHeader.Type != 1 {
		return nil, fmt.Errorf("invalid signature")
	}

	// entry represents the image entry with the highest resolution
	entry := IconDirectoryEntry{}
	for i := 0; i < int(iconHeader.Count); i++ {
		e := IconDirectoryEntry{}
		if err := binary.Read(r, binary.LittleEndian, &e); err != nil {
			return nil, fmt.Errorf("could not read icon directory entry signature: %s", err)
		}
		if e.Width > entry.Width {
			entry = e
		}
	}

	// discard data until the ImageOffset is reached
	discardBytes := entry.ImageOffset - iconHeaderSize - iconEntrieSize*uint32(iconHeader.Count)
	if _, err := io.CopyN(io.Discard, r, int64(discardBytes)); err != nil {
		return nil, fmt.Errorf("could not discard file data: %s", err)
	}

	// read all image data so we can try if it is a PNG first
	// since PNG are stored directly
	data := make([]byte, int64(entry.BytesInRes))
	if err := binary.Read(r, binary.LittleEndian, data); err != nil {
		return nil, fmt.Errorf("could not read image data %s", err)
	}

	// try PNG decoding
	img, err := png.Decode(bytes.NewReader(data))
	if err == nil {
		return img, nil
	}

	buf := bytes.NewBuffer(data)
	// if the image is stored in BMP format, the opening BitmapFileHeader
	// structure is excluded from the data and we need to add it manually before
	// decode as BMP. The first chunk represents the BitmapInfoHeader and we
	// need to read it to adjust the height of the image since it is stored as
	// twice the height declared in the image directory
	bmpInfoHeader := BitmapInfoHeader{}
	if err := binary.Read(buf, binary.LittleEndian, &bmpInfoHeader); err != nil {
		return nil, fmt.Errorf("could not read the BitmapInfoHeader")
	}
	bmpInfoHeader.Height = bmpInfoHeader.Height / 2

	// create the BMP file header
	bmpFileHeader := BitmapFileHeader{
		Type:       [2]byte{'B', 'M'},
		Size:       bmpFileHeaderSize + entry.BytesInRes,
		OffsetBits: bmpFileHeaderSize + bmpInfoHeaderSize,
	}

	// compose and decode the BMP image returning it as an image.Image
	// BitmapFileHeader + BitmapInfoHeader + Data
	bmpBuf := &bytes.Buffer{}
	if err = binary.Write(bmpBuf, binary.LittleEndian, bmpFileHeader); err != nil {
		return nil, fmt.Errorf("could not write BitmapFileHeader data: %s", err)
	}
	if err = binary.Write(bmpBuf, binary.LittleEndian, bmpInfoHeader); err != nil {
		return nil, fmt.Errorf("could not write BitmapInfoHeader data: %s", err)
	}
	bmpBuf.Write(buf.Bytes())
	return bmp.Decode(bmpBuf)
}

func DecodeConfig(r io.Reader) (image.Config, error) {
	return image.Config{}, fmt.Errorf("not implemented")
}

// IconHeader represents the Icon Directory Header structure
type IconHeader struct {
	Reserved uint16 // Reserved. Must always be 0.
	Type     uint16 // Specifies image type: 1 for icon (.ICO) image, 2 for cursor (.CUR) image. Other values are invalid.
	Count    uint16 // Specifies the number of iconDirectoryEntry
}

// IconDirectoryEntry represents the Icon Directory Entry structure
type IconDirectoryEntry struct {
	Width       uint8  // Specifies image width in pixels. Can be any number between 0 and 255. Value 0 means image width is 256 pixels.
	Height      uint8  // Specifies image height in pixels. Can be any number between 0 and 255. Value 0 means image height is 256 pixels.
	ColorCount  uint8  // Specifies number of colors in the color palette. Should be 0 if the image does not use a color palette.
	Reserved    uint8  // Reserved. Should be 0
	Planes      uint16 // Specifies color planes. Should be 0 or 1.
	BitCount    uint16 // Specifies bits per pixel.
	BytesInRes  uint32 // Specifies the size of the image's data in bytes.
	ImageOffset uint32 // Specifies the offset of BMP or PNG data from the beginning of the ICO/CUR file
}

// BitmapFileHeader represents the Bitmap File Header structure
type BitmapFileHeader struct {
	Type       [2]byte // The header field used to identify the BMP and DIB file is 0x42 0x4D in hexadecimal, same as BM in ASCII.
	Size       uint32  // The size of the BMP file in bytes
	Reserved1  uint16  // Reserved; actual value depends on the application that creates the image, if created manually can be 0
	Reserved2  uint16  // Reserved; actual value depends on the application that creates the image, if created manually can be 0
	OffsetBits uint32  // The offset, i.e. starting address, of the byte where the bitmap image data (pixel array) can be found.
}

// BitmapInfoHeader represents the Bitmap Info Header structure
type BitmapInfoHeader struct {
	Size     uint32 // the size of this header, in bytes (40)
	Width    int32  // the bitmap width in pixels (signed integer)
	Height   int32  // the bitmap width in pixels (signed integer)
	Planes   uint16 // the number of color planes (must be 1)
	BitCount uint16 // the number of bits per pixel, which is the color depth of the image. Typical values are 1, 4, 8, 16, 24 and 32.
	// Note for ico files the fields below can be zero
	Compression     uint32 // the compression method being used. See the next table for a list of possible values
	SizeImage       uint32 // the image size. This is the size of the raw bitmap data; a dummy 0 can be given for BI_RGB bitmaps.
	XPixelsPerMeter int32  // the horizontal resolution of the image. (pixel per metre, signed integer)
	YPixelsPerMeter int32  // the vertical resolution of the image. (pixel per metre, signed integer)
	ColorsUsed      uint32 // the number of colors in the color palette, or 0 to default to 2n
	ColorsImportant uint8  // the number of important colors used, or 0 when every color is important; generally ignored
}
