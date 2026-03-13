package netrelay

import (
	"bytes"
	"context"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"cid_gio_gio/internal/config"
	"cid_gio_gio/internal/core"
	appLog "cid_gio_gio/internal/logger"

	"github.com/rs/zerolog/log"
)

type DeviceEvent struct {
	DeviceID     int
	Data         string
	Time         time.Time
	Remote       string
	RelayBlocked bool
}

type TCPServer struct {
	cfg   config.ServerConfig
	rules config.CidRulesConfig
	queue *core.MessageQueue
	stats *core.Metrics

	listener net.Listener
	mu       sync.RWMutex
	conns    map[net.Conn]struct{}
	onDevice func(DeviceEvent)
	onEvent  func(DeviceEvent)

	filterMu    sync.RWMutex
	relayFilter core.RelayFilterRule
}

const (
	ackByte       = byte(0x06)
	nackByte      = byte(0x15)
	termByte      = byte(0x14)
	readTimeout   = 60 * time.Second
	writeTimeout  = 10 * time.Second
	replyTimeout  = 10 * time.Second
	maxBufferSize = 8192
)

func NewTCPServer(cfg config.ServerConfig, rules config.CidRulesConfig, queue *core.MessageQueue, stats *core.Metrics) *TCPServer {
	return &TCPServer{
		cfg:   cfg,
		rules: rules,
		queue: queue,
		stats: stats,
		conns: make(map[net.Conn]struct{}),
	}
}

func (s *TCPServer) SetCallbacks(onDevice, onEvent func(DeviceEvent)) {
	s.onDevice = onDevice
	s.onEvent = onEvent
}

func (s *TCPServer) UpdateRelayFilter(filter core.RelayFilterRule) {
	s.filterMu.Lock()
	defer s.filterMu.Unlock()
	s.relayFilter = filter
}

func (s *TCPServer) Run(ctx context.Context) error {
	defer appLog.RecoverPanic("tcp-server-run")
	addr := net.JoinHostPort(s.cfg.Host, s.cfg.Port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		log.Error().Err(err).Str("addr", addr).Msg("tcp server listen failed")
		return err
	}
	s.listener = ln
	log.Info().Str("addr", addr).Msg("tcp server listening")
	go func() {
		<-ctx.Done()
		s.closeAllClients()
	}()
	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				log.Info().Str("addr", addr).Msg("tcp server stopped")
				return nil
			default:
				log.Warn().Err(err).Msg("tcp server accept failed")
				continue
			}
		}
		go s.handleClient(ctx, conn)
	}
}

func (s *TCPServer) Stop() {
	if s.listener != nil {
		_ = s.listener.Close()
	}
	s.closeAllClients()
}

func (s *TCPServer) handleClient(ctx context.Context, conn net.Conn) {
	defer appLog.RecoverPanic("tcp-server-client")
	s.trackConn(conn)
	defer s.untrackConn(conn)
	defer conn.Close()
	log.Debug().Str("remote", conn.RemoteAddr().String()).Msg("client connected")
	buf := make([]byte, 4096)
	acc := make([]byte, 0, 8192)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		_ = conn.SetReadDeadline(time.Now().Add(readTimeout))
		n, err := conn.Read(buf)
		if err != nil {
			log.Debug().Err(err).Str("remote", conn.RemoteAddr().String()).Msg("client read failed")
			return
		}
		if n <= 0 {
			log.Debug().Str("remote", conn.RemoteAddr().String()).Msg("client disconnected")
			return
		}
		acc = append(acc, buf[:n]...)

		for {
			idx := bytes.IndexByte(acc, termByte)
			if idx < 0 {
				break
			}
			msgBytes := acc[:idx]
			acc = acc[idx+1:]

			if core.IsHeartbeatBytes(msgBytes) {
				if ctx.Err() != nil {
					return
				}
				safeWrite(conn, ackByte)
				continue
			}
			if !core.IsMessageValidBytes(msgBytes, s.rules) {
				if ctx.Err() != nil {
					return
				}
				safeWrite(conn, nackByte)
				continue
			}
			outgoing, err := core.ChangeAccountNumber(msgBytes, s.rules)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				safeWrite(conn, nackByte)
				continue
			}

			reply := make(chan core.DeliveryResult, 1)
			if s.stats != nil {
				s.stats.IncReceived()
			}

			id := extractDeviceID(outgoing)
			code := extractEventCode(outgoing)
			groupNo := extractGroupNo(outgoing)
			zoneNo := extractZoneNo(outgoing)

			s.filterMu.RLock()
			filter := s.relayFilter
			s.filterMu.RUnlock()
			relayBlocked := shouldBypassRelay(filter, id, groupNo, zoneNo, code)
			evt := DeviceEvent{
				DeviceID:     id,
				Data:         string(outgoing),
				Time:         time.Now(),
				Remote:       conn.RemoteAddr().String(),
				RelayBlocked: relayBlocked,
			}

			if relayBlocked {
				if s.onDevice != nil {
					s.onDevice(evt)
				}
				if s.onEvent != nil {
					s.onEvent(evt)
				}
				if ctx.Err() != nil {
					return
				}
				safeWrite(conn, ackByte)
				continue
			}

			if !s.queue.Enqueue(core.SharedMessage{Payload: outgoing, ReplyCh: reply}) {
				log.Warn().Int("device_id", id).Msg("queue full, message dropped")
				if ctx.Err() != nil {
					return
				}
				safeWrite(conn, nackByte)
				continue
			}

			ok := waitReply(ctx, reply)
			if ok {
				if s.onDevice != nil {
					s.onDevice(evt)
				}
				if s.onEvent != nil {
					s.onEvent(evt)
				}
				if ctx.Err() != nil {
					return
				}
				safeWrite(conn, ackByte)
			} else {
				if ctx.Err() != nil {
					return
				}
				safeWrite(conn, nackByte)
			}
		}
		if len(acc) > maxBufferSize {
			log.Warn().Str("remote", conn.RemoteAddr().String()).Msg("buffer overflow, closing connection")
			return
		}
	}
}

