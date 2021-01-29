//! Registry structures.
//!
//! # Note
//!
//! This **MUST** be kept in sync with go/registry/api.
//!
use serde::{Deserialize, Serialize};
use serde_repr::*;

use crate::{
    common::{
        crypto::{
            hash::Hash,
            signature::{PublicKey, SignatureBundle},
        },
        namespace::Namespace,
        quantity,
        version::Version,
    },
    consensus::staking,
    storage::mkvs::WriteLog,
};
use std::collections::BTreeMap;

#[derive(Clone, Debug, PartialEq, Eq, Hash, Serialize_repr, Deserialize_repr)]
#[repr(u32)]
pub enum RuntimeKind {
    #[serde(rename = "invalid")]
    KindInvalid = 0,
    #[serde(rename = "compute")]
    KindCompute = 1,
    #[serde(rename = "keymanager")]
    KindKeyManager = 2,
}

impl Default for RuntimeKind {
    fn default() -> Self {
        RuntimeKind::KindInvalid
    }
}

#[derive(Clone, Debug, Default, PartialEq, Eq, Hash, Serialize, Deserialize)]
pub struct ExecutorParameters {
    #[serde(default)]
    pub group_size: u64,
    #[serde(default)]
    pub group_backup_size: u64,
    #[serde(default)]
    pub allowed_stragglers: u64,
    #[serde(default)]
    pub round_timeout: i64,
    #[serde(default)]
    pub max_messages: u32,
    #[serde(default)]
    pub min_pool_size: u64,
}

#[derive(Clone, Debug, Default, PartialEq, Eq, Hash, Serialize, Deserialize)]
pub struct TxnSchedulerParameters {
    #[serde(default)]
    pub algorithm: String,
    #[serde(default)]
    pub batch_flush_timeout: i64, // In nanoseconds.
    #[serde(default)]
    pub max_batch_size: u64,
    #[serde(default)]
    pub max_batch_size_bytes: u64,
    #[serde(default)]
    pub propose_batch_timeout: i64,
}

#[derive(Clone, Debug, Default, PartialEq, Eq, Hash, Serialize, Deserialize)]
pub struct StorageParameters {
    #[serde(default)]
    pub group_size: u64,
    #[serde(default)]
    pub min_write_replication: u64,
    #[serde(default)]
    pub max_apply_write_log_entries: u64,
    #[serde(default)]
    pub max_apply_ops: u64,
    #[serde(default)]
    pub checkpoint_interval: u64,
    #[serde(default)]
    pub checkpoint_num_kept: u64,
    #[serde(default)]
    pub checkpoint_chunk_size: u64,
    #[serde(default)]
    pub min_pool_size: u64,
}

#[derive(Clone, Debug, Default, PartialEq, Eq, Hash, Serialize, Deserialize)]
pub struct RuntimeStakingParameters {
    #[serde(skip_serializing_if = "Option::is_none")]
    #[serde(default)]
    pub thresholds: Option<BTreeMap<staking::ThresholdKind, quantity::Quantity>>,
}

#[derive(Clone, Debug, PartialEq, Eq, Hash, PartialOrd, Ord, Serialize_repr, Deserialize_repr)]
#[repr(u32)]
pub enum RolesMask {
    #[serde(rename = "compute")]
    RoleComputeWorker = 1 << 0,
    #[serde(rename = "storage")]
    RoleStorageWorker = 1 << 1,
    #[serde(rename = "key-manager")]
    RoleKeyManager = 1 << 2,
    #[serde(rename = "validator")]
    RoleValidator = 1 << 3,
    #[serde(rename = "consensus-rpc")]
    RoleConsensusRPC = 1 << 4,
}

#[derive(Clone, Debug, Default, PartialEq, Eq, Hash, Serialize, Deserialize)]
pub struct EntityWhitelistRuntimeAdmissionPolicy {
    #[serde(skip_serializing_if = "Option::is_none")]
    #[serde(rename = "entities")]
    #[serde(default)]
    pub entities: Option<BTreeMap<PublicKey, EntityWhitelistConfig>>,
}

#[derive(Clone, Debug, Default, PartialEq, Eq, Hash, Serialize, Deserialize)]
pub struct EntityWhitelistConfig {
    #[serde(skip_serializing_if = "Option::is_none")]
    #[serde(rename = "max_nodes")]
    #[serde(default)]
    pub max_nodes: Option<BTreeMap<RolesMask, u16>>,
}

