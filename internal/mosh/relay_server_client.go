package mosh

import (
	"context"
	"fmt"
	"net"
)

type relayServerClient struct {
	sender, receiver chan []byte
	apiKey, moshKey  string
	remoteAddr       *net.UDPAddr

	conn *net.UDPConn
}

func newRelayServerClient(
	apiKey, moshKey string, remoteAddr *net.UDPAddr, sender, receiver chan []byte,
) *relayServerClient {
	return &relayServerClient{
		apiKey:     apiKey,
		sender:     sender,
		receiver:   receiver,
		moshKey:    moshKey,
		remoteAddr: remoteAddr,
	}
}

func (r *relayServerClient) connect(ctx context.Context) error {
	conn, err := net.DialUDP("udp", nil, r.remoteAddr)
	if err != nil {
		return fmt.Errorf("failed to dial udp: %w", err)
	}
	r.conn = conn

	if err := r.sendConnect(ctx); err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	errs := make(chan error, 2)
	go func() {
		if err := r.read(ctx); err != nil {
			errs <- fmt.Errorf("failed to read: %w", err)
		}
	}()

	go func() {
		if err := r.write(ctx); err != nil {
			errs <- fmt.Errorf("failed to write: %w", err)
		}
	}()

	return await(ctx, errs)
}

func (r *relayServerClient) stop() error {
	return r.conn.Close()
}

func (r *relayServerClient) sendConnect(ctx context.Context) error {
	payload := []byte(connectCommand(r.apiKey, r.moshKey))
	if _, err := r.conn.Write(payload); err != nil {
		return fmt.Errorf("failed to write to udp: %w", err)
	}
	return nil
}

func connectCommand(apiKey, moshKey string) string {
	return fmt.Sprintf("CONNECT %s %s", apiKey, moshKey)
}

func (r *relayServerClient) read(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			p := make([]byte, maxPacketSize)
			n, _, err := r.conn.ReadFromUDP(p)
			if err != nil {
				return fmt.Errorf("failed to read from udp: %w", err)
			}
			r.sender <- p[:n]
		}
	}
}

func (r *relayServerClient) write(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case p := <-r.receiver:
			if _, err := r.conn.Write(p); err != nil {
				return fmt.Errorf("failed to write to udp: %w", err)
			}
		}
	}
}
