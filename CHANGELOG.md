# Changelog

All notable changes to this project will be documented in this file.

## [1.0.0] - 2025-09-11

### Added
- New `GetMultiple()` batch operation for better performance
- Pre-allocation support via `NewWithCapacity()`

### Fixed
- Race condition in `Get()` method
- Memory leak in timer cleanup

### Changed
- Improved documentation with examples

## [0.0.2] - 2024-01-10

### Fixed
- Panic when using nil callback

## [0.0.1] - 2024-01-05

### Added
- `SetExpiry()` method to change TTL of existing keys
- Sharding support for better concurrency

### Deprecated
- `Old()` method (use `New()` instead)