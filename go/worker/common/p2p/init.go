package p2p

import (
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"
)

const (
	// CfgP2PEnabled enables the P2P worker (automatically enabled if compute worker enabled).
	CfgP2PEnabled = "worker.p2p.enabled"

	// CfgP2pPort configures the P2P port.
	CfgP2pPort = "worker.p2p.port"

	cfgP2pAddresses = "worker.p2p.addresses"

	// CfgP2PPeerOutboundQueueSize sets the libp2p gossipsub buffer size for outbound messages.
	CfgP2PPeerOutboundQueueSize = "worker.p2p.peer_outbound_queue_size"
	// CfgP2PValidateQueueSize sets the libp2p gossipsub buffer size of the validate queue.
	CfgP2PValidateQueueSize = "worker.p2p.validate_queue_size"
)

// Enabled reads our enabled flag from viper.
func Enabled() bool {
	return viper.GetBool(CfgP2PEnabled)
}

// Flags has the configuration flags.
var Flags = flag.NewFlagSet("", flag.ContinueOnError)

func init() {
	Flags.Bool(CfgP2PEnabled, false, "Enable P2P worker (automatically enabled if compute worker enabled)")
	Flags.Uint16(CfgP2pPort, 9200, "Port to use for incoming P2P connections")
	Flags.StringSlice(cfgP2pAddresses, []string{}, "Address/port(s) to use for P2P connections when registering this node (if not set, all non-loopback local interfaces will be used)")
	Flags.Int64(CfgP2PPeerOutboundQueueSize, 32, "Set libp2p gossipsub buffer size for outbound messages")
	Flags.Int64(CfgP2PValidateQueueSize, 32, "Set libp2p gossipsub buffer size of the validate queue")

	_ = viper.BindPFlags(Flags)
}
