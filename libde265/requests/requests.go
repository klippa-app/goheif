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
	Data []byte
}

type RenderDecoder struct {
	ID   string
	Data []byte
}
