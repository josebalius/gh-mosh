package mosh

import (
	"context"
	"fmt"
	"net"
)

type moshServerClient struct {
	sender, receiver chan []byte
	port             int64

	conn *net.UDPConn
}

func newMoshServerClient(port int64, sender, receiver chan []byte) *moshServerClient {
	return &moshServerClient{
		sender:   sender,
		receiver: receiver,
		port:     port,
	}
}

func (m *moshServerClient) connect(ctx context.Context) error {
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("127.0.0.1:%d", m.port))
	if err != nil {
		return err
	}
	conn, err := net.DialUDP("udp", nil, addr)
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

	return nil
}

func (m *moshServerClient) stop() error {
	return m.conn.Close()
}

func (m *moshServerClient) read(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			p := make([]byte, maxPacketSize)
			n, _, err := m.conn.ReadFromUDP(p)
			if err != nil {
				return fmt.Errorf("failed to read from udp: %w", err)
			}
			m.sender <- p[:n]
		}
	}
}

func (m *moshServerClient) write(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case p := <-m.receiver:
			if _, err := m.conn.Write(p); err != nil {
				return fmt.Errorf("failed to write to udp: %w", err)
			}
		}
	}
}
