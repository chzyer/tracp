// Traffic Control in Port level
package tracp

import (
	"math/rand"
	"net"
	"time"

	"gopkg.in/logex.v1"

	"github.com/chzyer/flagly"
	"github.com/chzyer/flow"
	"github.com/chzyer/tunnel"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

type RateLimit struct {
	Rate      int
	BandWidth int64
	time      int64
	current   int64
}

func (r *RateLimit) RandomDrop() bool {
	return rand.Intn(100) <= r.Rate
}

func (r *RateLimit) Process() int64 {
	now := time.Now().Unix()
	if now != r.time {
		return 0
	}
	return r.current
}

func (r *RateLimit) Drop(n int) bool {
	if r.RandomDrop() {
		return true
	}
	if r.BandWidth == 0 {
		return false
	}
	now := time.Now().Unix()
	if now != r.time {
		r.current = int64(n)
		r.time = now
		return false
	}
	if r.current >= r.BandWidth {
		return true
	}
	r.current += int64(n)
	return false
}

type Config struct {
	DropRate  int           `desc:"0-100"`
	BandWidth int           `desc:"max bandwidth per second, unit: byte"`
	MinDelay  time.Duration `desc:"min delay time"`
	MaxDelay  time.Duration `desc:"max delay time"`
}

func NewConfig() *Config {
	var cfg Config
	flagly.Bind(&cfg)
	return &cfg
}

func (c *Config) FlaglyVerify() error {
	if c.DropRate > 100 || c.DropRate < 0 {
		return flagly.Error("droprate in [0, 100]")
	}
	if c.MaxDelay < c.MinDelay {
		c.MaxDelay = c.MinDelay
	}
	return nil
}

type Tracp struct {
	cfg           *Config
	tun           *tunnel.Instance
	flow          *flow.Flow
	rateLimit     *RateLimit
	queue         *Queue
	newItemNotify chan struct{}

	tunOut chan []byte
	tunIn  chan []byte
}

func NewTracp(f *flow.Flow, cfg *Config) *Tracp {
	t := &Tracp{
		cfg:   cfg,
		flow:  f,
		queue: &Queue{},

		tunOut:        make(chan []byte),
		tunIn:         make(chan []byte),
		newItemNotify: make(chan struct{}, 1),
		rateLimit: &RateLimit{
			Rate:      cfg.DropRate,
			BandWidth: int64(cfg.BandWidth),
		},
	}
	f.SetOnClose(t.Close)
	return t
}

func (t *Tracp) Close() {
	t.tun.Close()
	t.flow.Close()
}

func (p *Tracp) initTun() error {
	t, err := tunnel.New(&tunnel.Config{
		DevId:   3,
		Gateway: net.ParseIP("10.0.0.254"),
		Mask:    net.CIDRMask(24, 32),
	})
	if err != nil {
		return logex.Trace(err)
	}

	p.tun = t
	go p.initTunRead()
	go p.initTunWrite()
	return nil
}

func (p *Tracp) initTunRead() {
	b := make([]byte, 65535)
	for {
		n, err := p.tun.Read(b)
		if err != nil {
			break
		}
		out := make([]byte, n)
		copy(out, b)
		select {
		case p.tunOut <- out:
		case <-p.flow.IsClose():
			return
		}
	}
}

func (p *Tracp) initTunWrite() {
	p.flow.Add(1)
	defer p.flow.DoneAndClose()
	for {
		select {
		case out := <-p.tunIn:
			p.tun.Write(out)
		case <-p.flow.IsClose():
			return
		}
	}
}

func (p *Tracp) sendPacket(packet gopacket.Packet) {
	buf := gopacket.NewSerializeBuffer()

	opts := gopacket.SerializeOptions{
		ComputeChecksums: true,
	}
	players := packet.Layers()
	slayer := make([]gopacket.SerializableLayer, 0, len(players))

	if tl := packet.TransportLayer(); tl != nil {
		switch t := tl.(type) {
		case *layers.TCP:
			t.SetNetworkLayerForChecksum(packet.NetworkLayer())
		case *layers.UDP:
			t.SetNetworkLayerForChecksum(packet.NetworkLayer())
		default:
			logex.Error("unknown type")
			return
		}
	}

	for _, l := range players {
		slayer = append(slayer, l.(gopacket.SerializableLayer))
	}

	if err := gopacket.SerializeLayers(buf, opts, slayer...); err != nil {
		logex.Error(err)
		return
	}

	ret := make([]byte, len(buf.Bytes())+4)
	ret[3] = 2
	copy(ret[4:], buf.Bytes())
	p.tunIn <- ret
}

func (p *Tracp) getSendTime() time.Time {
	duration := p.cfg.MaxDelay - p.cfg.MinDelay
	if duration > 0 {
		duration = time.Duration(rand.Int63n(int64(duration)))
	}
	duration += p.cfg.MinDelay
	if duration == 0 {
		return time.Time{}
	}
	return time.Now().Add(duration)
}

func (p *Tracp) smartSendPacket(packet gopacket.Packet) {
	send := p.getSendTime()
	if send.IsZero() {
		p.sendPacket(packet)
	} else {
		p.queue.SendInTime(packet, send)
		select {
		case p.newItemNotify <- struct{}{}:
		default:
		}
	}
}

func (p *Tracp) processPacket(n int, packet gopacket.Packet) {
	ipv4 := packet.NetworkLayer().(*layers.IPv4)
	dst := ipv4.DstIP[3]
	if p.rateLimit.Drop(n) {
		return
	}
	if dst == 253 {
		ipv4.DstIP = net.ParseIP("10.0.0.254")
		ipv4.SrcIP = net.ParseIP("10.0.0.1")
		p.smartSendPacket(packet)
		return
	}

	ipv4.DstIP = net.ParseIP("10.0.0.254")
	ipv4.SrcIP = net.ParseIP("10.0.0.253")
	p.smartSendPacket(packet)
}

func (p *Tracp) newPacket(b []byte) gopacket.Packet {
	return gopacket.NewPacket(
		b[4:], layers.LayerTypeIPv4, gopacket.Lazy)
}

func (p *Tracp) mainLoop() {
	p.flow.Add(1)
	defer p.flow.DoneAndClose()
	for {
		select {
		case <-p.flow.IsClose():
			return
		case out := <-p.tunOut:
			packet := p.newPacket(out)
			p.processPacket(len(out), packet)
		}
	}
}

func (p *Tracp) Run() {
	if err := p.initTun(); err != nil {
		p.flow.Error(err)
		return
	}
	go func() {
		for _ = range time.Tick(time.Second) {
			process := p.rateLimit.Process()
			if process > 0 {
				println(p.rateLimit.time, process)
			}
		}
	}()
	go p.sendQueueLoop()
	go p.mainLoop()
}

func (p *Tracp) sendQueueLoop() {
	p.flow.Add(1)
	defer p.flow.DoneAndClose()

	var wait time.Duration
	for {
		if wait > 0 {
			select {
			case <-time.After(wait):
			case <-p.newItemNotify:
			case <-p.flow.IsClose():
				return
			}
		}
		packet, send := p.queue.GetLastest()
		if send.IsZero() {
			wait = 100 * time.Second
			continue
		}
		if packet == nil {
			wait = time.Now().Sub(send)
		} else {
			p.sendPacket(packet)
			wait = 0
		}
	}
}
