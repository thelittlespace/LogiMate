package gameadapter

import (
	"errors"
	"fmt"
	"net"
	"sync"
	"time"
)

type ParseFunc func([]byte, time.Time) (Frame, error)

type UDPAdapter struct {
	desc  Descriptor
	parse ParseFunc

	mu        sync.Mutex
	conn      *net.UDPConn
	stop      chan struct{}
	done      chan struct{}
	health    Health
	last      Frame
	ring      []Frame
	automatic bool
}

func NewUDPAdapter(desc Descriptor, parse ParseFunc) (*UDPAdapter, error) {
	if desc.ID == "" || parse == nil {
		return nil, errors.New("adapter id and parser are required")
	}
	if desc.DefaultPort <= 0 || desc.DefaultPort > 65535 {
		return nil, fmt.Errorf("invalid default port %d", desc.DefaultPort)
	}
	desc.Implemented = true
	return &UDPAdapter{desc: desc, parse: parse}, nil
}

func (a *UDPAdapter) Descriptor() Descriptor { return a.desc }

func (a *UDPAdapter) Start(opts StartOptions) error {
	port := opts.Port
	if port <= 0 {
		port = a.desc.DefaultPort
	}
	if port <= 0 || port > 65535 {
		return fmt.Errorf("invalid adapter port %d", port)
	}
	_ = a.Stop()
	addr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: port}
	conn, err := net.ListenUDP("udp4", addr)
	if err != nil {
		return err
	}
	stop := make(chan struct{})
	done := make(chan struct{})
	now := time.Now()
	a.mu.Lock()
	a.conn = conn
	a.stop = stop
	a.done = done
	a.automatic = opts.Automatic
	a.health = Health{Running: true, Address: fmt.Sprintf("127.0.0.1:%d", port), StartedAt: now}
	a.last = Frame{}
	a.ring = nil
	a.mu.Unlock()
	go a.readLoop(conn, stop, done)
	return nil
}

func (a *UDPAdapter) readLoop(conn *net.UDPConn, stop <-chan struct{}, done chan<- struct{}) {
	defer close(done)
	defer conn.Close()
	buf := make([]byte, 65536)
	for {
		_ = conn.SetReadDeadline(time.Now().Add(350 * time.Millisecond))
		n, remote, err := conn.ReadFromUDP(buf)
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				select {
				case <-stop:
					return
				default:
					continue
				}
			}
			a.mu.Lock()
			a.health.LastError = err.Error()
			a.health.Running = false
			a.mu.Unlock()
			return
		}
		now := time.Now()
		a.mu.Lock()
		a.health.Packets++
		a.mu.Unlock()
		if remote == nil || !remote.IP.IsLoopback() {
			a.mu.Lock()
			a.health.Invalid++
			a.mu.Unlock()
			continue
		}
		f, parseErr := a.parse(buf[:n], now)
		a.mu.Lock()
		if parseErr != nil {
			a.health.Invalid++
			a.health.LastError = parseErr.Error()
		} else {
			f.Adapter = a.desc.ID
			if f.Game == "" {
				f.Game = a.desc.Game
			}
			f.ReceivedAt = now
			a.health.Frames++
			a.health.LastError = ""
			a.health.LastFrame = now
			a.last = f
			a.ring = append(a.ring, f)
			if len(a.ring) > 256 {
				a.ring = append([]Frame(nil), a.ring[len(a.ring)-256:]...)
			}
		}
		a.mu.Unlock()
	}
}

func (a *UDPAdapter) Stop() error {
	a.mu.Lock()
	stop, done, conn := a.stop, a.done, a.conn
	a.stop, a.done, a.conn = nil, nil, nil
	a.health.Running = false
	a.mu.Unlock()
	if stop != nil {
		select {
		case <-stop:
		default:
			close(stop)
		}
	}
	if conn != nil {
		_ = conn.SetReadDeadline(time.Now())
	}
	if done != nil {
		select {
		case <-done:
		case <-time.After(time.Second):
			return errors.New("adapter stop timed out")
		}
	}
	return nil
}

func (a *UDPAdapter) Snapshot() Snapshot {
	a.mu.Lock()
	defer a.mu.Unlock()
	s := Snapshot{Descriptor: a.desc, Health: a.health, LastFrame: a.last}
	if !a.last.ReceivedAt.IsZero() && time.Since(a.last.ReceivedAt) > 750*time.Millisecond {
		s.Stale = true
	}
	return s
}

func (a *UDPAdapter) RecentFrames() []Frame {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]Frame(nil), a.ring...)
}

func (a *UDPAdapter) Automatic() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.automatic
}
