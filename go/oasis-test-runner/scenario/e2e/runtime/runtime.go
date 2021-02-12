package runtime

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	beacon "github.com/oasisprotocol/oasis-core/go/beacon/api"
	"github.com/oasisprotocol/oasis-core/go/common"
	"github.com/oasisprotocol/oasis-core/go/common/cbor"
	"github.com/oasisprotocol/oasis-core/go/common/node"
	"github.com/oasisprotocol/oasis-core/go/common/sgx"
	"github.com/oasisprotocol/oasis-core/go/oasis-test-runner/cmd"
	"github.com/oasisprotocol/oasis-core/go/oasis-test-runner/env"
	"github.com/oasisprotocol/oasis-core/go/oasis-test-runner/log"
	"github.com/oasisprotocol/oasis-core/go/oasis-test-runner/oasis"
	"github.com/oasisprotocol/oasis-core/go/oasis-test-runner/scenario"
	"github.com/oasisprotocol/oasis-core/go/oasis-test-runner/scenario/e2e"
	registry "github.com/oasisprotocol/oasis-core/go/registry/api"
	runtimeClient "github.com/oasisprotocol/oasis-core/go/runtime/client/api"
	runtimeTransaction "github.com/oasisprotocol/oasis-core/go/runtime/transaction"
	scheduler "github.com/oasisprotocol/oasis-core/go/scheduler/api"
	"github.com/oasisprotocol/oasis-core/go/storage/database"
)

const (
	cfgClientBinaryDir          = "client.binary_dir"
	cfgRuntimeBinaryDirDefault  = "runtime.binary_dir.default"
	cfgRuntimeBinaryDirIntelSGX = "runtime.binary_dir.intel-sgx"
	cfgRuntimeLoader            = "runtime.loader"
	cfgTEEHardware              = "tee_hardware"
	cfgIasMock                  = "ias.mock"
	cfgEpochInterval            = "epoch.interval"
)

var (
	// RuntimeParamsDummy is a dummy instance of runtimeImpl used to register global e2e/runtime flags.
	RuntimeParamsDummy *runtimeImpl = newRuntimeImpl("", "", []string{})

	// Runtime is the basic network + client test case with runtime support.
	Runtime scenario.Scenario = newRuntimeImpl("runtime", "simple-keyvalue-client", nil)
	// RuntimeEncryption is the basic network + client with encryption test case.
	RuntimeEncryption scenario.Scenario = newRuntimeImpl("runtime-encryption", "simple-keyvalue-enc-client", nil)

	// DefaultRuntimeLogWatcherHandlerFactories is a list of default log watcher
	// handler factories for the basic scenario.
	DefaultRuntimeLogWatcherHandlerFactories = []log.WatcherHandlerFactory{
		oasis.LogAssertNoTimeouts(),
		oasis.LogAssertNoRoundFailures(),
		oasis.LogAssertNoExecutionDiscrepancyDetected(),
	}

	runtimeID    common.Namespace
	keymanagerID common.Namespace
	_            = runtimeID.UnmarshalHex("8000000000000000000000000000000000000000000000000000000000000000")
	_            = keymanagerID.UnmarshalHex("c000000000000000ffffffffffffffffffffffffffffffffffffffffffffffff")
)

// runtimeImpl is a base class for tests involving oasis-node with runtime.
type runtimeImpl struct {
	e2e.E2E

	clientBinary string
	clientArgs   []string
}

