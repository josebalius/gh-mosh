package mosh

import (
	"context"
	"fmt"
	"net"
)

type moshClientServer struct {
	sender, receiver chan []byte

	conn     *net.UDPConn
	lastaddr *net.UDPAddr
}

func newMoshClientServer(sender, receiver chan []byte) *moshClientServer {
	return &moshClientServer{sender: sender, receiver: receiver}
}

func (m *moshClientServer) listen(ctx context.Context) error {
	conn, err := net.ListenUDP("udp", nil)
	if err != nil {
		return fmt.Errorf("failed to dial udp: %w", err)
	}
	m.conn = conn

	errs := make(chan error, 2)
	go func() {
		if err := m.read(ctx); err != nil {
			errs <- fmt.Errorf("failed to read: %w", err)
		}
	}()

	go func() {
		if err := m.write(ctx); err != nil {
			errs <- fmt.Errorf("failed to write: %w", err)
		}
	}()

	return await(ctx, errs)
}

func (m *moshClientServer) stop() error {
	return m.conn.Close()
}

func (m *moshClientServer) localAddr() *net.UDPAddr {
	if m.conn == nil {
		return nil
	}
	return m.conn.LocalAddr().(*net.UDPAddr)
}

func (m *moshClientServer) read(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			p := make([]byte, maxPacketSize)
			n, incomingAddr, err := m.conn.ReadFromUDP(p)
			if err != nil {
				return fmt.Errorf("failed to read from udp: %w", err)
			}
			m.lastaddr = incomingAddr
			m.sender <- p[:n]
		}
	}
}

func (m *moshClientServer) write(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case p := <-m.receiver:
			if _, err := m.conn.WriteToUDP(p, m.lastaddr); err != nil {
				return fmt.Errorf("failed to write to udp: %w", err)
			}
		}
	}
}
