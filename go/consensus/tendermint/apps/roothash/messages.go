package roothash

import (
	"github.com/oasisprotocol/oasis-core/go/common/cbor"
	"github.com/oasisprotocol/oasis-core/go/common/errors"
	tmapi "github.com/oasisprotocol/oasis-core/go/consensus/tendermint/api"
	roothashApi "github.com/oasisprotocol/oasis-core/go/consensus/tendermint/apps/roothash/api"
	roothash "github.com/oasisprotocol/oasis-core/go/roothash/api"
	"github.com/oasisprotocol/oasis-core/go/roothash/api/message"
	staking "github.com/oasisprotocol/oasis-core/go/staking/api"
)

func (app *rootHashApplication) processRuntimeMessages(
	ctx *tmapi.Context,
	rtState *roothash.RuntimeState,
	msgs []message.Message,
) error {
	ctx = ctx.WithCallerAddress(staking.NewRuntimeAddress(rtState.Runtime.ID))
	defer ctx.Close()

	switch ctx.IsSimulation() {
	case false:
		// Delivery -- gas was already accounted for.
		ctx.SetGasAccountant(tmapi.NewNopGasAccountant())
	case true:
		// Gas estimation -- use parent gas accountant, discard state updates (there shouldn't be
		// any as we are using simulation mode, but make sure).
		cp := ctx.StartCheckpoint()
		defer cp.Close()
	}

	for i, msg := range msgs {
		ctx.Logger().Debug("dispatching runtime message",
			"index", i,
			"body", msg,
		)

		var err error
		switch {
		case msg.Staking != nil:
			err = app.md.Publish(ctx, roothashApi.RuntimeMessageStaking, msg.Staking)
		case msg.Registry != nil:
			err = app.md.Publish(ctx, roothashApi.RuntimeMessageRegistry, msg.Registry)
		default:
			// Unsupported message.
			err = roothash.ErrInvalidArgument
		}
		if err != nil {
			ctx.Logger().Warn("failed to process runtime message",
				"err", err,
				"runtime_id", rtState.Runtime.ID,
				"msg_index", i,
			)
		}

		// Make sure somebody actually handled the message, otherwise treat as unsupported.
		if err == tmapi.ErrNoSubscribers {
			err = roothash.ErrInvalidArgument
		}

		module, code := errors.Code(err)
		evV := ValueMessage{
			ID: rtState.Runtime.ID,
			Event: roothash.MessageEvent{
				Index:  uint32(i),
				Module: module,
				Code:   code,
			},
		}
		ctx.EmitEvent(
			tmapi.NewEventBuilder(app.Name()).
				Attribute(KeyMessage, cbor.Marshal(evV)).
				Attribute(KeyRuntimeID, ValueRuntimeID(evV.ID)),
		)
	}
	return nil
}
