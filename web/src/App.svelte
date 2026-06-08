<script>
  import { onMount } from 'svelte'
  import { snapshot, connection, startStream } from './lib/api.js'
  import Chart from './lib/Chart.svelte'
  import EventsTable from './lib/EventsTable.svelte'
  import RuleEditor from './lib/RuleEditor.svelte'
  import ThemeSelect from './lib/ThemeSelect.svelte'

  onMount(startStream)

  // Derive per-interval rates from the counter samples.
  function rates(samples, pick) {
    const out = []
    for (let i = 1; i < samples.length; i++) {
      const dt = (samples[i].ts - samples[i - 1].ts) / 1000
      const dv = pick(samples[i].stats) - pick(samples[i - 1].stats)
      out.push(dt > 0 ? Math.max(0, dv / dt) : 0)
    }
    return out
  }

  $: snap = $snapshot
  $: samples = snap?.samples ?? []
  $: latest = snap?.latest ?? null
  $: ct = latest?.conntrack ?? { capacity: 0, live: 0, lookups: 0, hits: 0, inserts: 0 }

  $: ppsSeries = [
    { label: 'egress allow', color: '#22c55e', data: rates(samples, (s) => s.acl.egress.allow) },
    { label: 'egress deny', color: '#ef4444', data: rates(samples, (s) => s.acl.egress.deny) },
    { label: 'ingress allow', color: '#3b82f6', data: rates(samples, (s) => s.acl.ingress.allow) },
    { label: 'ingress deny', color: '#f59e0b', data: rates(samples, (s) => s.acl.ingress.deny) },
  ]
  $: bpsSeries = [
    { label: 'egress', color: '#22c55e', data: rates(samples, (s) => s.acl.egress.bytes_allow + s.acl.egress.bytes_deny) },
    { label: 'ingress', color: '#3b82f6', data: rates(samples, (s) => s.acl.ingress.bytes_allow + s.acl.ingress.bytes_deny) },
  ]

  const sum = (d) => (d ? d.allow + d.deny : 0)
  $: totalAllow = latest ? latest.acl.egress.allow + latest.acl.ingress.allow : 0
  $: totalDeny = latest ? latest.acl.egress.deny + latest.acl.ingress.deny : 0
  $: rules = latest?.rules ?? []

  const connBadge = { open: 'badge-success', connecting: 'badge-warning', error: 'badge-error' }
</script>

<div class="min-h-screen bg-base-200">
  <div class="navbar bg-base-100 shadow-sm">
    <div class="flex-1 px-2">
      <span class="text-lg font-semibold">fw-ui</span>
      <span class="ml-2 text-sm opacity-60">socket_vmnet firewall</span>
    </div>
    <div class="flex-none px-2 flex items-center gap-2">
      <span class="badge {connBadge[$connection] || 'badge-ghost'} gap-1">
        {$connection}{#if snap && !snap.connected} · daemon down{/if}
      </span>
      <ThemeSelect />
    </div>
  </div>

  <div class="p-4 grid gap-4 max-w-6xl mx-auto">
    <!-- stat cards -->
    <div class="grid grid-cols-2 md:grid-cols-4 gap-4">
      <div class="stat bg-base-100 rounded-box shadow">
        <div class="stat-title">Allowed</div>
        <div class="stat-value text-success text-2xl">{totalAllow.toLocaleString()}</div>
        <div class="stat-desc">frames (egress+ingress)</div>
      </div>
      <div class="stat bg-base-100 rounded-box shadow">
        <div class="stat-title">Denied</div>
        <div class="stat-value text-error text-2xl">{totalDeny.toLocaleString()}</div>
        <div class="stat-desc">frames blocked</div>
      </div>
      <div class="stat bg-base-100 rounded-box shadow">
        <div class="stat-title">Conntrack</div>
        <div class="stat-value text-2xl">{ct.live}<span class="text-base opacity-50">/{ct.capacity}</span></div>
        <div class="stat-desc">live flows</div>
      </div>
      <div class="stat bg-base-100 rounded-box shadow">
        <div class="stat-title">CT hit rate</div>
        <div class="stat-value text-2xl">{ct.lookups ? Math.round((ct.hits / ct.lookups) * 100) : 0}%</div>
        <div class="stat-desc">{ct.inserts.toLocaleString()} inserts</div>
      </div>
    </div>

    <!-- charts -->
    <div class="grid md:grid-cols-2 gap-4">
      <div class="card bg-base-100 shadow"><div class="card-body p-4">
        <h2 class="card-title text-base">Packets / s</h2>
        <Chart series={ppsSeries} unit="pps" />
      </div></div>
      <div class="card bg-base-100 shadow"><div class="card-body p-4">
        <h2 class="card-title text-base">Bytes / s</h2>
        <Chart series={bpsSeries} unit="Bps" />
      </div></div>
    </div>

    <!-- rules + per-rule hits -->
    <div class="grid lg:grid-cols-2 gap-4">
      <div class="card bg-base-100 shadow"><div class="card-body p-4">
        <h2 class="card-title text-base">Ruleset</h2>
        <RuleEditor />
      </div></div>
      <div class="card bg-base-100 shadow"><div class="card-body p-4">
        <h2 class="card-title text-base">Per-rule hits</h2>
        <div class="overflow-x-auto max-h-72">
          <table class="table table-sm">
            <thead><tr><th>#</th><th class="text-right">hits</th></tr></thead>
            <tbody>
              {#each rules as r (r.index)}
                <tr><td>{r.index}</td><td class="text-right font-mono">{r.hits.toLocaleString()}</td></tr>
              {/each}
              {#if rules.length === 0}
                <tr><td colspan="2" class="text-center opacity-50 py-6">no rules</td></tr>
              {/if}
            </tbody>
          </table>
        </div>
      </div></div>
    </div>

    <!-- live events -->
    <div class="card bg-base-100 shadow"><div class="card-body p-4">
      <h2 class="card-title text-base">Live events</h2>
      <EventsTable events={snap?.events ?? []} />
    </div></div>
  </div>
</div>
