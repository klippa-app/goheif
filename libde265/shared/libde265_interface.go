package shared

import (
	"fmt"
	"net/rpc"

	"github.com/klippa-app/goheif/libde265/requests"
	"github.com/klippa-app/goheif/libde265/responses"

	"github.com/hashicorp/go-plugin"
)

type Libde265 interface {
	Ping() (string, error)
	NewDecoder(*requests.NewDecoder) (*responses.NewDecoder, error)
	CloseDecoder(*requests.CloseDecoder) (*responses.CloseDecoder, error)
	PushDecoder(*requests.PushDecoder) (*responses.PushDecoder, error)
	ResetDecoder(*requests.ResetDecoder) (*responses.ResetDecoder, error)
	RenderDecoder(*requests.RenderDecoder) (*responses.RenderDecoder, error)
	RenderFile(*requests.RenderFile) (*responses.RenderFile, error)
}

type Libde265RPC struct{ client *rpc.Client }

func (g *Libde265RPC) Ping() (string, error) {
	var resp string
	err := g.client.Call("Plugin.Ping", new(interface{}), &resp)
	if err != nil {
		return "", err
	}

	return resp, nil
}

func (g *Libde265RPC) NewDecoder(request *requests.NewDecoder) (*responses.NewDecoder, error) {
	resp := &responses.NewDecoder{}
	err := g.client.Call("Plugin.NewDecoder", request, resp)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (g *Libde265RPC) CloseDecoder(request *requests.CloseDecoder) (*responses.CloseDecoder, error) {
	resp := &responses.CloseDecoder{}
	err := g.client.Call("Plugin.CloseDecoder", request, resp)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (g *Libde265RPC) PushDecoder(request *requests.PushDecoder) (*responses.PushDecoder, error) {
	resp := &responses.PushDecoder{}
	err := g.client.Call("Plugin.PushDecoder", request, resp)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (g *Libde265RPC) ResetDecoder(request *requests.ResetDecoder) (*responses.ResetDecoder, error) {
	resp := &responses.ResetDecoder{}
	err := g.client.Call("Plugin.ResetDecoder", request, resp)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (g *Libde265RPC) RenderDecoder(request *requests.RenderDecoder) (*responses.RenderDecoder, error) {
	resp := &responses.RenderDecoder{}
	err := g.client.Call("Plugin.RenderDecoder", request, resp)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (g *Libde265RPC) RenderFile(request *requests.RenderFile) (*responses.RenderFile, error) {
	resp := &responses.RenderFile{}
	err := g.client.Call("Plugin.RenderFile", request, resp)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

type Libde265RPCServer struct {
	Impl Libde265
}

func (s *Libde265RPCServer) Ping(args interface{}, resp *string) error {
	var err error
	*resp, err = s.Impl.Ping()
	if err != nil {
		return err
	}
	return nil
}

func (s *Libde265RPCServer) NewDecoder(request *requests.NewDecoder, resp *responses.NewDecoder) (err error) {
	defer func() {
		if panicError := recover(); panicError != nil {
			err = fmt.Errorf("panic occurred in %s: %v", "NewDecoder", panicError)
		}
	}()

	implResp, err := s.Impl.NewDecoder(request)
	if err != nil {
		return err
	}

	// Overwrite the target address of resp to the target address of implResp.
	*resp = *implResp

	return nil
}

func (s *Libde265RPCServer) CloseDecoder(request *requests.CloseDecoder, resp *responses.CloseDecoder) (err error) {
	defer func() {
		if panicError := recover(); panicError != nil {
			err = fmt.Errorf("panic occurred in %s: %v", "CloseDecoder", panicError)
		}
	}()

	implResp, err := s.Impl.CloseDecoder(request)
	if err != nil {
		return err
	}

	// Overwrite the target address of resp to the target address of implResp.
	*resp = *implResp

	return nil
}

func (s *Libde265RPCServer) PushDecoder(request *requests.PushDecoder, resp *responses.PushDecoder) (err error) {
	defer func() {
		if panicError := recover(); panicError != nil {
			err = fmt.Errorf("panic occurred in %s: %v", "PushDecoder", panicError)
		}
	}()

	implResp, err := s.Impl.PushDecoder(request)
	if err != nil {
		return err
	}

	// Overwrite the target address of resp to the target address of implResp.
	*resp = *implResp

	return nil
}

func (s *Libde265RPCServer) ResetDecoder(request *requests.ResetDecoder, resp *responses.ResetDecoder) (err error) {
	defer func() {
		if panicError := recover(); panicError != nil {
			err = fmt.Errorf("panic occurred in %s: %v", "ResetDecoder", panicError)
		}
	}()

	implResp, err := s.Impl.ResetDecoder(request)
	if err != nil {
		return err
	}

	// Overwrite the target address of resp to the target address of implResp.
	*resp = *implResp

	return nil
}

func (s *Libde265RPCServer) RenderDecoder(request *requests.RenderDecoder, resp *responses.RenderDecoder) (err error) {
	defer func() {
		if panicError := recover(); panicError != nil {
			err = fmt.Errorf("panic occurred in %s: %v", "RenderDecoder", panicError)
		}
	}()

	implResp, err := s.Impl.RenderDecoder(request)
	if err != nil {
		return err
	}

	// Overwrite the target address of resp to the target address of implResp.
	*resp = *implResp

	return nil
}

func (s *Libde265RPCServer) RenderFile(request *requests.RenderFile, resp *responses.RenderFile) (err error) {
	defer func() {
		if panicError := recover(); panicError != nil {
			err = fmt.Errorf("panic occurred in %s: %v", "RenderFile", panicError)
		}
	}()

	implResp, err := s.Impl.RenderFile(request)
	if err != nil {
		return err
	}

	// Overwrite the target address of resp to the target address of implResp.
	*resp = *implResp

	return nil
}

type Libde265Plugin struct {
	Impl Libde265
}

func (p *Libde265Plugin) Server(*plugin.MuxBroker) (interface{}, error) {
	return &Libde265RPCServer{Impl: p.Impl}, nil
}

func (Libde265Plugin) Client(b *plugin.MuxBroker, c *rpc.Client) (interface{}, error) {
	return &Libde265RPC{client: c}, nil
}
