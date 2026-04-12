# SDK Implementation Plan

- Build reusable on-chain-first SDK under `/sdk` with clean interfaces and typed models.
- Implement deterministic discovery and ranking with protocol and quote-aware filtering.
- Add protocol adapters for Pumpfun, PumpSwap, Raydium, Orca, and Meteora with deterministic derivation first and bounded RPC verification.
- Add parser adapter hooks to ingest protocol/lifecycle evidence from `solana-dex-parser-go`.
- Implement pool/token metrics (price, liquidity, market cap, supply) and pluggable circulating supply policy.
- Add tests (unit, table-driven, regression, fixture/golden style) and examples.
- Document architecture, usage, discovery modes, and limitations in README.
