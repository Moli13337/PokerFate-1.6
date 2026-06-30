package ws

import (
	"io"
	"net"
	"sync"

	"github.com/gobwas/ws"
)

type Session struct {
	UID    int64
	conn   net.Conn
	server *Server
	send   chan []byte
	done   chan struct{}
	once   sync.Once
}

func NewSession(conn net.Conn, srv *Server) *Session {
	return &Session{
		conn:   conn,
		server: srv,
		send:   make(chan []byte, 256),
		done:   make(chan struct{}),
	}
}

func (s *Session) SendPacket(packType string, roomID uint32, body []byte) {
	data := Pack(packType, roomID, body)
	frame := ws.NewFrame(ws.OpBinary, true, data)
	raw := marshalFrame(frame)
	select {
	case s.send <- raw:
	default:
		s.Close()
	}
}

func (s *Session) Close() {
	s.once.Do(func() {
		close(s.done)
		s.conn.Close()
		if s.UID > 0 {
			s.server.RemoveSession(s.UID)
		}
	})
}

func (s *Session) ReadLoop() {
	defer s.Close()

	buf := make([]byte, 0, 4096)

	for {
		hdr, err := ws.ReadHeader(s.conn)
		if err != nil {
			return
		}

		payload := make([]byte, hdr.Length)
		if _, err := io.ReadFull(s.conn, payload); err != nil {
			return
		}

		if hdr.Masked {
			ws.Cipher(payload, hdr.Mask, 0)
		}

		if hdr.OpCode == ws.OpClose {
			return
		}
		if hdr.OpCode == ws.OpPing {
			pong := ws.NewFrame(ws.OpPong, true, payload)
			raw := marshalFrame(pong)
			s.send <- raw
			continue
		}
		if hdr.OpCode != ws.OpBinary && hdr.OpCode != ws.OpText {
			continue
		}

		buf = append(buf, payload...)
		for len(buf) > 0 {
			pkt, rest, err := Unpack(buf)
			if err != nil {
				return
			}
			if pkt == nil {
				break
			}
			buf = make([]byte, len(rest))
			copy(buf, rest)
			s.server.HandleMessage(s, pkt)
		}
	}
}

func (s *Session) WriteLoop() {
	defer s.Close()

	for {
		select {
		case <-s.done:
			return
		case msg, ok := <-s.send:
			if !ok {
				return
			}
			if _, err := s.conn.Write(msg); err != nil {
				return
			}
		}
	}
}

func marshalFrame(f ws.Frame) []byte {
	f.Header.Length = int64(len(f.Payload))
	var buf []byte
	ws.WriteHeader(&sliceWriter{buf: &buf}, f.Header)
	buf = append(buf, f.Payload...)
	return buf
}

type sliceWriter struct {
	buf *[]byte
}

func (w *sliceWriter) Write(p []byte) (int, error) {
	*w.buf = append(*w.buf, p...)
	return len(p), nil
}
