# Security policy

## Reporting a vulnerability

Report vulnerabilities privately through GitHub:
**[Report a vulnerability](https://github.com/witanmarkets/witan-sdk-go/security/advisories/new)**, in this
repository's **Security** tab.

Do not open a public issue, pull request or discussion for a security problem.

Please include:

- the module version (`go list -m github.com/witanmarkets/witan-sdk-go`) and your Go version
- what you did, what happened, and what you expected
- whether it is already public

You get an answer within a few days. Fixes ship as a new tag, and the advisory names the versions affected.

## Scope

In scope: the module `github.com/witanmarkets/witan-sdk-go`. Report problems with a WITAN origin (the hosted
service at witan.markets) through [witan.markets/contact](https://witan.markets/contact) or the same GitHub
form; they go to the same person.

The module has no dependencies outside the Go standard library. A release is a tag on this repository;
the Go checksum database (sum.golang.org) records each version's hash the first time anyone fetches it.
