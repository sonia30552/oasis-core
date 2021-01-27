# ADR 0008: Entity Key Generation Process

## Changelog

- 2021-01-27: Initial draft

## Status

Proposed

## Context

Currently, each application interacting with the [Oasis Network] defines its own
method of generating the entity's public/private key pair.

Entity's public key is in turn used to derive the [staking account]'s address
of the form `oasis1 ... 40 characters ...` which is used to for a variety of
operations (i.e. token transfers, delegations/undelegations, ...) on the
network.

The blockchain ecosystem has developed many standards for generating keys which
improve key storage and interoperability between different applications.

Adopting these standards will allow the Oasis ecosystem to:

- Make key derivation the same across different applications (i.e. wallets).
- Allow users to hold keys in hardware wallets.
- Allow users to hold keys in cold storage more reliably (i.e. using the
  familiar 24 word mnemonics).
- Define how users can generate multiple keys from a single seed (i.e.
  the 24 or 12 word mnemonic).

## Decision

### Mnemonic Codes for Master Key Derivation

We use Bitcoin's [BIP-0039]: _Mnemonic code for generating deterministic keys_
to derivate a binary seed from a mnemonic code.

The binary seed is in turn used to derive the _master key_, the root key from
which a hierarchy of deterministic keys is derived, as described in
[Hierarchical Key Derivation Scheme][hd-scheme].

We strongly recommend using 24 word mnemonics which correspond to 256 bits of
entropy.

### Hierarchical Key Derivation Scheme

We use [BIP32-Ed25519] key derivation scheme which adapts Bitcoin's [BIP-0032]:
_Hierarchical Deterministic Wallets_ derivation algorithm for the edwards25519
curve defined in [RFC 7748].

_TODO: Describe in more detail._

### Hierarchy of Multiple Entity Keys

We adapt [BIP-0044]: _Multi-Account Hierarchy for Deterministic Wallets_ for
generating deterministic keys where `coin_type` equals 474, as assigned to the
Oasis Network by [SLIP-0044].

The following [BIP-0032] path should be used to generate keys:

```
m / 44' / 474' / 0' / 0' / x'
```

where `x` represents the key number.

The key corresponding to key number 0 (i.e. `m / 44' / 474' / 0' / 0' / 0'`) is
called the _primary key_.

The staking account corresponding to the _primary key_ is called the _primary
account_. Applications (i.e. wallets) should use this account as a user's
default Oasis account.

## Rationale

BIPs and SLIPs are industry standards used by a majority of blockchain projects
and software/hardware wallets.

### BIP32-Ed25519 for Entity Key Derivation

The [BIP32-Ed25519] (also sometimes referred to as _Ed25519 and BIP32
based on Khovratovich_) is a key derivation scheme that also adapts [BIP-0032]'s
hierarchical derivation scheme for the edwards25519 curve from the Ed25519
signature scheme specified in [RFC 8032].

It is used by Cardano ([CIP 3]) and Tezos (dubbed [bip25519 derivation scheme]).

It is commonly used by Ledger applications, including:

- [Polkadot's Ledger app],
- [Zcash's Ledger app].

Web3's security researcher warned about a [potential key recovery attack on the
BIP32-Ed25519 scheme][BIP32-Ed25519-attack] which could occur under the
following circumstances:

1. The Ed25519 library used in BIP-Ed25519 derivation scheme does clamping
   immediately before signing.
2. Adversary has the power to make numerous small payments in deep hierarchies
   of key derivations, observe if the victim can cash out each payment, and
   adaptively continue this process.

Similar to Stellar's [SEP-0005], we decided not to use the full [BIP-0032]
derivation path specified by [BIP-0044] because [SLIP-0010]'s scheme for
edwards25519 curve only supports hardened key generation from a private parent
key to a private child key.

## Test Cases



## Alternatives

### SLIP-0010 for Entity Key Derivation

Entity keys are derived according to Sathoshi Labs' [SLIP-0010]: _Universal
private key derivation from master private key_, which is a superset of
Bitcoin's [BIP-0032]: _Hierarchical Deterministic Wallets_ derivation algorithm,
extended to work on other curves.

Entity keys use the edwards25519 curve from the Ed25519 signature scheme
specified in [RFC 8032].

## Consequences

> This section describes the consequences, after applying the decision. All
> consequences should be summarized here, not just the "positive" ones.

### Positive

### Negative

### Neutral

## References

- [SLIP-0010]
- Stellar's [SEP-0005]

[hd-scheme]: #hierarchical-key-derivation-scheme
[Oasis Network]: https://docs.oasis.dev/general/oasis-network/overview
[staking account]: ../consensus/staking.md#accounts
[BIP-0032]: https://github.com/bitcoin/bips/blob/master/bip-0032.mediawiki
[BIP-0039]: https://github.com/bitcoin/bips/blob/master/bip-0039.mediawiki
[BIP-0044]: https://github.com/bitcoin/bips/blob/master/bip-0044.mediawiki
[SLIP-0010]: https://github.com/satoshilabs/slips/blob/master/slip-0010.md
[SLIP-0044]: https://github.com/satoshilabs/slips/blob/master/slip-0044.md
[RFC 7748]: https://tools.ietf.org/html/rfc7748
[RFC 8032]: https://tools.ietf.org/html/rfc8032
[SEP-0005]:
  https://github.com/stellar/stellar-protocol/blob/master/ecosystem/sep-0005.md
[BIP32-Ed25519]:
  https://raw.githubusercontent.com/LedgerHQ/orakolo/master/papers/Ed25519_BIP%20Final.pdf
[BIP32-Ed25519-attack]:
  https://forum.w3f.community/t/key-recovery-attack-on-bip32-ed25519/44
[CIP 3]: https://cips.cardano.org/cips/cip3/
[bip25519 derivation scheme]:
  https://medium.com/@obsidian.systems/v2-2-0-of-tezos-ledger-apps-babylon-support-and-more-e8df0e4ea161
[Polkadot's Ledger app]:
  https://wiki.polkadot.network/docs/en/learn-accounts#portability
[Zcash's Ledger app]: https://zondax.ch/zcash.html#ledger-app
