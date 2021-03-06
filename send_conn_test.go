package quic

import (
	"errors"
	"net"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type mockPacketConnWrite struct {
	data []byte
	to   net.Addr
}

type mockPacketConn struct {
	addr         net.Addr
	dataToRead   chan []byte
	dataReadFrom net.Addr
	readErr      error
	dataWritten  chan mockPacketConnWrite
	closed       bool
}

func newMockPacketConn() *mockPacketConn {
	return &mockPacketConn{
		addr:        &net.UDPAddr{IP: net.IPv6zero, Port: 0x42},
		dataToRead:  make(chan []byte, 1000),
		dataWritten: make(chan mockPacketConnWrite, 1000),
	}
}

func (c *mockPacketConn) ReadFrom(b []byte) (int, net.Addr, error) {
	if c.readErr != nil {
		return 0, nil, c.readErr
	}
	data, ok := <-c.dataToRead
	if !ok {
		return 0, nil, errors.New("connection closed")
	}
	n := copy(b, data)
	return n, c.dataReadFrom, nil
}

func (c *mockPacketConn) WriteTo(b []byte, addr net.Addr) (n int, err error) {
	select {
	case c.dataWritten <- mockPacketConnWrite{to: addr, data: b}:
		return len(b), nil
	default:
		panic("channel full")
	}
}

func (c *mockPacketConn) Close() error {
	if !c.closed {
		close(c.dataToRead)
	}
	c.closed = true
	return nil
}
func (c *mockPacketConn) LocalAddr() net.Addr                { return c.addr }
func (c *mockPacketConn) SetDeadline(t time.Time) error      { panic("not implemented") }
func (c *mockPacketConn) SetReadDeadline(t time.Time) error  { panic("not implemented") }
func (c *mockPacketConn) SetWriteDeadline(t time.Time) error { panic("not implemented") }

var _ net.PacketConn = &mockPacketConn{}

var _ = Describe("Send-Connection", func() {
	var c sendConn
	var packetConn *mockPacketConn

	BeforeEach(func() {
		addr := &net.UDPAddr{
			IP:   net.IPv4(192, 168, 100, 200),
			Port: 1337,
		}
		packetConn = newMockPacketConn()
		c = newSendConn(packetConn, addr)
	})

	It("writes", func() {
		Expect(c.Write([]byte("foobar"))).To(Succeed())
		var write mockPacketConnWrite
		Expect(packetConn.dataWritten).To(Receive(&write))
		Expect(write.to.String()).To(Equal("192.168.100.200:1337"))
		Expect(write.data).To(Equal([]byte("foobar")))
	})

	It("gets the remote address", func() {
		Expect(c.RemoteAddr().String()).To(Equal("192.168.100.200:1337"))
	})

	It("gets the local address", func() {
		addr := &net.UDPAddr{
			IP:   net.IPv4(192, 168, 0, 1),
			Port: 1234,
		}
		packetConn.addr = addr
		Expect(c.LocalAddr()).To(Equal(addr))
	})

	It("closes", func() {
		err := c.Close()
		Expect(err).ToNot(HaveOccurred())
		Expect(packetConn.closed).To(BeTrue())
	})
})
