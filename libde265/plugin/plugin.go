package plugin

// #cgo pkg-config: libde265
// #include <stdint.h>
// #include <stdlib.h>
// #include "libde265/de265.h"
import "C"

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"sync"
	"unsafe"

	"github.com/klippa-app/goheif/heif"
	"github.com/klippa-app/goheif/libde265/requests"
	"github.com/klippa-app/goheif/libde265/responses"
	"github.com/klippa-app/goheif/libde265/shared"

	"github.com/google/uuid"
	"github.com/hashicorp/go-plugin"
)

var handshakeConfig = plugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "BASIC_PLUGIN",
	MagicCookieValue: "libde265",
}

func StartPlugin() {
	var pluginMap = map[string]plugin.Plugin{
		"libde265": &shared.Libde265Plugin{Impl: &libde265Implementation{
			Decoders:     map[string]*decoder{},
			DecodersLock: sync.Mutex{},
		}},
	}

	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: handshakeConfig,
		Plugins:         pluginMap,
	})
}

type libde265Implementation struct {
	Decoders     map[string]*decoder
	DecodersLock sync.Mutex
}

func (l *libde265Implementation) Ping() (string, error) {
	return "Pong", nil
}

func (l *libde265Implementation) getDecoder(id string) (*decoder, error) {
	l.DecodersLock.Lock()
	defer l.DecodersLock.Unlock()

	val, ok := l.Decoders[id]
	if !ok {
		return nil, errors.New("could not find decoder")
	}

	return val, nil
}

func (l *libde265Implementation) NewDecoder(request *requests.NewDecoder) (*responses.NewDecoder, error) {
	p := C.de265_new_decoder()
	if p == nil {
		return nil, fmt.Errorf("Unable to create decoder")
	}

	dec := &decoder{ctx: p, hasImage: request.SafeEncode}
	newID := uuid.New().String()
	l.DecodersLock.Lock()
	defer l.DecodersLock.Unlock()
	l.Decoders[newID] = dec

	return &responses.NewDecoder{ID: newID}, nil
}

func (l *libde265Implementation) CloseDecoder(request *requests.CloseDecoder) (*responses.CloseDecoder, error) {
	dec, err := l.getDecoder(request.ID)
	if err != nil {
		return nil, err
	}

	dec.Free()
	l.DecodersLock.Lock()
	defer l.DecodersLock.Unlock()
	delete(l.Decoders, request.ID)

	return &responses.CloseDecoder{}, nil
}

func (l *libde265Implementation) ResetDecoder(request *requests.ResetDecoder) (*responses.ResetDecoder, error) {
	dec, err := l.getDecoder(request.ID)
	if err != nil {
		return nil, err
	}

	dec.Reset()

	return &responses.ResetDecoder{}, nil
}

func (l *libde265Implementation) PushDecoder(request *requests.PushDecoder) (*responses.PushDecoder, error) {
	dec, err := l.getDecoder(request.ID)
	if err != nil {
		return nil, err
	}

	err = dec.Push(*request.Data)
	if err != nil {
		return nil, err
	}

	return &responses.PushDecoder{}, nil
}

func (l *libde265Implementation) RenderDecoder(request *requests.RenderDecoder) (*responses.RenderDecoder, error) {
	dec, err := l.getDecoder(request.ID)
	if err != nil {
		return nil, err
	}

	img, err := dec.DecodeImage(*request.Data)
	if err != nil {
		return nil, err
	}

	return &responses.RenderDecoder{
		Image: img,
	}, nil
}

type gridBox struct {
	columns, rows int
	width, height int
}

func newGridBox(data []byte) (*gridBox, error) {
	if len(data) < 8 {
		return nil, fmt.Errorf("invalid data")
	}
	// version := data[0]
	flags := data[1]
	rows := int(data[2]) + 1
	columns := int(data[3]) + 1

	var width, height int
	if (flags & 1) != 0 {
		if len(data) < 12 {
			return nil, fmt.Errorf("invalid data")
		}

		width = int(data[4])<<24 | int(data[5])<<16 | int(data[6])<<8 | int(data[7])
		height = int(data[8])<<24 | int(data[9])<<16 | int(data[10])<<8 | int(data[11])
	} else {
		width = int(data[4])<<8 | int(data[5])
		height = int(data[6])<<8 | int(data[7])
	}

	return &gridBox{columns: columns, rows: rows, width: width, height: height}, nil
}