#[derive(Clone, Debug, PartialEq, Eq, Hash, Serialize, Deserialize)]
pub enum RuntimeAdmissionPolicy {
    #[serde(rename = "any_node")]
    AnyNode {},
    #[serde(rename = "entity_whitelist")]
    EntityWhitelist {
        #[serde(flatten)]
        policy: EntityWhitelistRuntimeAdmissionPolicy,
    },
}

impl Default for RuntimeAdmissionPolicy {
    fn default() -> Self {
        RuntimeAdmissionPolicy::AnyNode {}
    }
}

#[derive(Clone, Debug, PartialEq, Eq, Hash, Serialize_repr, Deserialize_repr)]
#[repr(u8)]
pub enum RuntimeGovernanceModel {
    #[serde(rename = "invalid")]
    GovernanceInvalid = 0,
    #[serde(rename = "entity")]
    GovernanceEntity = 1,
    #[serde(rename = "runtime")]
    GovernanceRuntime = 2,
    #[serde(rename = "consensus")]
    GovernanceConsensus = 3,
}

impl Default for RuntimeGovernanceModel {
    fn default() -> Self {
        RuntimeGovernanceModel::GovernanceInvalid
    }
}

#[derive(Clone, Debug, Default, PartialEq, Eq, Hash, Serialize, Deserialize)]
pub struct VersionInfo {
    #[serde(default)]
    pub version: Version,
    #[serde(skip_serializing_if = "Option::is_none")]
    #[serde(with = "serde_bytes")]
    #[serde(default)]
    pub tee: Option<Vec<u8>>,
}

#[derive(Clone, Debug, PartialEq, Eq, Hash, Serialize_repr, Deserialize_repr)]
#[repr(u8)]
pub enum TEEHardware {
    #[serde(rename = "invalid")]
    TEEHardwareInvalid = 0,
    #[serde(rename = "intel-sgx")]
    TEEHardwareIntelSGX = 1,
}

impl Default for TEEHardware {
    fn default() -> Self {
        TEEHardware::TEEHardwareInvalid
    }
}

#[derive(Clone, Debug, Default, PartialEq, Eq, Hash, Serialize, Deserialize)]
pub struct Runtime {
    #[serde(default)]
    pub v: u16,
    #[serde(default)]
    pub id: Namespace,
    #[serde(default)]
    pub entity_id: PublicKey,
    #[serde(default)]
    pub genesis: RuntimeGenesis,
    #[serde(default)]
    pub kind: RuntimeKind,
    #[serde(default)]
    pub tee_hardware: TEEHardware,
    #[serde(default)]
    pub versions: VersionInfo,
    #[serde(skip_serializing_if = "Option::is_none")]
    #[serde(default)]
    pub key_manager: Option<Namespace>,
    #[serde(default)]
    pub executor: ExecutorParameters,
    #[serde(default)]
    pub txn_scheduler: TxnSchedulerParameters,
    #[serde(default)]
    pub storage: StorageParameters,
    #[serde(default)]
    pub admission_policy: RuntimeAdmissionPolicy,
    #[serde(skip_serializing_if = "staking_params_are_empty")]
    #[serde(default)]
    pub staking: RuntimeStakingParameters,
    #[serde(default)]
    pub governance_model: RuntimeGovernanceModel,
}

fn staking_params_are_empty(p: &RuntimeStakingParameters) -> bool {
    return Option::is_none(&p.thresholds);
}

/// Runtime genesis information that is used to initialize runtime state in the first block.
#[derive(Clone, Debug, Default, PartialEq, Eq, Hash, Serialize, Deserialize)]
pub struct RuntimeGenesis {
    /// State root that should be used at genesis time. If the runtime should start with empty state,
    /// this must be set to the empty hash.
    pub state_root: Hash,

    /// State identified by the state_root. It may be empty iff all storage_receipts are valid or
    /// state_root is an empty hash or if used in network genesis (e.g. during consensus chain init).
    pub state: Option<WriteLog>,

    /// Storage receipts for the state root. The list may be empty or a signature in the list
    /// invalid iff the state is non-empty or state_root is an empty hash or if used in network
    /// genesis (e.g. during consensus chain init).
    pub storage_receipts: Option<Vec<SignatureBundle>>,

    /// Runtime round in the genesis.
    pub round: u64,
}
