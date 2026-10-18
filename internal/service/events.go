package service

import (
	"context"
	"sync"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// Events emite eventos hacia el frontend (progreso de exportar/importar). Sin contexto (tests) no hace nada.
type Events struct {
	mu  sync.RWMutex
	ctx context.Context
}

func (e *Events) SetContext(ctx context.Context) {
	e.mu.Lock()
	e.ctx = ctx
	e.mu.Unlock()
}

func (e *Events) Emit(name string, data any) {
	e.mu.RLock()
	ctx := e.ctx
	e.mu.RUnlock()
	if ctx != nil {
		runtime.EventsEmit(ctx, name, data)
	}
}

// jobs registra las operaciones largas en curso para poder cancelarlas por id.
type jobs struct {
	mu sync.Mutex
	m  map[string]context.CancelFunc
}

func (j *jobs) start(id string) (context.Context, func()) {
	ctx, cancel := context.WithCancel(context.Background())
	j.mu.Lock()
	if j.m == nil {
		j.m = map[string]context.CancelFunc{}
	}
	j.m[id] = cancel
	j.mu.Unlock()
	return ctx, func() {
		cancel()
		j.mu.Lock()
		delete(j.m, id)
		j.mu.Unlock()
	}
}

func (j *jobs) cancel(id string) {
	j.mu.Lock()
	c := j.m[id]
	j.mu.Unlock()
	if c != nil {
		c()
	}
}