func newRuntimeImpl(name, clientBinary string, clientArgs []string) *runtimeImpl {
	// Empty scenario name is used for registering global parameters only.
	fullName := "runtime"
	if name != "" {
		fullName += "/" + name
	}

	sc := &runtimeImpl{
		E2E:          *e2e.NewE2E(fullName),
		clientBinary: clientBinary,
		clientArgs:   clientArgs,
	}
	sc.Flags.String(cfgClientBinaryDir, "", "path to the client binaries directory")
	sc.Flags.String(cfgRuntimeBinaryDirDefault, "", "(no-TEE) path to the runtime binaries directory")
	sc.Flags.String(cfgRuntimeBinaryDirIntelSGX, "", "(Intel SGX) path to the runtime binaries directory")
	sc.Flags.String(cfgRuntimeLoader, "oasis-core-runtime-loader", "path to the runtime loader")
	sc.Flags.String(cfgTEEHardware, "", "TEE hardware to use")
	sc.Flags.Bool(cfgIasMock, true, "if mock IAS service should be used")
	sc.Flags.Int64(cfgEpochInterval, 0, "epoch interval")

	return sc
}

func (sc *runtimeImpl) Clone() scenario.Scenario {
	return &runtimeImpl{
		E2E:          sc.E2E.Clone(),
		clientBinary: sc.clientBinary,
		clientArgs:   sc.clientArgs,
	}
}

func (sc *runtimeImpl) PreInit(childEnv *env.Env) error {
	return nil
}

func (sc *runtimeImpl) Fixture() (*oasis.NetworkFixture, error) {
	f, err := sc.E2E.Fixture()
	if err != nil {
		return nil, err
	}

	tee, err := sc.getTEEHardware()
	if err != nil {
		return nil, err
	}
	var mrSigner *sgx.MrSigner
	if tee == node.TEEHardwareIntelSGX {
		mrSigner = &sgx.FortanixDummyMrSigner
	}
	keyManagerBinary := "simple-keymanager"
	runtimeBinary := "simple-keyvalue"
	runtimeLoader, _ := sc.Flags.GetString(cfgRuntimeLoader)
	iasMock, _ := sc.Flags.GetBool(cfgIasMock)
	ff := &oasis.NetworkFixture{
		TEE: oasis.TEEFixture{
			Hardware: tee,
			MrSigner: mrSigner,
		},
		Network: oasis.NetworkCfg{
			NodeBinary:                        f.Network.NodeBinary,
			RuntimeSGXLoaderBinary:            runtimeLoader,
			DefaultLogWatcherHandlerFactories: DefaultRuntimeLogWatcherHandlerFactories,
			Consensus:                         f.Network.Consensus,
			IAS: oasis.IASCfg{
				Mock: iasMock,
			},
		},
		Entities: []oasis.EntityCfg{
			{IsDebugTestEntity: true},
			{},
		},
		Runtimes: []oasis.RuntimeFixture{
			// Key manager runtime.
			{
				ID:         keymanagerID,
				Kind:       registry.KindKeyManager,
				Entity:     0,
				Keymanager: -1,
				AdmissionPolicy: registry.RuntimeAdmissionPolicy{
					AnyNode: &registry.AnyNodeRuntimeAdmissionPolicy{},
				},
				Binaries: sc.resolveRuntimeBinaries([]string{keyManagerBinary}),
			},
			// Compute runtime.
			{
				ID:         runtimeID,
				Kind:       registry.KindCompute,
				Entity:     0,
				Keymanager: 0,
				Binaries:   sc.resolveRuntimeBinaries([]string{runtimeBinary}),
				Executor: registry.ExecutorParameters{
					GroupSize:       2,
					GroupBackupSize: 1,
					RoundTimeout:    20,
					MaxMessages:     128,
				},
				TxnScheduler: registry.TxnSchedulerParameters{
					Algorithm:         registry.TxnSchedulerSimple,
					MaxBatchSize:      1,
					MaxBatchSizeBytes: 1024,
					BatchFlushTimeout: 1 * time.Second,
					ProposerTimeout:   20,
				},
				Storage: registry.StorageParameters{
					GroupSize:               2,
					MinWriteReplication:     2,
					MaxApplyWriteLogEntries: 100_000,
					MaxApplyOps:             2,
				},
				AdmissionPolicy: registry.RuntimeAdmissionPolicy{
					AnyNode: &registry.AnyNodeRuntimeAdmissionPolicy{},
				},
				Constraints: map[scheduler.CommitteeKind]map[scheduler.Role]registry.SchedulingConstraints{
					scheduler.KindComputeExecutor: {
						scheduler.RoleWorker: {
							MinPoolSize: &registry.MinPoolSizeConstraint{
								Limit: 2,
							},
						},
						scheduler.RoleBackupWorker: {
							MinPoolSize: &registry.MinPoolSizeConstraint{
								Limit: 1,
							},
						},
					},
					scheduler.KindStorage: {
						scheduler.RoleWorker: {
							MinPoolSize: &registry.MinPoolSizeConstraint{
								Limit: 2,
							},
						},
					},
				},
			},
		},
		Validators: []oasis.ValidatorFixture{
			{Entity: 1, Consensus: oasis.ConsensusFixture{EnableConsensusRPCWorker: true}},
			{Entity: 1, Consensus: oasis.ConsensusFixture{EnableConsensusRPCWorker: true}},
			{Entity: 1, Consensus: oasis.ConsensusFixture{EnableConsensusRPCWorker: true}},
		},
		KeymanagerPolicies: []oasis.KeymanagerPolicyFixture{
			{Runtime: 0, Serial: 1},
		},
		Keymanagers: []oasis.KeymanagerFixture{
			{Runtime: 0, Entity: 1},
		},
		StorageWorkers: []oasis.StorageWorkerFixture{
			{Backend: database.BackendNameBadgerDB, Entity: 1},
			{Backend: database.BackendNameBadgerDB, Entity: 1},
		},
		ComputeWorkers: []oasis.ComputeWorkerFixture{
			{Entity: 1, Runtimes: []int{1}},
			{Entity: 1, Runtimes: []int{1}},
			{Entity: 1, Runtimes: []int{1}},
		},
		Sentries: []oasis.SentryFixture{},
		Seeds:    []oasis.SeedFixture{{}},
		Clients: []oasis.ClientFixture{
			{},
		},
	}

	if epochInterval, _ := sc.Flags.GetInt64(cfgEpochInterval); epochInterval > 0 {
		ff.Network.Beacon.InsecureParameters = &beacon.InsecureParameters{
			Interval: epochInterval,
		}
		ff.Network.Beacon.PVSSParameters = &beacon.PVSSParameters{
			CommitInterval:  epochInterval / 2,
			RevealInterval:  (epochInterval / 2) - 4,
			TransitionDelay: 4,
		}
	}

	return ff, nil
}

