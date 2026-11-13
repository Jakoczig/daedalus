# Performance-first module architecture — Superseded

**Status:** Superseded on 2026-11-12 by the maintainer's explicit decision to migrate the **runtime to Go now** and provide **full Windows and Linux desktop applications**.

The current design is [Go-First Cross-Platform Desktop Rebuild](./2026-11-12-go-first-windows-linux-rebuild-design.md).

The core product objectives are unchanged: one shared journal reader, resource-efficient native modules, secondary-screen browser access, independent enable/disable and per-device visibility, creator attribution, and safe verified updates. The former Node-service-as-permanent-target architecture described in this file is **not** approved for future features.

No executable or product code is modified by the design PR. Phase 1A security tests and the current UI remain important parity references; Phase 1B Inara logic should be reimplemented in Go after the new runtime foundation rather than expanding Node-specific code.