func (l *libde265Implementation) decodeHevcItem(decoderID string, hf *heif.File, item *heif.Item) (*image.YCbCr, error) {
	if item.Info.ItemType != "hvc1" {
		return nil, fmt.Errorf("Unsupported item type: %s", item.Info.ItemType)
	}

	hvcc, ok := item.HevcConfig()
	if !ok {
		return nil, fmt.Errorf("No hvcC")
	}

	hdr := hvcc.AsHeader()
	data, err := hf.GetItemData(item)
	if err != nil {
		return nil, err
	}

	dec, err := l.getDecoder(decoderID)
	if err != nil {
		return nil, err
	}

	dec.Reset()
	dec.Push(hdr)
	ycc, err := dec.DecodeImage(data)
	if err != nil {
		return nil, err
	}

	return ycc, nil
}

func (l *libde265Implementation) RenderFile(request *requests.RenderFile) (*responses.RenderFile, error) {
	ra := bytes.NewReader(*request.Data)
	hf := heif.Open(ra)

	it, err := hf.PrimaryItem()
	if err != nil {
		return nil, err
	}

	width, height, ok := it.SpatialExtents()
	if !ok {
		return nil, fmt.Errorf("No dimension")
	}

	if it.Info == nil {
		return nil, fmt.Errorf("No item info")
	}

	resp, err := l.NewDecoder(&requests.NewDecoder{SafeEncode: request.SafeEncoding})
	if err != nil {
		return nil, err
	}
	decoderID := resp.ID

	defer l.CloseDecoder(&requests.CloseDecoder{ID: decoderID})

	var outImage *image.YCbCr
	if it.Info.ItemType == "hvc1" {
		outImage, err = l.decodeHevcItem(decoderID, hf, it)
		if err != nil {
			return nil, err
		}
	} else {
		if it.Info.ItemType != "grid" {
			return nil, fmt.Errorf("No grid")
		}

		data, err := hf.GetItemData(it)
		if err != nil {
			return nil, err
		}

		grid, err := newGridBox(data)
		if err != nil {
			return nil, err
		}

		dimg := it.Reference("dimg")
		if dimg == nil {
			return nil, fmt.Errorf("No dimg")
		}

		if len(dimg.ToItemIDs) != grid.columns*grid.rows {
			return nil, fmt.Errorf("Tiles number not matched")
		}

		var tileWidth, tileHeight int
		for i, y := 0, 0; y < grid.rows; y += 1 {
			for x := 0; x < grid.columns; x += 1 {
				id := dimg.ToItemIDs[i]
				item, err := hf.ItemByID(id)
				if err != nil {
					return nil, err
				}

				ycc, err := l.decodeHevcItem(decoderID, hf, item)
				if err != nil {
					return nil, err
				}

				rect := ycc.Bounds()
				if tileWidth == 0 {
					tileWidth, tileHeight = rect.Dx(), rect.Dy()
					width, height := tileWidth*grid.columns, tileHeight*grid.rows
					outImage = image.NewYCbCr(image.Rectangle{image.Pt(0, 0), image.Pt(width, height)}, ycc.SubsampleRatio)
				}

				if tileWidth != rect.Dx() || tileHeight != rect.Dy() {
					return nil, fmt.Errorf("Inconsistent tile dimensions")
				}

				// copy y stride data
				for i := 0; i < rect.Dy(); i += 1 {
					copy(outImage.Y[(y*tileHeight+i)*outImage.YStride+x*ycc.YStride:], ycc.Y[i*ycc.YStride:(i+1)*ycc.YStride])
				}

				// height of c strides
				cHeight := len(ycc.Cb) / ycc.CStride

				// copy c stride data
				for i := 0; i < cHeight; i += 1 {
					copy(outImage.Cb[(y*cHeight+i)*outImage.CStride+x*ycc.CStride:], ycc.Cb[i*ycc.CStride:(i+1)*ycc.CStride])
					copy(outImage.Cr[(y*cHeight+i)*outImage.CStride+x*ycc.CStride:], ycc.Cr[i*ycc.CStride:(i+1)*ycc.CStride])
				}

				i += 1
			}
		}

		//crop to actual size when applicable
		outImage.Rect = image.Rectangle{image.Pt(0, 0), image.Pt(width, height)}
	}

	var imgBuf bytes.Buffer
	if request.OutputFormat == requests.RenderFileOutputFormatJPG {
		var opt jpeg.Options
		opt.Quality = 95

		for {
			err := jpeg.Encode(&imgBuf, outImage, &opt)
			if err != nil {
				return nil, err
			}

			if request.MaxFileSize == 0 || int64(imgBuf.Len()) < request.MaxFileSize {
				break
			}

			opt.Quality -= 10

			if opt.Quality <= 45 {
				return nil, errors.New("image would exceed maximum filesize")
			}

			imgBuf.Reset()
		}
	} else if request.OutputFormat == requests.RenderFileOutputFormatPNG {
		err := png.Encode(&imgBuf, outImage)
		if err != nil {
			return nil, err
		}

		if request.MaxFileSize != 0 && int64(imgBuf.Len()) > request.MaxFileSize {
			return nil, errors.New("image would exceed maximum filesize")
		}
	} else {
		return nil, errors.New("invalid output format given")
	}

	output := imgBuf.Bytes()
	return &responses.RenderFile{Output: &output}, nil
}

