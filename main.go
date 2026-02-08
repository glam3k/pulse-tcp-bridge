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
	"syscall"
	"time"

	libpulse "github.com/mesilliac/pulse-simple"
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

	sampleSpec := libpulse.SampleSpec{
		Format:   libpulse.SampleFormatS16LE,
		Rate:     uint32(*rate),
		Channels: uint8(*channels),
	}

	stream, err := libpulse.NewRecord(*paServer, "pulse-tcp-bridge", &sampleSpec, *paDevice)
	if err != nil {
		log.Fatalf("pulse connect failed: %v", err)
	}
	defer stream.Free()

	ln, err := net.Listen("tcp", *listenAddr)
	if err != nil {
		log.Fatalf("listen failed: %v", err)
	}
	defer ln.Close()
	log.Printf("pulse-tcp-bridge listening on %s", *listenAddr)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		ln.Close()
		stream.Flush()
		os.Exit(0)
	}()

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
		go handle(conn, stream, *bufferMs)
	}
}

func handle(conn net.Conn, stream *libpulse.Stream, bufferMs int) {
	defer conn.Close()

	rate := int(stream.GetSampleSpec().Rate)
	channels := int(stream.GetSampleSpec().Channels)
	frames := bufferMs * rate * channels / 1000
	buf := make([]byte, frames*2) // 16-bit samples

	writer := bufio.NewWriter(conn)
	for {
		if err := stream.Read(buf); err != nil {
			if err != io.EOF {
				log.Printf("pulse read error: %v", err)
			}
			return
		}
		if _, err := writer.Write(buf); err != nil {
			log.Printf("write error: %v", err)
			return
		}
		if err := writer.Flush(); err != nil {
			log.Printf("flush error: %v", err)
			return
		}
	}
}
