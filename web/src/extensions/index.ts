/*
Copyright (C) 2023-2026 QuantumNous

Extension module registry for the new-api web UI.

Each installed plugin lives under `src/extensions/<plugin-name>/` and exposes
its contributions (menu groups, channel types, locales) via named exports from
its own `index.ts`. This barrel file re-exports the *aggregated* contribution
objects (menus, channel types, locales) that the core host consumes in the
anchors documented in docs/zsy-runninghub-dev-plan.md §2.3 P6–P8.

The aggregated objects are intentionally initialised as empty; plugins
register by appending their contributions via the typed arrays below. Keeping
the registration explicit (as opposed to per-plugin barrel imports) means
installing or uninstalling a plugin only touches this and the plugin folder.
*/

export * from './menus'
export * from './channel-types'
export * from './locales'
