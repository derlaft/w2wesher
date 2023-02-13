package config

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"github.com/go-playground/validator/v10"
	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"gopkg.in/ini.v1"
)

var log = logging.Logger("w2wesher")

var validate = validator.New()

// TODO: validate config on load
type Config struct {
	P2P       P2P
	Wireguard Wireguard
}

const (
	DefaultP2PListenAddr       = "/ip4/0.0.0.0/udp/10042"
	DefaultP2PAnnounceInterval = time.Minute * 5
)

type P2P struct {
	// Network PSK
	// If not present, will be generated.
	// TODO: find a nice way of configuring PSK?
	PSK string `validate:"base64"`
	// PrivateKey encoded in base64
	// If not present, will be generated.
	PrivateKey string `validate:"base64"`
	// List of Bootstrap nodes
	// Each node is a multiaddr, containing
	// * the last known peer addr
	// * the last known peer ID
	// Might be empty on start.
	// Will be periodically updated in runtime.
	Bootstrap []string
	// ListenAddr is a libp2p multiaddr
	ListenAddr       string
	AnnounceInterval time.Duration
}

const (
	// Default values for the Wireguard settings
	DefaultWgInterface           = "wesh0"
	DefaultWgNetworkRange        = "fd6d:142e:65e7:4cc1::/64"
	DefaultWgListenPort          = 10043
	DefaultWgPersistentKeepalive = time.Minute
)

type Wireguard struct {
	// Wireguard interface name.
	Interface string
	// PrivateKey encoded in base64.
	// If not present, will be generated.
	PrivateKey string `validate:"base64"`
	// Wireguard listen port.
	ListenPort int
	// NetworkRange to use.
	NetworkRange string `validate:"cidr"`
	// NodeName is a network hostname will be used for generating the addr.
	// If not present, will be replaced with a hostname on the first time.
	NodeName string `validate:"hostname"`
	// Wireguard PersistentKeepalive setting.
	// If not present, will not be enabled.
	PersistentKeepalive *time.Duration
}

func Load(filename string) (*Config, error) {

	cfg, err := ini.Load(filename)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("config: cannot parse ini: %w", err)
		}
		cfg = ini.Empty()
	}

	// Load config from disk
	var parsed = new(Config)
	err = cfg.MapTo(parsed)
	if err != nil {
		return nil, fmt.Errorf("config: cannot map ini: %w", err)
	}

	// Apply defaults, generate keys
	changed, err := parsed.Load()
	if err != nil {
		return nil, fmt.Errorf("config: load config: %w", err)
	}

	// Save back to disk if changed
	if changed {
		err := ini.ReflectFrom(cfg, parsed)
		if err != nil {
			return nil, fmt.Errorf("config: applying ini config: %w", err)
		}

		var buf = bytes.NewBuffer(nil)
		_, err = cfg.WriteTo(buf)
		if err != nil {
			return nil, fmt.Errorf("config: marshaling ini config: %w", err)
		}

		err = ioutil.WriteFile(filename, buf.Bytes(), 0600)
		if err != nil {
			return nil, fmt.Errorf("config: saving file: %w", err)
		}
	}

	return parsed, nil
}

func (c *Config) Load() (bool, error) {
	p2pChanged, err := c.P2P.Load()
	if err != nil {
		return false, err
	}

	wgChanged, err := c.Wireguard.Load()
	if err != nil {
		return false, err
	}

	return p2pChanged || wgChanged, nil
}

func (p *P2P) Load() (bool, error) {
	var changed bool

	if p.PSK == "" {
		err := p.GeneratePsk()
		if err != nil {
			return false, nil
		}
		changed = true
	}

	if p.PrivateKey == "" {
		err := p.GeneratePrivateKey()
		if err != nil {
			return false, nil
		}
		changed = true
	}

	if p.ListenAddr == "" {
		p.ListenAddr = DefaultP2PListenAddr
		changed = true
	}

	if p.AnnounceInterval == 0 {
		p.AnnounceInterval = DefaultP2PAnnounceInterval
		changed = true
	}

	err := validate.Struct(p)
	if err != nil {
		return false, err
	}

	return changed, nil
}

func (w *Wireguard) Load() (bool, error) {

	var changed bool

	if w.Interface == "" {
		w.Interface = "wesh0"
		changed = true
	}

	if w.PrivateKey == "" {
		err := w.GeneratePrivateKey()
		if err != nil {
			return false, err
		}
		changed = true
	}

	if w.ListenPort <= 0 {
		w.ListenPort = DefaultWgListenPort
		changed = true
	}

	if w.NetworkRange == "" {
		w.NetworkRange = DefaultWgNetworkRange
		changed = true
	}

	if w.NodeName == "" {
		// retrieve this data from the hostname
		w.NodeName, _ = os.Hostname()
		changed = true
	}

	if w.PersistentKeepalive == nil {
		v := DefaultWgPersistentKeepalive
		w.PersistentKeepalive = &v
		changed = true
	}

	err := validate.Struct(w)
	if err != nil {
		return false, err
	}

	return changed, nil
}

func (w *Wireguard) GeneratePrivateKey() error {
	private, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		return err
	}

	w.PrivateKey = private.String()
	return nil
}

func (p *P2P) LoadPrivateKey() (crypto.PrivKey, error) {

	data, err := base64.StdEncoding.DecodeString(p.PrivateKey)
	if err != nil {
		return nil, err
	}

	privateKey, err := crypto.UnmarshalPrivateKey(data)
	if err != nil {
		return nil, err
	}

	return privateKey, nil
}

func (p *P2P) GeneratePrivateKey() error {

	key, _, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		return err
	}

	privateKey, err := crypto.MarshalPrivateKey(key)
	if err != nil {
		return err
	}

	p.PrivateKey = base64.StdEncoding.EncodeToString(privateKey)

	return nil
}

func (p *P2P) LoadPsk() ([]byte, error) {
	return base64.StdEncoding.DecodeString(p.PSK)
}

func (p *P2P) GeneratePsk() error {
	var d = make([]byte, 32)
	_, err := rand.Read(d)
	if err != nil {
		return err
	}

	p.PSK = base64.StdEncoding.EncodeToString(d)
	return nil
}

func (p *P2P) LoadBootstrapPeers() ([]peer.AddrInfo, error) {

	var bootstrap []peer.AddrInfo
	for _, rawAddr := range p.Bootstrap {
		addr, err := peer.AddrInfoFromString(rawAddr)
		if err != nil {
			return nil, fmt.Errorf("config: invalid bootstrap addr %v: %w", rawAddr, err)
		}
		bootstrap = append(bootstrap, *addr)
	}

	log.With("bootstrap", bootstrap).Debug("loaded boostrap peers")

	return bootstrap, nil
}
