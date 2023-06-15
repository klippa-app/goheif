package responses

import "image"

type NewDecoder struct {
	ID string
}

type CloseDecoder struct {
}

type ResetDecoder struct {
}

type PushDecoder struct {
}

type RenderDecoder struct {
	Image *image.YCbCr
}

type RenderFile struct {
	Output *[]byte
}
