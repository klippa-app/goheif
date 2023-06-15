package plugin

// #cgo pkg-config: libde265
// #include <stdint.h>
// #include <stdlib.h>
// #include "libde265/de265.h"
import "C"

import (
	"errors"
	"fmt"
	"image"
	"sync"
	"unsafe"

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

	err = dec.Push(request.Data)
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

	img, err := dec.DecodeImage(request.Data)
	if err != nil {
		return nil, err
	}

	return &responses.RenderDecoder{
		Image: img,
	}, nil
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
