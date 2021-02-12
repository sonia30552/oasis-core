package scheduler

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/abci/types"

	"github.com/oasisprotocol/oasis-core/go/common"
	"github.com/oasisprotocol/oasis-core/go/common/crypto/signature"
	"github.com/oasisprotocol/oasis-core/go/common/logging"
	"github.com/oasisprotocol/oasis-core/go/common/node"
	"github.com/oasisprotocol/oasis-core/go/consensus/tendermint/api"
	schedulerState "github.com/oasisprotocol/oasis-core/go/consensus/tendermint/apps/scheduler/state"
	registry "github.com/oasisprotocol/oasis-core/go/registry/api"
	scheduler "github.com/oasisprotocol/oasis-core/go/scheduler/api"
	staking "github.com/oasisprotocol/oasis-core/go/staking/api"
)

func TestDiffValidators(t *testing.T) {
	logger := logging.GetLogger("TestDiffValidators")
	powerOne := map[signature.PublicKey]int64{
		{}: 1,
	}
	powerTwo := map[signature.PublicKey]int64{
		{}: 2,
	}
	for _, tt := range []struct {
		msg     string
		current map[signature.PublicKey]int64
		pending map[signature.PublicKey]int64
		result  []types.ValidatorUpdate
	}{
		{
			msg:     "equal",
			current: powerOne,
			pending: powerOne,
			result:  nil,
		},
		{
			msg:     "add",
			current: nil,
			pending: powerOne,
			result: []types.ValidatorUpdate{
				api.PublicKeyToValidatorUpdate(signature.PublicKey{}, 1),
			},
		},
		{
			msg:     "change",
			current: powerOne,
			pending: powerTwo,
			result: []types.ValidatorUpdate{
				api.PublicKeyToValidatorUpdate(signature.PublicKey{}, 2),
			},
		},
		{
			msg:     "remove",
			current: powerOne,
			pending: nil,
			result: []types.ValidatorUpdate{
				api.PublicKeyToValidatorUpdate(signature.PublicKey{}, 0),
			},
		},
	} {
		require.Equal(t, tt.result, diffValidators(logger, tt.current, tt.pending), tt.msg)
	}
}