func (sc *runtimeImpl) start(childEnv *env.Env) (<-chan error, *exec.Cmd, error) {
	var err error
	if err = sc.Net.Start(); err != nil {
		return nil, nil, err
	}

	cmd, err := sc.startClient(childEnv)
	if err != nil {
		return nil, nil, err
	}

	clientErrCh := make(chan error)
	go func() {
		clientErrCh <- cmd.Wait()
	}()
	return clientErrCh, cmd, nil
}

// getTEEHardware returns the configured TEE hardware.
func (sc *runtimeImpl) getTEEHardware() (node.TEEHardware, error) {
	teeStr, _ := sc.Flags.GetString(cfgTEEHardware)
	var tee node.TEEHardware
	if err := tee.FromString(teeStr); err != nil {
		return node.TEEHardwareInvalid, err
	}
	return tee, nil
}

func (sc *runtimeImpl) resolveClientBinary(clientBinary string) string {
	cbDir, _ := sc.Flags.GetString(cfgClientBinaryDir)
	return filepath.Join(cbDir, clientBinary)
}

func (sc *runtimeImpl) resolveRuntimeBinaries(runtimeBinaries []string) map[node.TEEHardware][]string {
	binaries := make(map[node.TEEHardware][]string)
	for _, tee := range []node.TEEHardware{
		node.TEEHardwareInvalid,
		node.TEEHardwareIntelSGX,
	} {
		for _, binary := range runtimeBinaries {
			binaries[tee] = append(binaries[tee], sc.resolveRuntimeBinary(binary, tee))
		}
	}
	return binaries
}

