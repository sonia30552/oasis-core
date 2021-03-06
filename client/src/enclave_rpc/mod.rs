//! Enclave RPC client.

#[cfg(not(target_env = "sgx"))]
mod api;
pub mod client;
pub mod macros;
mod transport;

// Re-exports.
pub use self::client::RpcClient;
