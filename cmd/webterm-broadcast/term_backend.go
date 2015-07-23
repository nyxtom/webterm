package main

import "github.com/nyxtom/broadcast/server"

type TermBackend struct {
	server.Backend

	app *server.BroadcastServer
}

func RegisterTermBackend(app *server.BroadcastServer) (server.Backend, error) {
	backend := new(TermBackend)
	backend.app = app
	return backend, nil
}

func (b *TermBackend) Load() error {
	return nil
}

func (b *TermBackend) Unload() error {
	return nil
}