func (s *TCPServer) trackConn(conn net.Conn) {
	s.mu.Lock()
	s.conns[conn] = struct{}{}
	count := len(s.conns)
	s.mu.Unlock()
	if s.stats != nil {
		s.stats.SetClients(count)
	}
}

func (s *TCPServer) untrackConn(conn net.Conn) {
	s.mu.Lock()
	delete(s.conns, conn)
	count := len(s.conns)
	s.mu.Unlock()
	if s.stats != nil {
		s.stats.SetClients(count)
	}
}

func (s *TCPServer) closeAllClients() {
	s.mu.Lock()
	conns := make([]net.Conn, 0, len(s.conns))
	for c := range s.conns {
		conns = append(conns, c)
	}
	s.mu.Unlock()
	for _, c := range conns {
		_ = c.Close()
	}
}

func waitReply(ctx context.Context, ch <-chan core.DeliveryResult) bool {
	t := time.NewTimer(replyTimeout)
	defer t.Stop()
	select {
	case res := <-ch:
		return res.Status
	case <-t.C:
		return false
	case <-ctx.Done():
		return false
	}
}

func safeWrite(conn net.Conn, b byte) {
	_ = conn.SetWriteDeadline(time.Now().Add(writeTimeout))
	var buf [1]byte
	buf[0] = b
	_, _ = conn.Write(buf[:])
}

func extractDeviceID(msg []byte) int {
	if len(msg) < 11 {
		return 0
	}
	id := 0
	for _, ch := range msg[7:11] {
		if ch < '0' || ch > '9' {
			return 0
		}
		id = id*10 + int(ch-'0')
	}
	return id
}

func extractEventCode(msg []byte) string {
	if len(msg) < 15 {
		return ""
	}
	return strings.ToUpper(strings.TrimSpace(string(msg[11:15])))
}

func extractGroupNo(msg []byte) int {
	if len(msg) < 17 {
		return -1
	}
	text := strings.TrimSpace(string(msg[15:17]))
	if text == "" {
		return -1
	}
	g, err := strconv.Atoi(text)
	if err != nil {
		return -1
	}
	return g
}

func extractZoneNo(msg []byte) int {
	if len(msg) < 20 {
		return -1
	}
	text := strings.TrimSpace(string(msg[17:20]))
	if text == "" {
		return -1
	}
	z, err := strconv.Atoi(text)
	if err != nil {
		return -1
	}
	return z
}

func shouldBypassRelay(filter core.RelayFilterRule, deviceID, partitionNo, zoneNo int, code string) bool {
	if !filter.Enabled {
		return false
	}
	if code == "" {
		return false
	}

	// 1. Check per-object codes first
	if objCodes, ok := filter.ObjectCodes[deviceID]; ok {
		for _, c := range objCodes {
			if strings.EqualFold(c, code) || (len(code) == 4 && strings.EqualFold(c, code[1:])) {
				codeKey := strings.ToUpper(strings.TrimSpace(c))
				// Code matches, check details if any
				if matchDetails(filter.ObjCodeDetails[deviceID][codeKey], partitionNo, zoneNo) {
					return true
				}
			}
		}
	}

	// 2. Fallback to global codes + object/group filter
	if len(filter.Codes) == 0 {
		return false
	}

	codeInGlobal := false
	for _, c := range filter.Codes {
		if strings.EqualFold(c, code) || (len(code) == 4 && strings.EqualFold(c, code[1:])) {
			codeKey := strings.ToUpper(strings.TrimSpace(c))
			// Code matches global list, check global details
			if matchDetails(filter.CodeDetails[codeKey], partitionNo, zoneNo) {
				codeInGlobal = true
				break
			}
		}
	}

	if !codeInGlobal {
		return false
	}

	// Global match found, now check if this device is specifically targeted (if targets defined)
	hasObjectTargets := len(filter.ObjectIDs) > 0
	hasGroupTargets := len(filter.GroupNumbers) > 0
	if !hasObjectTargets && !hasGroupTargets {
		return true
	}

	objectMatch := false
	if hasObjectTargets {
		for _, id := range filter.ObjectIDs {
			if id == deviceID {
				objectMatch = true
				break
			}
		}
	}

	groupMatch := false
	if hasGroupTargets {
		for _, g := range filter.GroupNumbers {
			if g == partitionNo {
				groupMatch = true
				break
			}
		}
	}

	return objectMatch || groupMatch
}

func matchDetails(d core.RelayFilterDetail, partitionNo, zoneNo int) bool {
	// If no specific details (zones/partitions) are defined, it's a catch-all for this code
	if len(d.Zones) == 0 && len(d.Partitions) == 0 {
		return true
	}

	// Check partitions
	partitionMatch := false
	if len(d.Partitions) > 0 {
		for _, p := range d.Partitions {
			if p == partitionNo {
				partitionMatch = true
				break
			}
		}
	} else {
		partitionMatch = true // No partition filter means all partitions match
	}

	// Check zones
	zoneMatch := false
	if len(d.Zones) > 0 {
		for _, z := range d.Zones {
			if z == zoneNo {
				zoneMatch = true
				break
			}
		}
	} else {
		zoneMatch = true // No zone filter means all zones match
	}

	return partitionMatch && zoneMatch
}
