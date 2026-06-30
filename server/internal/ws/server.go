package ws

import (
	"bufio"
	"database/sql"
	"net"
	"net/http"
	"sync"

	"github.com/gobwas/ws"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"poker-fate-server/internal/config"
)

type Server struct {
	Config   *config.Config
	DB       *sql.DB
	Redis    *redis.Client
	Logger   *zap.Logger
	sessions map[int64]*Session
	handlers map[string]func(*Session, *Packet)
	mu       sync.RWMutex
}

func NewServer(cfg *config.Config, db *sql.DB, rdb *redis.Client, logger *zap.Logger) *Server {
	return &Server{
		Config:   cfg,
		DB:       db,
		Redis:    rdb,
		Logger:   logger,
		sessions: make(map[int64]*Session),
		handlers: make(map[string]func(*Session, *Packet)),
	}
}

func (s *Server) Start(addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				s.Logger.Warn("accept failed", zap.Error(err))
				continue
			}
			go s.handleConn(conn)
		}
	}()

	s.Logger.Info("ws server listening", zap.String("addr", addr))
	return nil
}

func (s *Server) handleConn(conn net.Conn) {
	// Use ws.Upgrade() for raw TCP connections - reads HTTP upgrade request from conn
	_, err := ws.Upgrade(conn)
	if err != nil {
		s.Logger.Debug("ws upgrade failed", zap.Error(err))
		conn.Close()
		return
	}

	sess := NewSession(conn, s)
	go sess.WriteLoop()
	sess.ReadLoop()
}

func (s *Server) HandleMessage(sess *Session, pkt *Packet) {
	s.mu.RLock()
	handler, ok := s.handlers[pkt.PackType]
	s.mu.RUnlock()

	if !ok {
		s.Logger.Info("unhandled packet type", zap.String("type", pkt.PackType), zap.Int("body_len", len(pkt.Body)))
		return
	}
	handler(sess, pkt)
}

func (s *Server) RegisterHandler(packType string, fn func(*Session, *Packet)) {
	s.mu.Lock()
	s.handlers[packType] = fn
	s.mu.Unlock()
}

func (s *Server) GetSession(uid int64) *Session {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sessions[uid]
}

// IsOnline reports whether a uid currently has an active WS session.
func (s *Server) IsOnline(uid int64) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.sessions[uid]
	return ok
}

func (s *Server) AddSession(uid int64, sess *Session) {
	s.mu.Lock()
	s.sessions[uid] = sess
	s.mu.Unlock()
}

func (s *Server) RemoveSession(uid int64) {
	s.mu.Lock()
	delete(s.sessions, uid)
	s.mu.Unlock()
}

func (s *Server) BroadcastToUIDs(uids []int64, packType string, roomID uint32, body []byte) {
	data := Pack(packType, roomID, body)
	frame := ws.NewFrame(ws.OpBinary, true, data)
	raw := marshalFrame(frame)
	s.mu.RLock()
	for _, uid := range uids {
		if sess, ok := s.sessions[uid]; ok {
			select {
			case sess.send <- raw:
			default:
			}
		}
	}
	s.mu.RUnlock()
}

func (s *Server) UpgradeHTTP(w http.ResponseWriter, r *http.Request) {
	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "websocket not supported", http.StatusInternalServerError)
		return
	}

	conn, br, err := hj.Hijack()
	if err != nil {
		return
	}

	_, _, _, err = ws.UpgradeHTTP(r, &nopResponseWriter{conn: conn})
	if err != nil {
		conn.Close()
		return
	}
	_ = br

	go func() {
		sess := NewSession(conn, s)
		go sess.WriteLoop()
		sess.ReadLoop()
	}()
}

type nopResponseWriter struct {
	conn   net.Conn
	header http.Header
	code   int
}

func (w *nopResponseWriter) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}

func (w *nopResponseWriter) WriteHeader(code int) { w.code = code }

func (w *nopResponseWriter) Write(p []byte) (int, error) { return w.conn.Write(p) }

func (w *nopResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return w.conn, bufio.NewReadWriter(bufio.NewReader(w.conn), bufio.NewWriter(w.conn)), nil
}
