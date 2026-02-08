package main

import (
	"bufio"
	"errors"
	"flag"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	pulse "github.com/mesilliac/pulse-simple"
)

func main() {
	var (
		listenAddr = flag.String("listen", ":5903", "TCP listen address")
		paServer   = flag.String("pa-server", "", "PulseAudio server")
		paDevice   = flag.String("pa-device", "", "PulseAudio source")
		channels   = flag.Int("channels", 2, "channels")
		rate       = flag.Int("rate", 44100, "sample rate")
		bufferMs   = flag.Int("buffer-ms", 50, "buffer length (ms)")
	)
	flag.Parse()

	spec := pulse.SampleSpec{pulse.SAMPLE_S16LE, uint32(*rate), uint8(*channels)}
	if !spec.Valid() {
		log.Fatalf("invalid sample spec: %v", spec)
	}

	stream, err := pulse.NewStream(*paServer, "pulse-tcp-bridge", pulse.STREAM_RECORD, *paDevice, "audio", &spec, nil, nil)
	if err != nil {
		log.Fatalf("pulse connect failed: %v", err)
	}
	defer stream.Free()

	ln, err := net.Listen("tcp", *listenAddr)
	if err != nil {
		log.Fatalf("listen failed: %v", err)
	}
	defer ln.Close()
	log.Printf("listening on %s", *listenAddr)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	clients := newClientRegistry()

	go func() {
		<-sigCh
		ln.Close()
		clients.Close()
		stream.Flush()
		os.Exit(0)
	}()

	go broadcastLoop(stream, &spec, *bufferMs, clients)

	for {
		conn, err := ln.Accept()
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				time.Sleep(time.Second)
				continue
			}
			if !errors.Is(err, net.ErrClosed) {
				log.Printf("accept error: %v", err)
			}
			return
		}
		clients.Add(conn)
	}
}

func broadcastLoop(stream *pulse.Stream, spec *pulse.SampleSpec, bufferMs int, registry *clientRegistry) {
	frameBytes := int(spec.FrameSize())
	if frameBytes == 0 {
		frameBytes = int(spec.Channels) * 2
	}
	frames := bufferMs * int(spec.Rate) / 1000
	if frames < 1 {
		frames = 1
	}
	buf := make([]byte, frames*frameBytes)
	for {
		if _, err := stream.Read(buf); err != nil {
			if err != io.EOF {
				log.Printf("pulse read error: %v", err)
			}
			return
		}
		registry.Broadcast(buf)
	}
}

type clientRegistry struct {
	mu      sync.Mutex
	clients map[*client]struct{}
	closed  bool
}

type client struct {
	conn   net.Conn
	writer *bufio.Writer
}

func newClientRegistry() *clientRegistry {
	return &clientRegistry{clients: make(map[*client]struct{})}
}

func (r *clientRegistry) Add(conn net.Conn) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		conn.Close()
		return
	}
	c := &client{conn: conn, writer: bufio.NewWriter(conn)}
	r.clients[c] = struct{}{}
}

func (r *clientRegistry) Broadcast(data []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return
	}
	var dead []*client
	for c := range r.clients {
		if _, err := c.writer.Write(data); err != nil {
			dead = append(dead, c)
			continue
		}
		if err := c.writer.Flush(); err != nil {
			dead = append(dead, c)
		}
	}
	for _, c := range dead {
		c.conn.Close()
		delete(r.clients, c)
	}
}

func (r *clientRegistry) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return
	}
	r.closed = true
	for c := range r.clients {
		c.conn.Close()
		delete(r.clients, c)
	}
}
