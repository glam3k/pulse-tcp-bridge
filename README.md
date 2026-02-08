# pulse-tcp-bridge

A tiny PulseAudio capture daemon that exposes PCM audio over a TCP socket. Intended to run inside headless VMs (e.g., Rackcore sandboxes) so another service can proxy audio to a browser.

## Usage

```
pulse-tcp-bridge --listen=:5903 --pa-device=@DEFAULT_SOURCE@
```

The binary connects to the local PulseAudio server, records 16-bit little endian PCM, and streams it to every TCP client.

## Build

Requires Linux with PulseAudio headers (`libpulse-simple-dev`). Building on macOS
is not supported because PulseAudio doesn't ship the required headers there.
On a Linux host:

```
GO111MODULE=on go build ./...
```

Cross-compiling from macOS is not supported because pulse-simple depends on system headers.
