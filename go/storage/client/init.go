package client

import (
	"context"
	"fmt"

	"github.com/oasisprotocol/oasis-core/go/common"
	"github.com/oasisprotocol/oasis-core/go/common/crypto/signature"
	"github.com/oasisprotocol/oasis-core/go/common/identity"
	"github.com/oasisprotocol/oasis-core/go/common/logging"
	"github.com/oasisprotocol/oasis-core/go/common/node"
	registry "github.com/oasisprotocol/oasis-core/go/registry/api"
	"github.com/oasisprotocol/oasis-core/go/runtime/nodes"
	"github.com/oasisprotocol/oasis-core/go/runtime/nodes/grpc"
	"github.com/oasisprotocol/oasis-core/go/storage/api"
)

// BackendName is the name of this implementation.
const BackendName = "client"

// NewForNodesClient creates a new storage client that connects to nodes watched
// by the provided nodes gRPC client.
func NewForNodesClient(
	ctx context.Context,
	client grpc.NodesClient,
	runtime registry.RuntimeDescriptorProvider,
) (api.Backend, error) {
	b := &storageClientBackend{
		ctx:         ctx,
		logger:      logging.GetLogger("storage/client"),
		nodesClient: client,
		runtime:     runtime,
	}
	return api.NewMetricsWrapper(b), nil
}

// NewForNodes creates a new storage client that connects to nodes watched by
// the provided nodes lookup.
func NewForNodes(
	ctx context.Context,
	ident *identity.Identity,
	nodes nodes.NodeDescriptorLookup,
	runtime registry.RuntimeDescriptorProvider,
) (api.Backend, error) {
	client, err := grpc.NewNodesClient(ctx, nodes, grpc.WithClientAuthentication(ident))
	if err != nil {
		return nil, fmt.Errorf("storage/client: failed to create committee client: %w", err)
	}
	return NewForNodesClient(ctx, client, runtime)
}

// New creates a new storage client that automatically follows a given runtime's storage committee.
func New(
	ctx context.Context,
	namespace common.Namespace,
	ident *identity.Identity,
	registryBackend registry.Backend,
	runtime registry.RuntimeDescriptorProvider,
) (api.Backend, error) {
	nl, err := nodes.NewRuntimeNodeLookup(
		ctx,
		registryBackend,
		namespace,
	)
	if err != nil {
		return nil, fmt.Errorf("storage/client: failed to create runtime node watcher: %w", err)
	}

	publicStorageNl := nodes.NewFilteredNodeLookup(nl,
		nodes.WithAllFilters(
			// Ignore self.
			nodes.IgnoreNodeFilter(ident.NodeSigner.Public()),
			// Only storage nodes that opted into public RPC.
			nodes.TagFilter(nodes.TagsForRoleMask(node.RoleStorageRPC)[0]),
		),
	)

	return NewForNodes(ctx, ident, publicStorageNl, runtime)
}

// NewStatic creates a new storage client that only follows a specific storage node.
//
// This is mostly useful for tests.
func NewStatic(
	ctx context.Context,
	ident *identity.Identity,
	registryBackend registry.Backend,
	nodeID signature.PublicKey,
) (api.Backend, error) {
	nw, err := nodes.NewVersionedNodeDescriptorWatcher(ctx, registryBackend)
	if err != nil {
		return nil, fmt.Errorf("storage/client: failed to create node descriptor watcher: %w", err)
	}

	client, err := NewForNodes(ctx, ident, nw, nil)
	if err != nil {
		return nil, err
	}

	nw.Reset()
	_, err = nw.WatchNode(ctx, nodeID)
	if err != nil {
		return nil, fmt.Errorf("storage/client: failed to watch node %s: %w", nodeID, err)
	}
	nw.Freeze(0)

	return client, nil
}
