package main

import (
	"context"

	"github.com/SrChr1s/pluton-sql/internal/config"
	"github.com/SrChr1s/pluton-sql/internal/service"
	"github.com/SrChr1s/pluton-sql/internal/session"
)

// App agrupa los servicios expuestos al frontend.
type App struct {
	ctx         context.Context
	sess        *session.Manager
	Connections *service.ConnectionService
	Schema      *service.SchemaService
	Query       *service.QueryService
}

func NewApp() (*App, error) {
	store, err := config.NewStore("")
	if err != nil {
		return nil, err
	}
	sess := session.NewManager(store)
	return &App{
		sess:        sess,
		Connections: service.NewConnectionService(store, sess),
		Schema:      service.NewSchemaService(sess),
		Query:       service.NewQueryService(sess),
	}, nil
}

func (a *App) startup(ctx context.Context) { a.ctx = ctx }

func (a *App) shutdown(context.Context) { a.sess.CloseAll() }
