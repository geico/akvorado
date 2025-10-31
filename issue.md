# Feature Request: Outlet Dual-Write Support

## ✅ STATUS: Implementation Complete

All code changes, tests, and documentation have been completed. Ready for review and testing.

See `CHANGES.md` for detailed implementation summary.

## Summary

Add support for outlets to write flow data to multiple ClickHouse destinations simultaneously (parallel write), enabling resilient architectures where datacenters maintain local storage while also writing to centralized cloud storage.

## Use Case

We're deploying Akvorado across multiple datacenters (~200-300k flows/sec per datacenter) and need:

1. **Local ClickHouse** in each datacenter for fast queries during outages
2. **Centralized ClickHouse** in Azure for cross-datacenter visibility
3. **Operational resilience** - if a datacenter loses Azure connectivity, local operations continue unaffected

## Proposed Architecture

```
Datacenter A:
  Orchestrator A → manages Local ClickHouse A
  Outlet A → writes to [Local ClickHouse A, Azure ClickHouse]
  Console A → reads from Local ClickHouse A

Datacenter B:
  Orchestrator B → manages Local ClickHouse B
  Outlet B → writes to [Local ClickHouse B, Azure ClickHouse]
  Console B → reads from Local ClickHouse B

Azure:
  Orchestrator Azure → manages Azure ClickHouse
  Azure ClickHouse ← receives writes from all datacenters
  Central Console → reads from Azure ClickHouse
```

## Required Changes

### Outlet Configuration

Add support for additional ClickHouse destinations in outlet configuration:

```yaml
# Primary destination (backward compatible)
clickhousedb:
  servers: ["clickhouse-local:9000"]
  database: flows
  username: default

clickhouse:
  maximum-batch-size: 50000
  maximum-wait-time: 5s

# NEW: Additional destinations
additional-clickhouses:
  - name: azure
    connection:
      servers: ["clickhouse-azure.database.windows.net:9440"]
      database: flows
      username: akvorado_user
      password: "${AZURE_PASSWORD}"
      max-retries: 3  # Stop after 3 failed attempts (primary retries indefinitely)
      tls:
        enable: true
        verify: true
    clickhouse:
      maximum-batch-size: 50000
      maximum-wait-time: 5s
```

### Outlet Implementation

**Core changes:**

1. **Parallel writes** - Write to all destinations simultaneously using `errgroup`
2. **Per-destination retry limits** - Primary retries indefinitely, additional destinations can fail after N attempts
3. **Per-destination metrics** - Track success/failure/latency per destination
4. **Unified code** - Single code path for all destinations (no duplication)

**Key implementation pattern:**

```go
// Normalize destinations at initialization
destinations := []destination{
    {name: "primary", db: primaryDB, maxRetries: 0},      // Infinite retries
    {name: "azure", db: azureDB, maxRetries: 3},          // Stop after 3 attempts
}

// Write to all destinations in parallel
for _, dest := range destinations {
    g.Go(func() error {
        return writeWithRetryLimit(dest, maxRetries)
    })
}
```

## Non-Goals

- ❌ Orchestrator changes (each orchestrator manages its own ClickHouse)
- ❌ Schema management flags (not needed with separate orchestrators)
- ❌ Per-destination resolutions (each orchestrator has its own)
- ❌ Synchronous replication (writes are parallel, not sequential)

## Benefits

1. **Resilience** - Local operations continue during Azure outages
2. **Centralized visibility** - Single pane of glass across all datacenters
3. **Flexible retention** - Different TTLs per location (e.g., 15 days local, 30 days centralized)
4. **Simple architecture** - Each orchestrator manages one ClickHouse
5. **Backward compatible** - Existing single-destination configs work unchanged

## Implementation Complexity

**Low** - Only outlet changes needed:
- Configuration parsing (~100 LOC)
- Parallel write logic (~200 LOC)
- Retry limit logic (~50 LOC)
- Metrics (~50 LOC)

**Total:** ~400 LOC, 6-8 weeks implementation + testing

## Alternatives Considered

### 1. ClickHouse Replication

Use ClickHouse's built-in replication to replicate from local to Azure.

**Rejected because:**
- Requires ClickHouse cluster setup (complex)
- Doesn't work across isolated datacenters
- If local ClickHouse fails, data is lost (not written to Azure)

### 2. Separate Outlet Instances

Run two outlet instances per datacenter, each writing to a different ClickHouse.

**Rejected because:**
- Doubles Kafka consumption (inefficient)
- Doubles CPU/memory usage
- Enrichment happens twice (wasteful)

### 3. Async Replication Service

Write to primary, then async replicate to additional destinations.

**Rejected because:**
- Additional component to maintain
- Data delay in additional destinations
- If primary fails, data is lost

## Questions

1. **Configuration distribution** - Should additional destinations be configured in orchestrator (and distributed to outlets via HTTP API) or directly in outlet config?
   - **Recommendation:** Configure in orchestrator for centralized management

2. **Retry strategy** - Should max retries be per-destination or global?
   - **Recommendation:** Per-destination for flexibility

3. **Failure handling** - Should outlet continue if all destinations fail?
   - **Recommendation:** Block until primary succeeds or context expires

4. **Metrics** - What metrics are needed?
   - **Recommendation:** Per-destination: flows, insert_time, errors, retries_exceeded

## Success Criteria

- [ ] Outlet writes to multiple destinations in parallel
- [ ] Primary destination retries indefinitely
- [ ] Additional destinations respect max retry limit
- [ ] Per-destination metrics available
- [ ] Backward compatible (single destination works unchanged)
- [ ] Performance impact < 10% for dual-write
- [ ] Comprehensive tests (unit, integration, failure scenarios)
- [ ] Documentation (config, deployment, troubleshooting)

## Timeline

| Phase | Duration | Deliverables |
|-------|----------|--------------|
| Phase 1 | 1 week | Configuration structures, validation |
| Phase 2 | 2-3 weeks | Parallel write implementation, retry limits |
| Phase 3 | 1 week | Orchestrator config distribution |
| Phase 4 | 1-2 weeks | Testing (unit, integration, performance) |
| Phase 5 | 1 week | Documentation |
| **Total** | **6-8 weeks** | **Complete dual-write feature** |

## References

- Implementation plan: `CLEAN_IMPLEMENTATION_PLAN.md`
- Resolutions analysis: `RESOLUTIONS_ANALYSIS.md`
- Unified destinations pattern: `IMPLEMENTATION_UNIFIED_DESTINATIONS.md`

