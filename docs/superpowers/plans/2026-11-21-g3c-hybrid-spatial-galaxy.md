# G3C implementation sequence

Base: G2Q 905ef388579c9bacbffd149dd68fc9791861f61d. Implement approved directive without further design approval.

1. Add frozen G3A fixtures, strict manifest/index/ESP1/classic ZIP readers and rejection tests in atlas/spatial_format*.go. Reuse existing canonical Ed25519 verification.
2. Add independently persisted spatial manager with signed pointer/descriptor validation, atomic install/rollback, lazy Bubble ZIP reads, bounded SHA cache, cancellable two-worker region queue, owner consent and query budgets. Test offline restart, corrupt updates, cache reuse, prefetch/cancellation and permissions.
3. Connect owner management/cached-view RPCs to existing authenticated Go service. Add event-driven Canvas Local Volume and controls beside unchanged overview, exact synchronized projection tests.
4. Verify public signed release in disposable profile, record matching Sol/sparse screenshots and resource measurements. Complete focused tests, then final Windows/Linux/security/race/browser CI and unsigned artifacts. Draft stacked PR against G2Q; preserve original application and production infrastructure.
