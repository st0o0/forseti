package session

import (
	"fmt"
	"log/slog"
	"sync"

	"github.com/st0o0/forseti/internal/config"
	"github.com/st0o0/forseti/internal/pihole"
)

const defaultMaxConcurrent = 4

type PoolCallbacks struct {
	OnNewSession func(target string)
	OnReauth     func(target string)
	OnClose      func()
}

type Pool struct {
	mu        sync.Mutex
	clients   map[string]*pihole.Client
	gates     map[string]chan struct{}
	apiConfig map[string]config.APIConfig
	callbacks PoolCallbacks
}

func NewPool() *Pool {
	return &Pool{
		clients:   make(map[string]*pihole.Client),
		gates:     make(map[string]chan struct{}),
		apiConfig: make(map[string]config.APIConfig),
	}
}

func (p *Pool) SetCallbacks(cb PoolCallbacks) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.callbacks = cb
}

func (p *Pool) SetAPIConfig(configs map[string]config.APIConfig) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.apiConfig = configs
	for name, cfg := range configs {
		needed := cfg.MaxConcurrentOrDefault()
		if existing, ok := p.gates[name]; ok && cap(existing) == needed {
			continue
		}
		p.gates[name] = make(chan struct{}, needed)
	}
}

func (p *Pool) gate(name string) chan struct{} {
	if g, ok := p.gates[name]; ok {
		return g
	}
	maxC := defaultMaxConcurrent
	if cfg, ok := p.apiConfig[name]; ok {
		maxC = cfg.MaxConcurrentOrDefault()
	}
	g := make(chan struct{}, maxC)
	p.gates[name] = g
	return g
}

func (p *Pool) Acquire(name string) {
	p.mu.Lock()
	g := p.gate(name)
	p.mu.Unlock()
	g <- struct{}{}
}

func (p *Pool) TryAcquire(name string) bool {
	p.mu.Lock()
	g := p.gate(name)
	p.mu.Unlock()
	select {
	case g <- struct{}{}:
		return true
	default:
		return false
	}
}

func (p *Pool) Release(name string) {
	p.mu.Lock()
	g, ok := p.gates[name]
	p.mu.Unlock()
	if !ok {
		return
	}
	select {
	case <-g:
	default:
	}
}

func (p *Pool) Get(target config.Target) (*pihole.Client, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if c, ok := p.clients[target.Name]; ok && c.HasSession() {
		slog.Debug("reusing session", "target", target.Name)
		return c, nil
	}

	slog.Debug("creating session", "target", target.Name)
	c := pihole.NewClient(target.URL, target.Password, target.API.TimeoutOrDefault())
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

func (p *Pool) Invalidate(name string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if c, ok := p.clients[name]; ok {
		slog.Debug("invalidating session", "target", name)
		c.Close()
		delete(p.clients, name)
	}
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
