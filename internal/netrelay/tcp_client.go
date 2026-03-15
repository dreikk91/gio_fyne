package netrelay

import (
	"context"
	"net"
	"sync"
	"time"

	"cid_fyne/internal/config"
	"cid_fyne/internal/core"
	appLog "cid_fyne/internal/logger"

	"github.com/rs/zerolog/log"
)

type RelayClient struct {
	cfg     config.ClientConfig
	queue   *core.MessageQueue
	metrics *core.Metrics
	cancel  context.CancelFunc

	filterMu    sync.RWMutex
	relayFilter core.RelayFilterRule

	connMu sync.Mutex
	conn   net.Conn
}

func NewRelayClient(cfg config.ClientConfig, queue *core.MessageQueue, metrics *core.Metrics) *RelayClient {
	return &RelayClient{cfg: cfg, queue: queue, metrics: metrics}
}

func (r *RelayClient) UpdateRelayFilter(filter core.RelayFilterRule) {
	r.filterMu.Lock()
	defer r.filterMu.Unlock()
	r.relayFilter = filter
}

func (r *RelayClient) Run(ctx context.Context) {
	defer appLog.RecoverPanic("relay-client-run")
	runCtx, cancel := context.WithCancel(ctx)
	r.cancel = cancel
	initial := core.ParseDuration(r.cfg.ReconnectInitial, time.Second)
	maxDelay := core.ParseDuration(r.cfg.ReconnectMax, time.Minute)
	delay := initial
	target := net.JoinHostPort(r.cfg.Host, r.cfg.Port)
	log.Info().Str("target", target).Msg("relay client started")
	for {
		select {
		case <-runCtx.Done():
			return
		default:
		}
		conn, err := net.Dial("tcp", target)
		if err != nil {
			r.metrics.SetConnected(false)
			r.metrics.IncReconnects()
			log.Warn().Err(err).Dur("next_retry", delay).Str("target", target).Msg("relay connect failed")
			select {
			case <-runCtx.Done():
				return
			case <-time.After(delay):
			}
			delay = minDuration(delay*2, maxDelay)
			continue
		}
		r.metrics.SetConnected(true)
		delay = initial
		log.Info().Str("target", target).Msg("relay connected")
		r.setConn(conn)
		err = r.processConnection(runCtx, conn)
		r.metrics.SetConnected(false)
		r.clearConn(conn)
		_ = conn.Close()
		if err != nil {
			r.metrics.IncReconnects()
			log.Warn().Err(err).Str("target", target).Msg("relay connection lost")
		}
	}
}

func (r *RelayClient) Stop() {
	if r.cancel != nil {
		r.cancel()
	}
	r.connMu.Lock()
	if r.conn != nil {
		_ = r.conn.Close()
	}
	r.connMu.Unlock()
}

func (r *RelayClient) processConnection(ctx context.Context, conn net.Conn) error {
	defer appLog.RecoverPanic("relay-client-connection")
	defer r.clearConn(conn)
	replyBuf := make([]byte, 8)
	for {
		select {
		case <-ctx.Done():
			return nil
		case msg, ok := <-r.queue.Reader():
			if !ok {
				return nil
			}
			r.filterMu.RLock()
			filter := r.relayFilter
			r.filterMu.RUnlock()
			if shouldBypassRelay(filter, extractDeviceID(msg.Payload), extractGroupNo(msg.Payload), extractZoneNo(msg.Payload), extractEventCode(msg.Payload)) {
				msg.ReplyCh <- core.DeliveryResult{Status: true}
				continue
			}
			_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if _, err := conn.Write(msg.Payload); err != nil {
				msg.ReplyCh <- core.DeliveryResult{Status: false}
				return err
			}
			_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
			n, err := conn.Read(replyBuf)
			if err != nil {
				msg.ReplyCh <- core.DeliveryResult{Status: false}
				return err
			}
			okAck := n > 0 && replyBuf[0] == byte(0x06)
			if okAck {
				r.metrics.IncAccepted()
			} else {
				r.metrics.IncRejected()
			}
			msg.ReplyCh <- core.DeliveryResult{Status: okAck}
		}
	}
}

func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

func (r *RelayClient) setConn(conn net.Conn) {
	r.connMu.Lock()
	r.conn = conn
	r.connMu.Unlock()
}

func (r *RelayClient) clearConn(conn net.Conn) {
	r.connMu.Lock()
	if r.conn == conn {
		r.conn = nil
	}
	r.connMu.Unlock()
}
