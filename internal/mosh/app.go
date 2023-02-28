package mosh

import (
	"context"
	"fmt"
	"net"
	"os"
)

type AppType int

const (
	AppTypeClient AppType = iota
	AppTypeServer
)

type App struct {
	appType AppType

	apiKey     string
	remoteAddr string
}

func NewApp(apiKey, remoteAddr string, appType AppType) *App {
	return &App{
		appType: appType,

		apiKey:     apiKey,
		remoteAddr: remoteAddr,
	}
}

func (a *App) Run(ctx context.Context) error {
	if a.appType == AppTypeServer {
		return a.runServer(ctx)
	}
	return a.runClient(ctx)
}

func (a *App) runServer(ctx context.Context) (err error) {
	moshServerClientCh, relayServerClientCh := make(chan []byte), make(chan []byte)
	errs := make(chan error, 1)

	serverProcess := newServerProcess()
	defer safeStop(serverProcess, &err)

	fmt.Println("Ensuring compatibility...")
	installer := newInstaller(serverProcess)
	if err := installer.ensureCompatible(ctx); err != nil {
		return fmt.Errorf("failed to ensure compatibility: %w", err)
	}

	fmt.Println("Starting mosh server process...")
	if err := serverProcess.run(ctx); err != nil {
		return fmt.Errorf("failed to run server process: %w", err)
	}

	fmt.Println("Getting connection details...")
	port, moshKey, err := serverProcess.connDetails()
	if err != nil {
		return fmt.Errorf("failed to get mosh key: %w", err)
	}
	remoteAddr, err := net.ResolveUDPAddr("udp", a.remoteAddr)
	if err != nil {
		return fmt.Errorf("failed to resolve remote address: %w", err)
	}

	fmt.Println("Starting relay server client...")
	client := newRelayServerClient(a.apiKey, moshKey, remoteAddr, moshServerClientCh, relayServerClientCh)
	defer safeStop(client, &err)
	go func() {
		if err := client.connect(ctx); err != nil {
			errs <- fmt.Errorf("failed to connect to relay server: %w", err)
		}
	}()

	fmt.Println("Starting mosh server client...")
	serverClient := newMoshServerClient(port, relayServerClientCh, moshServerClientCh)
	defer safeStop(serverClient, &err)
	go func() {
		if err := serverClient.connect(ctx); err != nil {
			errs <- fmt.Errorf("failed to connect to mosh server: %w", err)
		}
	}()

	fmt.Println("Printing mosh key...")
	if _, err := fmt.Fprintf(os.Stdout, "%s %s\n", moshKeyPrefix, moshKey); err != nil {
		return fmt.Errorf("failed to print mosh key: %w", err)
	}

	fmt.Println("Running...")
	return await(ctx, errs)
}

func (a *App) runClient(ctx context.Context) (err error) {
	moshClientServerCh, relayServerClientCh := make(chan []byte), make(chan []byte)
	errs := make(chan error, 4)

	moshKey := os.Getenv("MOSH_KEY")
	if moshKey == "" {
		fmt.Println("Starting codespace process...")
		codespaceProcess := newCodespaceProcess(a.apiKey, a.remoteAddr)
		defer safeStop(codespaceProcess, &err)
		go func() {
			if err := codespaceProcess.start(ctx); err != nil {
				errs <- fmt.Errorf("failed to start codespace process: %w", err)
			}
		}()

		moshKey, err = codespaceProcess.moshKey(ctx)
		if err != nil {
			return fmt.Errorf("failed to get mosh key: %w", err)
		}
	}

	remoteAddr, err := net.ResolveUDPAddr("udp", a.remoteAddr)
	if err != nil {
		return fmt.Errorf("failed to resolve remote address: %w", err)
	}

	relayServerClient := newRelayServerClient(a.apiKey, moshKey, remoteAddr, moshClientServerCh, relayServerClientCh)
	defer safeStop(relayServerClient, &err)

	clientServer := newMoshClientServer(relayServerClientCh, moshClientServerCh)
	defer safeStop(clientServer, &err)

	fmt.Println("Starting client server...")
	go func() {
		if err := clientServer.listen(ctx); err != nil {
			errs <- fmt.Errorf("failed to listen: %w", err)
		}
	}()

	fmt.Println("Connecting to relay server...")
	go func() {
		if err := relayServerClient.connect(ctx); err != nil {
			errs <- fmt.Errorf("failed to connect: %w", err)
		}
	}()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				fmt.Println("Waiting for client server to start...")
				if addr := clientServer.localAddr(); addr != nil {
					fmt.Printf("Starting mosh client with key: %s...\n", moshKey)
					if err := a.startMoshClient(ctx, moshKey, addr); err != nil {
						errs <- fmt.Errorf("failed to start mosh client: %w", err)
						return
					}
					errs <- nil // successful exit
					return
				}
			}
		}
	}()

	return await(ctx, errs)
}

func (a *App) startMoshClient(ctx context.Context, moshKey string, addr *net.UDPAddr) (err error) {
	localProcess := newClientProcess(moshKey, addr)
	defer safeStop(localProcess, &err)

	fmt.Println("Ensuring compatibility...")
	installer := newInstaller(localProcess)
	if err := installer.ensureCompatible(ctx); err != nil {
		return fmt.Errorf("failed to ensure compatibility: %w", err)
	}

	fmt.Println("Starting mosh client process...")
	return localProcess.start(ctx)
}

type stopper interface {
	stop() error
}

func safeStop(s stopper, err *error) {
	if e := s.stop(); e != nil && *err == nil {
		*err = e
	}
}