func (sc *runtimeImpl) resolveRuntimeBinary(runtimeBinary string, tee node.TEEHardware) string {
	var runtimeExt, path string
	switch tee {
	case node.TEEHardwareInvalid:
		runtimeExt = ""
		path, _ = sc.Flags.GetString(cfgRuntimeBinaryDirDefault)
	case node.TEEHardwareIntelSGX:
		runtimeExt = ".sgxs"
		path, _ = sc.Flags.GetString(cfgRuntimeBinaryDirIntelSGX)
	}

	return filepath.Join(path, runtimeBinary+runtimeExt)
}

func (sc *runtimeImpl) startClient(childEnv *env.Env) (*exec.Cmd, error) {
	ctx := context.Background()

	clients := sc.Net.Clients()
	if len(clients) == 0 {
		return nil, fmt.Errorf("scenario/e2e: network has no client nodes")
	}

	sc.Logger.Info("ensuring client node is synced")
	ctrl, err := oasis.NewController(clients[0].SocketPath())
	if err != nil {
		return nil, fmt.Errorf("failed to create controller for client: %w", err)
	}
	if err = ctrl.WaitSync(ctx); err != nil {
		return nil, fmt.Errorf("client-0 failed to sync: %w", err)
	}

	d, err := childEnv.NewSubDir("client")
	if err != nil {
		return nil, err
	}

	w, err := d.NewLogWriter("client.log")
	if err != nil {
		return nil, err
	}

	args := []string{
		"--node-address", "unix:" + clients[0].SocketPath(),
		"--runtime-id", runtimeID.String(),
	}
	args = append(args, sc.clientArgs...)

	binary := sc.resolveClientBinary(sc.clientBinary)
	cmd := exec.Command(binary, args...)
	cmd.SysProcAttr = env.CmdAttrs
	cmd.Stdout = w
	cmd.Stderr = w

	sc.Logger.Info("launching client",
		"binary", binary,
		"args", strings.Join(args, " "),
	)

	if err = cmd.Start(); err != nil {
		return nil, fmt.Errorf("scenario/e2e: failed to start client: %w", err)
	}

	return cmd, nil
}

func (sc *runtimeImpl) waitClient(childEnv *env.Env, cmd *exec.Cmd, clientErrCh <-chan error) error {
	var err error
	select {
	case err = <-sc.Net.Errors():
		_ = cmd.Process.Kill()
	case err = <-clientErrCh:
	}
	if err != nil {
		return err
	}

	return nil
}

func (sc *runtimeImpl) wait(childEnv *env.Env, cmd *exec.Cmd, clientErrCh <-chan error) error {
	if err := sc.waitClient(childEnv, cmd, clientErrCh); err != nil {
		return err
	}
	return sc.Net.CheckLogWatchers()
}

func (sc *runtimeImpl) Run(childEnv *env.Env) error {
	clientErrCh, cmd, err := sc.start(childEnv)
	if err != nil {
		return err
	}

	return sc.wait(childEnv, cmd, clientErrCh)
}

func (sc *runtimeImpl) submitRuntimeTx(ctx context.Context, id common.Namespace, method string, args interface{}) (cbor.RawMessage, error) {
	c := sc.Net.ClientController().RuntimeClient

	// Submit a transaction and check the result.
	var rsp runtimeTransaction.TxnOutput
	rawRsp, err := c.SubmitTx(ctx, &runtimeClient.SubmitTxRequest{
		RuntimeID: id,
		Data: cbor.Marshal(&runtimeTransaction.TxnCall{
			Method: method,
			Args:   args,
		}),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to submit runtime tx: %w", err)
	}
	if err = cbor.Unmarshal(rawRsp, &rsp); err != nil {
		return nil, fmt.Errorf("malformed tx output from runtime: %w", err)
	}
	if rsp.Error != nil {
		return nil, fmt.Errorf("runtime tx failed: %s", *rsp.Error)
	}
	return rsp.Success, nil
}

func (sc *runtimeImpl) submitKeyValueRuntimeInsertTx(ctx context.Context, id common.Namespace, key, value string) error {
	_, err := sc.submitRuntimeTx(ctx, id, "insert", struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}{
		Key:   key,
		Value: value,
	})
	return err
}

