package relay

import (
	"bytes"
	"errors"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/influxdata/influxdb/models"

	"github.com/strike-team/influxdb-relay/config"
)

const (
	defaultMTU = 1024
)

// UDP is a relay for UDP influxdb writes
type UDP struct {
	addr      string
	name      string
	precision string

	closing int64
	l       *net.UDPConn
	c       *net.UDPConn

	backends []*udpBackend
}

// NewUDP -TODO-
func NewUDP(config config.UDPConfig) (Relay, error) {
	u := new(UDP)

	u.name = config.Name
	u.addr = config.Addr
	u.precision = config.Precision

	l, err := net.ListenPacket("udp", u.addr)
	if err != nil {
		return nil, err
	}

	ul, ok := l.(*net.UDPConn)
	if !ok {
		return nil, errors.New("problem listening for UDP")
	}

	if config.ReadBuffer != 0 {
		if err = ul.SetReadBuffer(config.ReadBuffer); err != nil {
			return nil, err
		}
	}

	u.l = ul

	// UDP doesn't really "listen", this just gets us a socket with
	// the local UDP address set to something random
	u.c, err = net.ListenUDP("udp", nil)
	if err != nil {
		return nil, err
	}

	for i := range config.Outputs {
		cfg := &config.Outputs[i]
		if cfg.Name == "" {
			cfg.Name = cfg.Location
		}

		if cfg.MTU == 0 {
			cfg.MTU = defaultMTU
		}

		addr, err := net.ResolveUDPAddr("udp", cfg.Location)
		if err != nil {
			return nil, err
		}

		u.backends = append(u.backends, &udpBackend{u, cfg.Name, addr, cfg.MTU})
	}

	return u, nil
}

// Name -TODO-
func (u *UDP) Name() string {
	if u.name == "" {
		return u.addr
	}
	return u.name
}

// udpPool is used to reuse and auto-size payload buffers, if incoming packets
// are never larger than 2K, then none of the buffers will be larger than that.
// This prevents having to manually tune the UDP buffer size, or having every
// buffer be 64K to hold the maximum possible payload
var udpPool = sync.Pool{New: func() interface{} { return new(bytes.Buffer) }}

func getUDPBuf() *bytes.Buffer {
	return udpPool.Get().(*bytes.Buffer)
}

func putUDPBuf(b *bytes.Buffer) {
	b.Reset()
	udpPool.Put(b)
}

type packet struct {
	timestamp time.Time
	data      *bytes.Buffer
	from      *net.UDPAddr
}

// Run -TODO-
func (u *UDP) Run() error {

	// buffer that can hold the largest possible UDP payload
	var buf [65536]byte

	// arbitrary queue size for now
	queue := make(chan packet, 1024)

	var wg sync.WaitGroup

	go func() {
		for p := range queue {
			u.post(&p)
			wg.Done()
		}
	}()

	log.Printf("starting UDP relay %q on %v", u.Name(), u.l.LocalAddr())

	for {
		n, remote, err := u.l.ReadFromUDP(buf[:])
		if err != nil {
			if atomic.LoadInt64(&u.closing) == 0 {
				log.Printf("Error reading packet in relay %q from %v: %v", u.name, remote, err)
			} else {
				err = nil
			}
			close(queue)
			wg.Wait()
			return err
		}
		start := time.Now()

		wg.Add(1)

		// copy the data into a buffer and queue it for processing
		b := getUDPBuf()
		b.Grow(n)
		// bytes.Buffer.Write always returns a nil error, and will panic if out of memory
		_, _ = b.Write(buf[:n])
		queue <- packet{start, b, remote}
	}
}

// Stop -TODO-
func (u *UDP) Stop() error {
	atomic.StoreInt64(&u.closing, 1)
	return u.l.Close()
}

func (u *UDP) post(p *packet) {
	points, err := models.ParsePointsWithPrecision(p.data.Bytes(), p.timestamp, u.precision)
	if err != nil {
		log.Printf("Error parsing packet in relay %q from %v: %v", u.Name(), p.from, err)
		putUDPBuf(p.data)
		return
	}

	out := getUDPBuf()
	for _, pt := range points {
		if _, err = out.WriteString(pt.PrecisionString(u.precision)); err != nil {
			break
		}
		if err = out.WriteByte('\n'); err != nil {
			break
		}
	}

	putUDPBuf(p.data)

	if err != nil {
		putUDPBuf(out)
		log.Printf("Error writing points in relay %q: %v", u.Name(), err)
		return
	}

	for _, b := range u.backends {
		if err := b.post(out.Bytes()); err != nil {
			log.Printf("Error writing points in relay %q to backend %q: %v", u.Name(), b.name, err)
		}
	}

	putUDPBuf(out)
}

type udpBackend struct {
	u    *UDP
	name string
	addr *net.UDPAddr
	mtu  int
}

var errPacketTooLarge = errors.New("payload larger than MTU")

func (b *udpBackend) post(data []byte) error {
	var err error
	for len(data) > b.mtu {
		// find the last line that will fit within the MTU
		idx := bytes.LastIndexByte(data[:b.mtu], '\n')
		if idx < 0 {
			// first line is larger than MTU
			return errPacketTooLarge
		}
		_, err = b.u.c.WriteToUDP(data[:idx+1], b.addr)
		if err != nil {
			return err
		}
		data = data[idx+1:]
	}

	_, err = b.u.c.WriteToUDP(data, b.addr)
	return err
}