type decoder struct {
	ctx        unsafe.Pointer
	hasImage   bool
	safeEncode bool
}

func (dec *decoder) Free() {
	dec.Reset()
	C.de265_free_decoder(dec.ctx)
}

func (dec *decoder) Reset() {
	if dec.ctx != nil && dec.hasImage {
		C.de265_release_next_picture(dec.ctx)
		dec.hasImage = false
	}

	C.de265_reset(dec.ctx)
}

func (dec *decoder) Push(data []byte) error {
	var pos int
	totalSize := len(data)
	for pos < totalSize {
		if pos+4 > totalSize {
			return fmt.Errorf("Invalid NAL data")
		}

		nalSize := uint32(data[pos])<<24 | uint32(data[pos+1])<<16 | uint32(data[pos+2])<<8 | uint32(data[pos+3])
		pos += 4

		if pos+int(nalSize) > totalSize {
			return fmt.Errorf("Invalid NAL size: %d", nalSize)
		}

		C.de265_push_NAL(dec.ctx, unsafe.Pointer(&data[pos]), C.int(nalSize), C.de265_PTS(0), nil)
		pos += int(nalSize)
	}

	return nil
}

func (dec *decoder) DecodeImage(data []byte) (*image.YCbCr, error) {
	if dec.hasImage {
		fmt.Printf("previous image may leak")
	}

	if len(data) > 0 {
		if err := dec.Push(data); err != nil {
			return nil, err
		}
	}

	if ret := C.de265_flush_data(dec.ctx); ret != C.DE265_OK {
		return nil, fmt.Errorf("flush_data error")
	}

	var more C.int = 1
	for more != 0 {
		if decerr := C.de265_decode(dec.ctx, &more); decerr != C.DE265_OK {
			return nil, fmt.Errorf("decode error")
		}

		for {
			warning := C.de265_get_warning(dec.ctx)
			if warning == C.DE265_OK {
				break
			}
			fmt.Printf("warning: %v\n", C.GoString(C.de265_get_error_text(warning)))
		}

		if img := C.de265_get_next_picture(dec.ctx); img != nil {
			dec.hasImage = true // lazy release

			width := C.de265_get_image_width(img, 0)
			height := C.de265_get_image_height(img, 0)

			var ystride, cstride C.int
			y := C.de265_get_image_plane(img, 0, &ystride)
			cb := C.de265_get_image_plane(img, 1, &cstride)
			cheight := C.de265_get_image_height(img, 1)
			cr := C.de265_get_image_plane(img, 2, &cstride)
			//			crh := C.de265_get_image_height(img, 2)

			// sanity check
			if int(height)*int(ystride) >= int(1<<30) {
				return nil, fmt.Errorf("image too big")
			}

			var r image.YCbCrSubsampleRatio
			switch chroma := C.de265_get_chroma_format(img); chroma {
			case C.de265_chroma_420:
				r = image.YCbCrSubsampleRatio420
			case C.de265_chroma_422:
				r = image.YCbCrSubsampleRatio422
			case C.de265_chroma_444:
				r = image.YCbCrSubsampleRatio444
			}
			ycc := &image.YCbCr{
				YStride:        int(ystride),
				CStride:        int(cstride),
				SubsampleRatio: r,
				Rect:           image.Rectangle{Min: image.Point{0, 0}, Max: image.Point{int(width), int(height)}},
			}
			if dec.safeEncode {
				ycc.Y = C.GoBytes(unsafe.Pointer(y), C.int(height*ystride))
				ycc.Cb = C.GoBytes(unsafe.Pointer(cb), C.int(cheight*cstride))
				ycc.Cr = C.GoBytes(unsafe.Pointer(cr), C.int(cheight*cstride))
			} else {
				ycc.Y = (*[1 << 30]byte)(unsafe.Pointer(y))[:int(height)*int(ystride)]
				ycc.Cb = (*[1 << 30]byte)(unsafe.Pointer(cb))[:int(cheight)*int(cstride)]
				ycc.Cr = (*[1 << 30]byte)(unsafe.Pointer(cr))[:int(cheight)*int(cstride)]
			}

			//C.de265_release_next_picture(dec.ctx)

			return ycc, nil
		}
	}

	return nil, fmt.Errorf("No picture")
}
