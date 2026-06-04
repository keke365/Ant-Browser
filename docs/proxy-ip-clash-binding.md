# Proxy IP And Clash Node Binding

## Background

Ant Browser already has a proxy pool and supports importing Clash subscriptions. Existing code can fetch and parse Clash subscription content through `BrowserProxyFetchClashByURL`, and browser profiles already have proxy binding fields such as `proxyId`, `proxyConfig`, and proxy source metadata.

The requested proxy IP feature should be handled separately from site account management because it touches subscription import, proxy node identity, profile binding, refresh behavior, runtime startup, and health checks.

## Goals

- Import Clash subscription nodes into the proxy pool.
- Bind imported Clash nodes to fingerprint browser profiles.
- Ensure browser profiles use the selected Clash node for network access.
- Preserve bindings after subscription refresh when possible.
- Expose health check and speed test information for imported nodes.
- Avoid breaking existing direct, HTTP, SOCKS, and chain proxy behavior.

## Non-Goals

- Site account management.
- Automatic reading or check-in tasks.
- Building a full Clash client UI.
- Replacing the existing proxy pool.

## Current Project Basis

Relevant existing capabilities:

- Clash subscription URL fetching and validation.
- Proxy pool persistence in `browser_proxies`.
- Profile proxy fields in browser profiles.
- Proxy speed and IP health result fields.
- Existing proxy selection UI and profile proxy binding logic.

This means the feature should extend the current proxy pool instead of creating a separate proxy system.

## Suggested Phases

### Phase 1: Subscription Import To Proxy Pool

- Support importing Clash subscriptions by URL.
- Parse `proxies` nodes from the subscription.
- Convert each supported node into a proxy pool record.
- Store subscription metadata:
  - source ID;
  - source URL;
  - source name prefix;
  - source refresh setting;
  - source last refresh time.
- Group imported nodes by subscription or selected proxy group.

### Phase 2: Bind Clash Nodes To Browser Profiles

- Let users select an imported Clash node from the proxy picker.
- Save the selected node as the browser profile's `proxyId`.
- Keep a binding snapshot on the browser profile:
  - source ID;
  - source URL;
  - node name;
  - updated time.
- On profile start, resolve the selected proxy into an effective proxy config.

### Phase 3: Refresh And Reconcile

- Refresh subscription nodes manually.
- Optionally refresh subscriptions automatically.
- Reconcile existing profile bindings after refresh:
  - If the same proxy ID exists, keep it.
  - If the proxy ID changed but source URL and node name match, rebind automatically.
  - If no match exists, mark the profile proxy binding as invalid and show a warning.

### Phase 4: Runtime Isolation And Advanced Clash Mode

Depending on implementation choice, add one of:

- Convert Clash nodes into direct browser proxy configs when supported.
- Or start a per-profile local proxy runtime that routes to the selected Clash node.

This phase is more complex and should be developed only after Phase 1-3 are stable.

## Data Model

### Existing Proxy Fields

Proxy pool records should continue to use the existing proxy model:

| Field | Description |
| --- | --- |
| `proxyId` | Stable proxy node ID |
| `proxyName` | Display name |
| `proxyConfig` | Browser-compatible proxy config or local runtime config |
| `dnsServers` | Optional DNS config |
| `groupName` | UI grouping |
| `sourceId` | Subscription/source ID |
| `sourceUrl` | Subscription URL |
| `sourceNamePrefix` | Prefix used when importing |
| `sourceAutoRefresh` | Whether auto refresh is enabled |
| `sourceRefreshIntervalM` | Refresh interval in minutes |
| `sourceLastRefreshAt` | Last refresh time |
| `lastLatencyMs` | Last speed test latency |
| `lastTestOk` | Last speed test status |
| `lastTestedAt` | Last speed test time |
| `lastIPHealthJSON` | Last IP health result |

### Profile Binding Snapshot

Browser profiles already contain fields that can preserve binding context:

| Field | Description |
| --- | --- |
| `proxyId` | Current selected proxy node |
| `proxyConfig` | Saved direct proxy config fallback |
| `proxyBindSourceId` | Source ID at bind time |
| `proxyBindSourceUrl` | Source URL at bind time |
| `proxyBindName` | Proxy node name at bind time |
| `proxyBindUpdatedAt` | Binding update time |

## Proxy Config Strategy

There are two practical strategies.

### Strategy A: Convert Nodes To Browser Proxy Configs

Use this when a Clash node can be represented as a browser-supported proxy:

- HTTP.
- SOCKS5.
- Some simple chain proxy cases if already supported by the existing bridge.

Advantages:

- Simpler startup.
- Works with existing profile proxy flow.
- Easier to test and debug.

