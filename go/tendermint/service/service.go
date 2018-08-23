// Package service provides the tendermint service interface.
package service

import (
	tmcli "github.com/tendermint/tendermint/rpc/client"

	"github.com/oasislabs/ekiden/go/common/service"
	"github.com/oasislabs/ekiden/go/tendermint/abci"
)

// TendermintService provides Tendermint access to Ekiden backends.
type TendermintService interface {
	service.BackgroundService

	// GetClient creates a Tendermint client that talks to this service instance.
	GetClient() tmcli.Client

	// RegisterApplication registers an ABCI multiplexer application
	// with this service instance.
	RegisterApplication(abci.Application) error

	// ForceInitialize force-initializes the Tendermint service iff
	// it has not been started.  Otherwise the routine has no effect
	// and will succeed.
	ForceInitialize() error
}