func (sc *runtimeImpl) waitNodesSynced() error {
	ctx := context.Background()

	checkSynced := func(n *oasis.Node) error {
		c, err := oasis.NewController(n.SocketPath())
		if err != nil {
			return fmt.Errorf("failed to create node controller: %w", err)
		}
		defer c.Close()

		if err = c.WaitSync(ctx); err != nil {
			return fmt.Errorf("failed to wait for node to sync: %w", err)
		}
		return nil
	}

	sc.Logger.Info("waiting for all nodes to be synced")

	for _, n := range sc.Net.Validators() {
		if err := checkSynced(&n.Node); err != nil {
			return err
		}
	}
	for _, n := range sc.Net.StorageWorkers() {
		if err := checkSynced(&n.Node); err != nil {
			return err
		}
	}
	for _, n := range sc.Net.ComputeWorkers() {
		if err := checkSynced(&n.Node); err != nil {
			return err
		}
	}
	for _, n := range sc.Net.Clients() {
		if err := checkSynced(&n.Node); err != nil {
			return err
		}
	}

	sc.Logger.Info("nodes synced")
	return nil
}

func (sc *runtimeImpl) initialEpochTransitions(fixture *oasis.NetworkFixture) error {
	ctx := context.Background()

	if len(sc.Net.Keymanagers()) > 0 {
		// First wait for validator and key manager nodes to register. Then perform an epoch
		// transition which will cause the compute and storage nodes to register.
		sc.Logger.Info("waiting for validators to initialize",
			"num_validators", len(sc.Net.Validators()),
		)
		for i, n := range sc.Net.Validators() {
			if fixture.Validators[i].NoAutoStart {
				// Skip nodes that don't auto start.
				continue
			}
			if err := n.WaitReady(ctx); err != nil {
				return fmt.Errorf("failed to wait for a validator: %w", err)
			}
		}
		sc.Logger.Info("waiting for key managers to initialize",
			"num_keymanagers", len(sc.Net.Keymanagers()),
		)
		for i, n := range sc.Net.Keymanagers() {
			if fixture.Keymanagers[i].NoAutoStart {
				// Skip nodes that don't auto start.
				continue
			}
			if err := n.WaitReady(ctx); err != nil {
				return fmt.Errorf("failed to wait for a key manager: %w", err)
			}
		}
		sc.Logger.Info("triggering epoch transition")
		if err := sc.Net.Controller().SetEpoch(ctx, 1); err != nil {
			return fmt.Errorf("failed to set epoch: %w", err)
		}
		sc.Logger.Info("epoch transition done")
	}

	// Wait for storage workers and compute workers to become ready.
	sc.Logger.Info("waiting for storage workers to initialize",
		"num_storage_workers", len(sc.Net.StorageWorkers()),
	)
	for i, n := range sc.Net.StorageWorkers() {
		if fixture.StorageWorkers[i].NoAutoStart {
			// Skip nodes that don't auto start.
			continue
		}
		if err := n.WaitReady(ctx); err != nil {
			return fmt.Errorf("failed to wait for a storage worker: %w", err)
		}
	}
	sc.Logger.Info("waiting for compute workers to initialize",
		"num_compute_workers", len(sc.Net.ComputeWorkers()),
	)
	for i, n := range sc.Net.ComputeWorkers() {
		if fixture.ComputeWorkers[i].NoAutoStart {
			// Skip nodes that don't auto start.
			continue
		}
		if err := n.WaitReady(ctx); err != nil {
			return fmt.Errorf("failed to wait for a compute worker: %w", err)
		}
	}

	// Byzantine nodes can only registered. If defined, since we cannot control them directly, wait
	// for all nodes to become registered.
	if len(sc.Net.Byzantine()) > 0 {
		sc.Logger.Info("waiting for (all) nodes to register",
			"num_nodes", sc.Net.NumRegisterNodes(),
		)
		if err := sc.Net.Controller().WaitNodesRegistered(ctx, sc.Net.NumRegisterNodes()); err != nil {
			return fmt.Errorf("failed to wait for nodes: %w", err)
		}
	}

	// Then perform another epoch transition to elect the committees.
	sc.Logger.Info("triggering epoch transition")
	if err := sc.Net.Controller().SetEpoch(ctx, 2); err != nil {
		return fmt.Errorf("failed to set epoch: %w", err)
	}
	sc.Logger.Info("epoch transition done")

	return nil
}

