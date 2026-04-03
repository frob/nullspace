// Package bridge registers data CRUD commands on TCP and IPC transports,
// bridging the file data module to non-HTTP clients.
package bridge

import (
	"context"

	"github.com/frob/nullspace/core/ipc"
	"github.com/frob/nullspace/core/tcp"
	"github.com/frob/nullspace/kernel"
	"github.com/frob/nullspace/module/data/file"
)

// Module is the data bridge kernel module. It registers data commands on
// TCP and IPC routers so that non-HTTP clients can perform CRUD operations.
type Module struct {
	fileMod *file.Module
}

// New creates a new data bridge module.
func New() *Module {
	return &Module{}
}

func (m *Module) Name() string { return "data.bridge" }

func (m *Module) Config() kernel.ModuleConfig {
	return kernel.ModuleConfig{
		Key:            "data.bridge",
		Default:        struct{}{},
		DefaultEnabled: false,
	}
}

func (m *Module) Init(k *kernel.Kernel) error {
	fileMod, err := kernel.GetResource[*file.Module](k, "data.file")
	if err != nil {
		return err
	}
	m.fileMod = fileMod

	// Discover transport routers after all modules have initialized.
	k.Hook("kernel.after_init", 10, func(_ context.Context) error {
		if tcpAdapter, err := kernel.GetResource[*tcp.Adapter](k, "transport.tcp"); err == nil {
			m.registerHandlers(tcpAdapter.Router())
			k.Logger().Info("data bridge registered on TCP")
		}
		if ipcAdapter, err := kernel.GetResource[*ipc.Adapter](k, "transport.ipc"); err == nil {
			m.registerHandlers(ipcAdapter.Router())
			k.Logger().Info("data bridge registered on IPC")
		}
		return nil
	})

	return nil
}

func (m *Module) Start(_ context.Context) error { return nil }
func (m *Module) Stop(_ context.Context) error  { return nil }

func (m *Module) registerHandlers(router *tcp.Router) {
	router.Handle("data.list", m.handleList)
	router.Handle("data.get", m.handleGet)
	router.Handle("data.create", m.handleCreate)
	router.Handle("data.update", m.handleUpdate)
	router.Handle("data.delete", m.handleDelete)
}
