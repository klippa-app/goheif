package requests

type NewDecoder struct {
	SafeEncode bool
}

type CloseDecoder struct {
	ID string
}

type ResetDecoder struct {
	ID string
}

type PushDecoder struct {
	ID   string
	Data *[]byte
}

type RenderDecoder struct {
	ID   string
	Data *[]byte
}

type RenderFileOutputFormat string // The file format to render output as.

const (
	RenderFileOutputFormatJPG RenderFileOutputFormat = "jpg" // Render the file as a JPEG file.
	RenderFileOutputFormatPNG RenderFileOutputFormat = "png" // Render the file as a PNG file.
)

type RenderFile struct {
	Data         *[]byte                // The file data.
	OutputFormat RenderFileOutputFormat // The format to output the image as
	MaxFileSize  int64                  // The maximum filesize, if jpg is chosen as output format, it will try to compress it until it fits
	SafeEncoding bool                   // Whether to use safe encoding.
}
