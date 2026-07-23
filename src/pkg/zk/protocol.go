// Package zk is a clean-room client for the ZKTeco device TCP protocol (the
// "pull SDK", TCP port 4370). It implements exactly what this gateway needs:
// connect, optional COMM-key authentication, read attendance logs, read the
// serial number, and disconnect. It is written from the publicly documented
// protocol (the same one pyzk / node-zklib implement).
//
// Scope note: this implements the traditional PREPARE_DATA / DATA transfer path,
// which the large majority of ZKTeco attendance terminals support. A minority of
// firmware variants only speak the newer buffered transfer; for those, use push
// (ADMS/iclock) mode instead. Pull results should be spot-checked against your
// specific device model on first deployment.
package zk

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"time"
)

// Command codes.
const (
	cmdConnect       uint16 = 1000
	cmdExit          uint16 = 1001
	cmdEnableDevice  uint16 = 1002
	cmdDisableDevice uint16 = 1003
	cmdAckOK         uint16 = 2000
	cmdAckError      uint16 = 2001
	cmdAckUnauth     uint16 = 2005
	cmdPrepareData   uint16 = 1500
	cmdData          uint16 = 1501
	cmdAuth          uint16 = 1102
	cmdOptionsRRQ    uint16 = 11
	cmdAttlogRRQ     uint16 = 13
)

// TCP framing magic (little-endian shorts) that prefixes every packet.
const (
	tcpMagic1 uint16 = 0x5050
	tcpMagic2 uint16 = 0x827d
)

// Client is a single TCP session with one device. Not safe for concurrent use;
// the collector uses one Client per poll.
type Client struct {
	conn      net.Conn
	sessionID uint16
	replyID   uint16
	password  int
	timeout   time.Duration
}

// Dial opens a TCP connection to a device. Call Connect next.
func Dial(host string, port int, password int, timeout time.Duration) (*Client, error) {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", host, port), timeout)
	if err != nil {
		return nil, fmt.Errorf("zk: dial %s:%d: %w", host, port, err)
	}
	return &Client{conn: conn, password: password, timeout: timeout}, nil
}

// checksum16 computes the ZK packet checksum: a 16-bit ones-complement sum over
// the packet (with the checksum field zeroed), matching the reference protocol.
func checksum16(p []byte) uint16 {
	var sum uint32
	i, l := 0, len(p)
	for l > 1 {
		sum += uint32(binary.LittleEndian.Uint16(p[i : i+2]))
		i += 2
		l -= 2
	}
	if l == 1 {
		sum += uint32(p[i])
	}
	for sum > 0xffff {
		sum = (sum & 0xffff) + (sum >> 16)
	}
	return uint16(^sum)
}

// buildPacket assembles a full TCP-framed packet for a command + payload.
func (c *Client) buildPacket(cmd uint16, data []byte) []byte {
	c.replyID++
	inner := make([]byte, 8+len(data))
	binary.LittleEndian.PutUint16(inner[0:], cmd)
	binary.LittleEndian.PutUint16(inner[2:], 0) // checksum placeholder
	binary.LittleEndian.PutUint16(inner[4:], c.sessionID)
	binary.LittleEndian.PutUint16(inner[6:], c.replyID)
	copy(inner[8:], data)
	binary.LittleEndian.PutUint16(inner[2:], checksum16(inner))

	top := make([]byte, 8+len(inner))
	binary.LittleEndian.PutUint16(top[0:], tcpMagic1)
	binary.LittleEndian.PutUint16(top[2:], tcpMagic2)
	binary.LittleEndian.PutUint32(top[4:], uint32(len(inner)))
	copy(top[8:], inner)
	return top
}

// send writes a command packet.
func (c *Client) send(cmd uint16, data []byte) error {
	_ = c.conn.SetWriteDeadline(time.Now().Add(c.timeout))
	_, err := c.conn.Write(c.buildPacket(cmd, data))
	return err
}

// recv reads one TCP-framed packet and returns its command, session id and
// payload.
func (c *Client) recv() (cmd, session uint16, payload []byte, err error) {
	_ = c.conn.SetReadDeadline(time.Now().Add(c.timeout))
	top := make([]byte, 8)
	if _, err = io.ReadFull(c.conn, top); err != nil {
		return 0, 0, nil, fmt.Errorf("zk: read header: %w", err)
	}
	if binary.LittleEndian.Uint16(top[0:]) != tcpMagic1 || binary.LittleEndian.Uint16(top[2:]) != tcpMagic2 {
		return 0, 0, nil, errors.New("zk: bad TCP magic in response")
	}
	size := binary.LittleEndian.Uint32(top[4:])
	if size < 8 || size > 8<<20 { // sanity: 8 bytes header .. 8 MB
		return 0, 0, nil, fmt.Errorf("zk: implausible packet size %d", size)
	}
	pkt := make([]byte, size)
	if _, err = io.ReadFull(c.conn, pkt); err != nil {
		return 0, 0, nil, fmt.Errorf("zk: read body: %w", err)
	}
	cmd = binary.LittleEndian.Uint16(pkt[0:])
	session = binary.LittleEndian.Uint16(pkt[4:])
	payload = pkt[8:]
	return cmd, session, payload, nil
}

// Connect performs the handshake and COMM-key authentication if the device
// demands it.
func (c *Client) Connect() error {
	c.sessionID = 0
	c.replyID = 0
	if err := c.send(cmdConnect, nil); err != nil {
		return err
	}
	cmd, session, _, err := c.recv()
	if err != nil {
		return err
	}
	c.sessionID = session

	if cmd == cmdAckUnauth {
		// Device requires the COMM key.
		if err := c.send(cmdAuth, commKey(c.password, c.sessionID, 50)); err != nil {
			return err
		}
		cmd, _, _, err = c.recv()
		if err != nil {
			return err
		}
	}
	if cmd != cmdAckOK {
		return fmt.Errorf("zk: connect refused (command %d) — check COMM password", cmd)
	}
	return nil
}

// Disconnect enables the device (in case we disabled it) and closes the session.
func (c *Client) Disconnect() {
	_ = c.send(cmdExit, nil)
	_, _, _, _ = c.recv()
}

// Close closes the underlying socket.
func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// disableDevice / enableDevice bracket a read to stop the device mutating its
// buffer mid-transfer. Failures are non-fatal (best effort).
func (c *Client) disableDevice() {
	if c.send(cmdDisableDevice, []byte{0, 0, 0, 0}) == nil {
		_, _, _, _ = c.recv()
	}
}

func (c *Client) enableDevice() {
	if c.send(cmdEnableDevice, nil) == nil {
		_, _, _, _ = c.recv()
	}
}

// commKey derives the CMD_AUTH payload from the device COMM password and the
// session id, per the reference algorithm.
func commKey(key int, sessionID uint16, ticks byte) []byte {
	k := 0
	for i := 0; i < 32; i++ {
		if key&(1<<uint(i)) != 0 {
			k = (k << 1) | 1
		} else {
			k = k << 1
		}
	}
	k += int(sessionID)

	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, uint32(k))
	b[0] ^= 'Z'
	b[1] ^= 'K'
	b[2] ^= 'S'
	b[3] ^= 'O'

	h1 := binary.LittleEndian.Uint16(b[0:2])
	h2 := binary.LittleEndian.Uint16(b[2:4])
	binary.LittleEndian.PutUint16(b[0:2], h2)
	binary.LittleEndian.PutUint16(b[2:4], h1)

	B := ticks
	b[0] ^= B
	b[1] ^= B
	b[3] = B ^ b[3]
	return b
}