// RegisterScenarios registers all end-to-end scenarios.
func RegisterScenarios() error {
	// Register non-scenario-specific parameters.
	cmd.RegisterScenarioParams(RuntimeParamsDummy.Name(), RuntimeParamsDummy.Parameters())

	// Register default scenarios which are executed, if no test names provided.
	for _, s := range []scenario.Scenario{
		// Runtime test.
		Runtime,
		RuntimeEncryption,
		// Byzantine executor node.
		ByzantineExecutorHonest,
		ByzantineExecutorSchedulerHonest,
		ByzantineExecutorWrong,
		ByzantineExecutorSchedulerWrong,
		ByzantineExecutorStraggler,
		ByzantineExecutorSchedulerStraggler,
		ByzantineExecutorFailureIndicating,
		ByzantineExecutorSchedulerFailureIndicating,
		// Byzantine storage node.
		ByzantineStorageHonest,
		ByzantineStorageFailApply,
		ByzantineStorageFailApplyBatch,
		ByzantineStorageFailRead,
		// Storage sync test.
		StorageSync,
		StorageSyncFromRegistered,
		// Sentry test.
		Sentry,
		SentryEncryption,
		// Keymanager restart test.
		KeymanagerRestart,
		// Keymanager replicate test.
		KeymanagerReplicate,
		// Dump/restore test.
		DumpRestore,
		DumpRestoreRuntimeRoundAdvance,
		// Halt test.
		HaltRestore,
		// Consensus upgrade tests.
		GovernanceConsensusUpgrade,
		GovernanceConsensusFailUpgrade,
		GovernanceConsensusCancelUpgrade,
		// Multiple runtimes test.
		MultipleRuntimes,
		// Node shutdown test.
		NodeShutdown,
		// Gas fees tests.
		GasFeesRuntimes,
		// Runtime prune test.
		RuntimePrune,
		// Runtime dynamic registration test.
		RuntimeDynamic,
		// Transaction source test.
		TxSourceMultiShort,
		// ClientExpire test.
		ClientExpire,
		// Late start test.
		LateStart,
		// KeymanagerUpgrade test.
		KeymanagerUpgrade,
		// RuntimeUpgrade test.
		RuntimeUpgrade,
		// HistoryReindex test.
		HistoryReindex,
	} {
		if err := cmd.Register(s); err != nil {
			return err
		}
	}

	// Register non-default scenarios which are executed on-demand only.
	for _, s := range []scenario.Scenario{
		// Transaction source test. Non-default, because it runs for ~6 hours.
		TxSourceMulti,
		// SGX version of the txsource-multi-short test. Non-default, because
		// it is identical to the txsource-multi-short, only using fewer nodes
		// due to SGX CI instance resource constrains.
		TxSourceMultiShortSGX,
	} {
		if err := cmd.RegisterNondefault(s); err != nil {
			return err
		}
	}

	return nil
}