func TestElectCommittee(t *testing.T) {
	if testing.Verbose() {
		// Initialize logging to aid debugging.
		_ = logging.Initialize(os.Stdout, logging.FmtLogfmt, logging.LevelDebug, map[string]logging.Level{})
	}

	require := require.New(t)

	now := time.Unix(1580461674, 0)
	appState := api.NewMockApplicationState(&api.MockApplicationStateConfig{})
	ctx := appState.NewContext(api.ContextBeginBlock, now)
	defer ctx.Close()

	app := &schedulerApplication{
		state: appState,
	}

	schedState := schedulerState.NewMutableState(ctx.State())

	mockBeacon := []byte("mock random beacon mock random beacon mock random beacon!!")

	rtID1 := common.NewTestNamespaceFromSeed([]byte("runtime 1"), 0)
	rtID2 := common.NewTestNamespaceFromSeed([]byte("runtime 2"), 0)

	nodeID1 := signature.NewPublicKey("0000000000000000000000000000000000000000000000000000000000000001")
	nodeID2 := signature.NewPublicKey("0000000000000000000000000000000000000000000000000000000000000002")
	nodeID3 := signature.NewPublicKey("0000000000000000000000000000000000000000000000000000000000000003")

	entityID1 := signature.NewPublicKey("1000000000000000000000000000000000000000000000000000000000000001")
	entityID2 := signature.NewPublicKey("1000000000000000000000000000000000000000000000000000000000000002")

	for _, tc := range []struct { //nolint: maligned
		msg               string
		kind              scheduler.CommitteeKind
		nodes             []*node.Node
		validatorEntities map[staking.Address]bool
		rt                registry.Runtime
		shouldElect       bool
	}{
		{
			"executor: should not elect when everything is empty",
			scheduler.KindComputeExecutor,
			[]*node.Node{},
			map[staking.Address]bool{},
			registry.Runtime{},
			false,
		},
		{
			"executor: should elect single node with no constraints",
			scheduler.KindComputeExecutor,
			[]*node.Node{
				{
					ID: nodeID1,
					Runtimes: []*node.Runtime{
						{ID: rtID1}, // Matching runtime ID.
					},
					Roles: node.RoleComputeWorker,
				},
				{
					ID:       nodeID2,
					Runtimes: []*node.Runtime{},  // No runtimes.
					Roles:    node.RoleValidator, // Validator.
				},
				{
					ID: nodeID3,
					Runtimes: []*node.Runtime{
						{ID: rtID2}, // Different runtime ID.
					},
					Roles: node.RoleComputeWorker,
				},
			},
			map[staking.Address]bool{},
			registry.Runtime{
				ID:   rtID1,
				Kind: registry.KindCompute,
				Executor: registry.ExecutorParameters{
					GroupSize:       1,
					GroupBackupSize: 0,
				},
			},
			true,
		},
		{
			"executor: only node not for the correct runtime",
			scheduler.KindComputeExecutor,
			[]*node.Node{
				{
					ID:       nodeID1,
					Runtimes: []*node.Runtime{{ID: rtID2}},
					Roles:    node.RoleComputeWorker,
				},
			},
			map[staking.Address]bool{},
			registry.Runtime{
				ID:   rtID1,
				Kind: registry.KindCompute,
				Executor: registry.ExecutorParameters{
					GroupSize:       1,
					GroupBackupSize: 0,
				},
			},
			false,
		},
		{
			"executor: not enough eligible nodes",
			scheduler.KindComputeExecutor,
			[]*node.Node{
				{
					ID: nodeID1,
					Runtimes: []*node.Runtime{
						{ID: rtID1}, // Matching runtime ID.
					},
					Roles: node.RoleComputeWorker,
				},
				{
					ID:       nodeID2,
					Runtimes: []*node.Runtime{},  // No runtimes.
					Roles:    node.RoleValidator, // Validator.
				},
				{
					ID: nodeID3,
					Runtimes: []*node.Runtime{
						{ID: rtID2}, // Different runtime ID.
					},
					Roles: node.RoleComputeWorker,
				},
			},
			map[staking.Address]bool{},
			registry.Runtime{
				ID:   rtID1,
				Kind: registry.KindCompute,
				Executor: registry.ExecutorParameters{
					GroupSize:       2,
					GroupBackupSize: 0,
				},
			},
			false,
		},
		{
			"executor: enough eligible nodes",
			scheduler.KindComputeExecutor,
			[]*node.Node{
				{
					ID: nodeID1,
					Runtimes: []*node.Runtime{
						{ID: rtID1}, // Matching runtime ID.
					},
					Roles: node.RoleComputeWorker,
				},
				{
					ID:       nodeID2,
					Runtimes: []*node.Runtime{},  // No runtimes.
					Roles:    node.RoleValidator, // Validator.
				},
				{
					ID: nodeID3,
					Runtimes: []*node.Runtime{
						{ID: rtID1}, // Matching runtime ID.
					},
					Roles: node.RoleComputeWorker,
				},
			},
			map[staking.Address]bool{},
			registry.Runtime{
				ID:   rtID1,
				Kind: registry.KindCompute,
				Executor: registry.ExecutorParameters{
					GroupSize:       2,
					GroupBackupSize: 0,
				},
			},
			true,
		},
		{
			"executor: satisfied min pool size constraint",
			scheduler.KindComputeExecutor,
			[]*node.Node{
				{
					ID: nodeID1,
					Runtimes: []*node.Runtime{
						{ID: rtID1}, // Matching runtime ID.
					},
					Roles: node.RoleComputeWorker,
				},
				{
					ID:       nodeID2,
					Runtimes: []*node.Runtime{},  // No runtimes.
					Roles:    node.RoleValidator, // Validator.
				},
				{
					ID: nodeID3,
					Runtimes: []*node.Runtime{
						{ID: rtID1}, // Matching runtime ID.
					},
					Roles: node.RoleComputeWorker,
				},
			},
			map[staking.Address]bool{},
			registry.Runtime{
				ID:   rtID1,
				Kind: registry.KindCompute,
				Executor: registry.ExecutorParameters{
					GroupSize:       2,
					GroupBackupSize: 0,
				},
				Constraints: map[scheduler.CommitteeKind]map[scheduler.Role]registry.SchedulingConstraints{
					scheduler.KindComputeExecutor: {
						scheduler.RoleWorker: {
							MinPoolSize: &registry.MinPoolSizeConstraint{
								Limit: 2,
							},
						},
					},
				},
			},
			true,
		},
		{
			"executor: unsatisfied min pool size constraint",
			scheduler.KindComputeExecutor,
			[]*node.Node{
				{
					ID: nodeID1,
					Runtimes: []*node.Runtime{
						{ID: rtID1}, // Matching runtime ID.
					},
					Roles: node.RoleComputeWorker,
				},
				{
					ID:       nodeID2,
					Runtimes: []*node.Runtime{},  // No runtimes.
					Roles:    node.RoleValidator, // Validator.
				},
				{
					ID: nodeID3,
					Runtimes: []*node.Runtime{
						{ID: rtID1}, // Matching runtime ID.
					},
					Roles: node.RoleComputeWorker,
				},
			},
			map[staking.Address]bool{},
			registry.Runtime{
				ID:   rtID1,
				Kind: registry.KindCompute,
				Executor: registry.ExecutorParameters{
					GroupSize:       2,
					GroupBackupSize: 0,
				},
				Constraints: map[scheduler.CommitteeKind]map[scheduler.Role]registry.SchedulingConstraints{
					scheduler.KindComputeExecutor: {
						scheduler.RoleWorker: {
							MinPoolSize: &registry.MinPoolSizeConstraint{
								Limit: 3,
							},
						},
					},
				},
			},
			false,
		},
		{
			"executor: unsatisfied validator set constraint",
			scheduler.KindComputeExecutor,
			[]*node.Node{
				{
					ID: nodeID1,
					Runtimes: []*node.Runtime{
						{ID: rtID1}, // Matching runtime ID.
					},
					Roles: node.RoleComputeWorker,
				},
				{
					ID:       nodeID2,
					Runtimes: []*node.Runtime{},  // No runtimes.
					Roles:    node.RoleValidator, // Validator.
				},
				{
					ID: nodeID3,
					Runtimes: []*node.Runtime{
						{ID: rtID1}, // Matching runtime ID.
					},
					Roles: node.RoleComputeWorker,
				},
			},
			map[staking.Address]bool{},
			registry.Runtime{
				ID:   rtID1,
				Kind: registry.KindCompute,
				Executor: registry.ExecutorParameters{
					GroupSize:       2,
					GroupBackupSize: 0,
				},
				Constraints: map[scheduler.CommitteeKind]map[scheduler.Role]registry.SchedulingConstraints{
					scheduler.KindComputeExecutor: {
						scheduler.RoleWorker: {
							ValidatorSet: &registry.ValidatorSetConstraint{},
						},
					},
				},
			},
			false,
		},
		{
			"executor: satisfied validator set constraint",
			scheduler.KindComputeExecutor,
			[]*node.Node{
				{
					ID:       nodeID1,
					EntityID: entityID1,
					Runtimes: []*node.Runtime{
						{ID: rtID1}, // Matching runtime ID.
					},
					Roles: node.RoleComputeWorker,
				},
				{
					ID:       nodeID2,
					EntityID: entityID1,
					Runtimes: []*node.Runtime{},  // No runtimes.
					Roles:    node.RoleValidator, // Validator.
				},
				{
					ID:       nodeID3,
					EntityID: entityID2,
					Runtimes: []*node.Runtime{
						{ID: rtID1}, // Matching runtime ID.
					},
					Roles: node.RoleComputeWorker,
				},
			},
			map[staking.Address]bool{
				staking.NewAddress(entityID1): true,
				staking.NewAddress(entityID2): true,
			},
			registry.Runtime{
				ID:   rtID1,
				Kind: registry.KindCompute,
				Executor: registry.ExecutorParameters{
					GroupSize:       2,
					GroupBackupSize: 0,
				},
				Constraints: map[scheduler.CommitteeKind]map[scheduler.Role]registry.SchedulingConstraints{
					scheduler.KindComputeExecutor: {
						scheduler.RoleWorker: {
							ValidatorSet: &registry.ValidatorSetConstraint{},
						},
					},
				},
			},
			true,
		},
		{
			"executor: unsatisfied max nodes per entity constraint",
			scheduler.KindComputeExecutor,
			[]*node.Node{
				{
					ID:       nodeID1,
					EntityID: entityID1,
					Runtimes: []*node.Runtime{
						{ID: rtID1}, // Matching runtime ID.
					},
					Roles: node.RoleComputeWorker,
				},
				{
					ID:       nodeID2,
					EntityID: entityID1,
					Runtimes: []*node.Runtime{},  // No runtimes.
					Roles:    node.RoleValidator, // Validator.
				},
				{
					ID:       nodeID3,
					EntityID: entityID1,
					Runtimes: []*node.Runtime{
						{ID: rtID1}, // Matching runtime ID.
					},
					Roles: node.RoleComputeWorker,
				},
			},
			map[staking.Address]bool{},
			registry.Runtime{
				ID:   rtID1,
				Kind: registry.KindCompute,
				Executor: registry.ExecutorParameters{
					GroupSize:       2,
					GroupBackupSize: 0,
				},
				Constraints: map[scheduler.CommitteeKind]map[scheduler.Role]registry.SchedulingConstraints{
					scheduler.KindComputeExecutor: {
						scheduler.RoleWorker: {
							MaxNodes: &registry.MaxNodesConstraint{
								Limit: 1,
							},
						},
					},
				},
			},
			false,
		},
		{
			"executor: satisfied max nodes per entity constraint",
			scheduler.KindComputeExecutor,
			[]*node.Node{
				{
					ID:       nodeID1,
					EntityID: entityID1,
					Runtimes: []*node.Runtime{
						{ID: rtID1}, // Matching runtime ID.
					},
					Roles: node.RoleComputeWorker,
				},
				{
					ID:       nodeID2,
					EntityID: entityID1,
					Runtimes: []*node.Runtime{},  // No runtimes.
					Roles:    node.RoleValidator, // Validator.
				},
				{
					ID:       nodeID3,
					EntityID: entityID1,
					Runtimes: []*node.Runtime{
						{ID: rtID1}, // Matching runtime ID.
					},
					Roles: node.RoleComputeWorker,
				},
			},
			map[staking.Address]bool{},
			registry.Runtime{
				ID:   rtID1,
				Kind: registry.KindCompute,
				Executor: registry.ExecutorParameters{
					GroupSize:       2,
					GroupBackupSize: 0,
				},
				Constraints: map[scheduler.CommitteeKind]map[scheduler.Role]registry.SchedulingConstraints{
					scheduler.KindComputeExecutor: {
						scheduler.RoleWorker: {
							MaxNodes: &registry.MaxNodesConstraint{
								Limit: 2,
							},
						},
					},
				},
			},
			true,
		},
		{
			"storage: should elect single node with no constraints",
			scheduler.KindStorage,
			[]*node.Node{
				{
					ID: nodeID1,
					Runtimes: []*node.Runtime{
						{ID: rtID1}, // Matching runtime ID.
					},
					Roles: node.RoleStorageWorker,
				},
				{
					ID:       nodeID2,
					Runtimes: []*node.Runtime{},  // No runtimes.
					Roles:    node.RoleValidator, // Validator.
				},
				{
					ID: nodeID3,
					Runtimes: []*node.Runtime{
						{ID: rtID2}, // Different runtime ID.
					},
					Roles: node.RoleStorageWorker,
				},
			},
			map[staking.Address]bool{},
			registry.Runtime{
				ID:   rtID1,
				Kind: registry.KindCompute,
				Storage: registry.StorageParameters{
					GroupSize: 1,
				},
			},
			true,
		},
	} {
		err := app.electCommittee(ctx, 1, mockBeacon, nil, nil, tc.validatorEntities, &tc.rt, tc.nodes, tc.kind)
		require.NoError(err, "committee election should not fail")

		c, err := schedState.Committee(ctx, tc.kind, tc.rt.ID)
		require.NoError(err, "Committee")
		if !tc.shouldElect {
			require.Nil(c, "Committee should not have been elected (%s)", tc.msg)
			continue
		}

		require.NotNil(c, "Committee should have been elected (%s)", tc.msg)
	}
}
