[![GitHub release](https://img.shields.io/github/release/cybozu-go/necotiator.svg?maxAge=60)][releases]
[![CI](https://github.com/cybozu-go/necotiator/actions/workflows/ci.yaml/badge.svg)](https://github.com/cybozu-go/necotiator/actions/workflows/ci.yaml)
[![PkgGoDev](https://pkg.go.dev/badge/github.com/cybozu-go/necotiator?tab=overview)](https://pkg.go.dev/github.com/cybozu-go/necotiator?tab=overview)
[![Go Report Card](https://goreportcard.com/badge/github.com/cybozu-go/necotiator)](https://goreportcard.com/report/github.com/cybozu-go/necotiator)

# Necotiator

Necotiator is a Kubernetes controller for soft multi-tenancy environments.

Necotiator provides TenantResourceQuota CRD, that allows us to limit tenant teams' resource usage.
Each tenant team can have multiple namespaces and set ResourceQuota on their own within that tenant's limit.

**Project Status**: Alpha

## Documentation

[docs](docs/) directory contains documents about designs and specifications.

[releases]: https://github.com/cybozu-go/necotiator/releases
