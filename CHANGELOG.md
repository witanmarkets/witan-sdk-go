# Changelog

Every release of `github.com/witanmarkets/witan-sdk-go`, newest first, grouped the way
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) groups them.

## 0.1.0 — 2026-10-23

### Added

- The first release: `SearchError`, `Search`, `Read`, `Datasets`, `Dataset`, `DatasetData`, `Query`,
  `Quota` and `Credits`, on the standard library alone.
- Options `WithAPIKey`, `WithBaseURL`, `WithHTTPClient`, `WithUserAgent`, `WithRetries`; `WITAN_API_KEY`
  and `WITAN_BASE_URL` as fallbacks.
- `*APIError` for every non-2xx answer and `*PaymentRequiredError` for a 402, with the price and the x402
  URL. Network errors, 429, 502, 503 and 504 are retried twice, honouring `Retry-After`.
