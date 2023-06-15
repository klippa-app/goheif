package libde265

import (
	"errors"
	"image"
	"log"
	"os"
	"os/exec"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"

	"github.com/klippa-app/goheif/libde265/requests"
	"github.com/klippa-app/goheif/libde265/shared"
)

type Decoder struct {
	id         string
	safeEncode bool
}

var client *plugin.Client
var gRPCClient plugin.ClientProtocol
var libde265plugin shared.Libde265
var currentConfig Config

type Config struct {
	Command Command
}

type Command struct {
	BinPath string
	Args    []string

	// StartTimeout is the timeout to wait for the plugin to say it
	// has started successfully.
	StartTimeout time.Duration
}

func Init(config Config) error {
	if client != nil {
		return nil
	}

	currentConfig = config

	return startPlugin()
}

func DeInit() {
	gRPCClient.Close()
	gRPCClient = nil
	client.Kill()
	client = nil
	libde265plugin = nil
}

func startPlugin() error {
	var handshakeConfig = plugin.HandshakeConfig{
		ProtocolVersion:  1,
		MagicCookieKey:   "BASIC_PLUGIN",
		MagicCookieValue: "libde265",
	}

	// pluginMap is the map of plugins we can dispense.
	var pluginMap = map[string]plugin.Plugin{
		"libde265": &shared.Libde265Plugin{},
	}

	logger := hclog.New(&hclog.LoggerOptions{
		Name:   "plugin",
		Output: os.Stdout,
		Level:  hclog.Debug,
	})

	client := plugin.NewClient(&plugin.ClientConfig{
		HandshakeConfig: handshakeConfig,
		Plugins:         pluginMap,
		Cmd:             exec.Command(currentConfig.Command.BinPath, currentConfig.Command.Args...),
		Logger:          logger,
	})

	rpcClient, err := client.Client()
	if err != nil {
		log.Fatal(err)
	}

	gRPCClient = rpcClient

	raw, err := rpcClient.Dispense("libde265")
	if err != nil {
		log.Fatal(err)
	}

	pluginInstance := raw.(shared.Libde265)
	pong, err := pluginInstance.Ping()
	if err != nil {
		return err
	}

	if pong != "Pong" {
		return errors.New("Wrong ping/pong result")
	}

	libde265plugin = pluginInstance

	return nil
}

func checkPlugin() error {
	pong, err := libde265plugin.Ping()
	if err != nil {
		log.Printf("restarting libde265 plugin due to wrong pong result: %s", err.Error())
		err = startPlugin()
		if err != nil {
			log.Printf("could not restart libde265 plugin: %s", err.Error())
			return err
		}
	}

	if pong != "Pong" {
		log.Printf("restarting libde265 plugin due to wrong pong result: %s", pong)
		err = startPlugin()
		if err != nil {
			log.Printf("could not restart libde265 plugin: %s", err.Error())
			return err
		}
	}

	return nil
}

var NotInitializedError = errors.New("libde265 was not initialized, you must call the Init() method")

type Option func(*Decoder)

func WithSafeEncoding(b bool) Option {
	return func(dec *Decoder) {
		dec.safeEncode = b
	}
}

func NewDecoder(opts ...Option) (*Decoder, error) {
	if libde265plugin == nil {
		return nil, NotInitializedError
	}

	err := checkPlugin()
	if err != nil {
		return nil, errors.New("could not check or start plugin")
	}

	dec := &Decoder{}
	for _, opt := range opts {
		opt(dec)
	}

	newDecoder, err := libde265plugin.NewDecoder(&requests.NewDecoder{
		SafeEncode: dec.safeEncode,
	})
	if err != nil {
		return nil, err
	}

	dec.id = newDecoder.ID
	return dec, nil
}

func (dec *Decoder) Free() error {
	if libde265plugin == nil {
		return NotInitializedError
	}

	err := checkPlugin()
	if err != nil {
		return errors.New("could not check or start plugin")
	}

	_, err = libde265plugin.CloseDecoder(&requests.CloseDecoder{ID: dec.id})
	if err != nil {
		return err
	}

	return nil
}

func (dec *Decoder) Reset() error {
	if libde265plugin == nil {
		return NotInitializedError
	}

	err := checkPlugin()
	if err != nil {
		return errors.New("could not check or start plugin")
	}

	_, err = libde265plugin.ResetDecoder(&requests.ResetDecoder{ID: dec.id})
	if err != nil {
		return err
	}

	return nil
}

func (dec *Decoder) Push(data []byte) error {
	if libde265plugin == nil {
		return NotInitializedError
	}

	err := checkPlugin()
	if err != nil {
		return errors.New("could not check or start plugin")
	}

	_, err = libde265plugin.PushDecoder(&requests.PushDecoder{ID: dec.id, Data: data})
	if err != nil {
		return err
	}
	return nil
}

func (dec *Decoder) DecodeImage(data []byte) (*image.YCbCr, error) {
	if libde265plugin == nil {
		return nil, NotInitializedError
	}

	err := checkPlugin()
	if err != nil {
		return nil, errors.New("could not check or start plugin")
	}

	resp, err := libde265plugin.RenderDecoder(&requests.RenderDecoder{ID: dec.id, Data: data})
	if err != nil {
		return nil, err
	}
	return resp.Image, nil
}
