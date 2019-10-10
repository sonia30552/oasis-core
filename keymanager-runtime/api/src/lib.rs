//! Key manager API.
extern crate ekiden_runtime;
extern crate failure;
extern crate lazy_static;
extern crate rand;
extern crate rustc_hex;
extern crate serde;
extern crate serde_bytes;
extern crate serde_derive;
extern crate x25519_dalek;

use failure::Fallible;
use lazy_static::lazy_static;
use std::collections::HashSet;

use ekiden_runtime::common::{
    cbor,
    crypto::signature::{PrivateKey as EkidenPrivateKey, PublicKey as EkidenPublicKey},
};

#[macro_use]
mod api;

// Re-exports.
pub use api::*;

lazy_static! {
    static ref MULTISIG_KEYS: HashSet<EkidenPublicKey> = {
        let mut set = HashSet::new();
        if option_env!("EKIDEN_UNSAFE_KM_POLICY_KEYS").is_some() {
            for seed in [
                "ekiden key manager test multisig key 0",
                "ekiden key manager test multisig key 1",
                "ekiden key manager test multisig key 2",
            ].iter() {
                let private_key = EkidenPrivateKey::from_test_seed(
                    seed.to_string(),
                );
                set.insert(private_key.public_key());
            }
        }

        // TODO: Populate with the production keys as well.
        set
    };
}
const MULTISIG_THRESHOLD: usize = 9001; // TODO: Set this to a real value.

const POLICY_SIGN_CONTEXT: [u8; 8] = *b"EkKmPolS";

impl SignedPolicySGX {
    /// Verify the signatures and return the PolicySGX, if the signatures are correct.
    pub fn verify(&self) -> Fallible<PolicySGX> {
        // Verify the signatures.
        let untrusted_policy_raw = cbor::to_vec(&self.policy);
        let mut signers: HashSet<EkidenPublicKey> = HashSet::new();
        for sig in &self.signatures {
            let public_key = match sig.public_key {
                Some(public_key) => public_key,
                None => return Err(KeyManagerError::PolicyInvalid.into()),
            };

            if !sig
                .signature
                .verify(&public_key, &POLICY_SIGN_CONTEXT, &untrusted_policy_raw)
                .is_ok()
            {
                return Err(KeyManagerError::PolicyInvalidSignature.into());
            }
            signers.insert(public_key);
        }

        // Ensure that enough valid signatures from trusted signers are present.
        let signers: HashSet<_> = MULTISIG_KEYS.intersection(&signers).collect();
        let multisig_threshold = match option_env!("EKIDEN_UNSAFE_KM_POLICY_KEYS") {
            Some(_) => 2,
            None => MULTISIG_THRESHOLD,
        };
        if signers.len() < multisig_threshold {
            return Err(KeyManagerError::PolicyInsufficientSignatures.into());
        }

        Ok(self.policy.clone())
    }
}
