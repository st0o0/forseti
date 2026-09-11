package session

import (
	"fmt"
	"log/slog"
	"sync"

	"github.com/st0o0/forseti/internal/config"
	"github.com/st0o0/forseti/internal/pihole"
)

type PoolCallbacks struct {
	OnNewSession func(target string)
	OnReauth     func(target string)
	OnClose      func()
}

type Pool struct {
	mu        sync.Mutex
	clients   map[string]*pihole.Client
	callbacks PoolCallbacks
}

func NewPool() *Pool {
	return &Pool{
		clients: make(map[string]*pihole.Client),
	}
}

func (p *Pool) SetCallbacks(cb PoolCallbacks) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.callbacks = cb
}

func (p *Pool) Get(target config.Target) (*pihole.Client, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if c, ok := p.clients[target.Name]; ok && c.HasSession() {
		slog.Debug("reusing session", "target", target.Name)
		return c, nil
	}

	slog.Debug("creating session", "target", target.Name)
	c := pihole.NewClient(target.URL, target.Password)
	if p.callbacks.OnReauth != nil {
		name := target.Name
		c.OnReauth = func() {
			slog.Debug("session re-authenticated", "target", name)
			p.callbacks.OnReauth(name)
		}
	}
	if err := c.Login(); err != nil {
		return nil, fmt.Errorf("session pool login %s: %w", target.Name, err)
	}
	p.clients[target.Name] = c
	if p.callbacks.OnNewSession != nil {
		p.callbacks.OnNewSession(target.Name)
	}
	return c, nil
}

func (p *Pool) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	slog.Debug("closing all sessions", "count", len(p.clients))
	var firstErr error
	for name, c := range p.clients {
		if err := c.Close(); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("close session %s: %w", name, err)
		}
	}
	p.clients = make(map[string]*pihole.Client)
	if p.callbacks.OnClose != nil {
		p.callbacks.OnClose()
	}
	return firstErr
}