Limitations:

- Not every Clash node type can be represented directly.
- Advanced Clash rules, proxy groups, and DNS behavior may not apply.

### Strategy B: Per-Profile Local Runtime

Start a local proxy runtime per browser profile or per selected node, then point the browser to the local port.

Advantages:

- Can support more Clash node types.
- Can preserve Clash-specific behavior more accurately.

Limitations:

- More processes and ports to manage.
- Higher memory and CPU cost.
- More complex failure handling.
- Requires careful cleanup when profiles stop.

Recommended approach:

1. Use Strategy A for the first deliverable.
2. Add Strategy B only for node types that cannot be converted safely.

## Supported Node Types

The requirement should explicitly define supported node types.

Recommended first phase:

- `http`
- `socks5`

Consider later:

- `ss`
- `trojan`
- `vmess`
- `vless`
- `hysteria2`
- provider-based groups

Unsupported node types should be imported with a clear status or skipped with an import report.

## Subscription Refresh Rules

On refresh:

- Fetch the subscription content again.
- Parse nodes.
- Generate stable node keys from source URL, node name, type, server, and port.
- Upsert existing nodes.
- Mark missing nodes as removed or delete them, depending on user setting.
- Reconcile browser profile bindings.
- Show a refresh report:
  - added count;
  - updated count;
  - removed count;
  - failed count;
  - affected profile count.

## Profile Start Behavior

When starting a browser profile:

1. Read the profile's selected `proxyId`.
2. Resolve the proxy from the proxy pool.
3. If the proxy exists and is healthy enough, use it.
4. If the proxy is missing but binding snapshot exists, try to re-associate by source and node name.
5. If the proxy cannot be resolved, show a clear startup warning.
6. Depending on existing user setting, allow direct startup for this run without modifying the saved proxy binding.

## Performance And Stability

Potential risks:

- Starting many per-profile proxy runtimes can consume memory and ports.
- Subscription refresh can invalidate active bindings.
- Slow proxy health checks can block UI if not queued.
- DNS behavior may differ between direct browser proxy mode and Clash runtime mode.

Recommended mitigations:

- Run speed tests and health checks in a bounded queue.
- Do not refresh subscriptions while profiles using those proxies are starting.
- Keep current bindings stable during refresh, then reconcile after refresh succeeds.
- Use local runtime only when direct proxy config is not enough.
- Track process ownership and clean up runtime processes when browser profiles stop.

## UI Requirements

Proxy pool page:

- Import Clash subscription by URL.
- Preview parsed nodes before saving.
- Show source/subscription group.
- Show supported or unsupported status.
- Show refresh button per source.
- Show refresh report.

Proxy picker:

- Show imported Clash nodes together with other proxies.
- Support filtering by source/group/type/health.
- Display node name, type, latency, IP health, and source.
- Allow binding selected node to a browser profile.

Profile edit page:

- Show selected proxy node.
- Show subscription source and last binding update time.
- Warn when the selected node is missing after refresh.

## Backend API Requirements

Suggested Wails APIs:

- `BrowserProxyFetchClashByURL`
- `BrowserProxyImportClash`
- `BrowserProxyRefreshSource`
- `BrowserProxyListSources`
- `BrowserProxyReconcileBindings`
- `BrowserProfileSetProxy`
- `BrowserProxyBatchTestSpeed`
- `BrowserProxyBatchCheckIPHealth`

Some APIs already exist or partially exist; final implementation should reuse the current names and patterns where possible.

## Validation Rules

- Subscription URL must be HTTP or HTTPS.
- Subscription response must be within size limits.
- Imported YAML must contain valid proxy nodes.
- Unsupported node types should not silently become broken proxy configs.
- Profile binding must reference an existing proxy or preserve a clear invalid-binding state.

## Open Questions

- Should unsupported Clash nodes be skipped, imported as disabled, or require local runtime support?
- Should subscription refresh delete missing nodes or mark them inactive?
- Should a profile be allowed to start directly when its bound proxy disappears?
- Should one local runtime be shared by multiple profiles using the same node, or should every profile get an isolated runtime?
- Which node types are required in the first production version?

## Acceptance Criteria

Phase 1 is complete when:

- A user can import a Clash subscription URL.
- Supported nodes appear in the proxy pool.
- Imported nodes keep source metadata.
- Unsupported nodes are reported clearly.

Phase 2 is complete when:

- A user can bind an imported node to a browser profile.
- Starting the profile uses the selected node.
- Existing speed test and IP health checks work for imported nodes where supported.

Phase 3 is complete when:

- A user can refresh a subscription.
- Existing profile bindings survive refresh when the node can be matched.
- Invalid bindings are visible and do not fail silently.
