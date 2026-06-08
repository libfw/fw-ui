<script>
  import { protoName } from './api.js'
  export let events = []

  let paused = false
  let frozen = []
  let fVerdict = 'all' // all | allow | deny
  let fDir = 'all' // all | egress | ingress
  let fText = ''

  // While paused, keep showing the events captured at the moment of pausing.
  function togglePause() {
    if (!paused) frozen = events
    paused = !paused
  }
  $: source = paused ? frozen : events

  const endpoint = (ip, port) => (ip ? (port ? `${ip}:${port}` : ip) : '—')
  const clock = (ts) => new Date(ts * 1000).toLocaleTimeString()

  function match(e) {
    if (fVerdict !== 'all' && e.verdict !== fVerdict) return false
    if (fDir !== 'all' && e.dir !== fDir) return false
    if (fText) {
      const t = fText.toLowerCase()
      if (!((e.src || '') + (e.dst || '')).toLowerCase().includes(t)) return false
    }
    return true
  }
  $: rows = [...source].reverse().filter(match).slice(0, 300)
</script>

<div class="flex flex-wrap items-center gap-2 mb-2">
  <select class="select select-bordered select-xs" bind:value={fVerdict}>
    <option value="all">all verdicts</option>
    <option value="allow">allow</option>
    <option value="deny">deny</option>
  </select>
  <select class="select select-bordered select-xs" bind:value={fDir}>
    <option value="all">both dirs</option>
    <option value="egress">egress</option>
    <option value="ingress">ingress</option>
  </select>
  <input
    class="input input-bordered input-xs w-40"
    placeholder="filter src/dst…"
    bind:value={fText}
  />
  <div class="flex-1"></div>
  <span class="text-xs opacity-60">{rows.length} shown</span>
  <button class="btn btn-xs {paused ? 'btn-warning' : 'btn-ghost'}" on:click={togglePause}>
    {paused ? '▶ resume' : '⏸ pause'}
  </button>
</div>

<div class="overflow-x-auto max-h-96">
  <table class="table table-xs table-pin-rows">
    <thead>
      <tr>
        <th>time</th><th>dir</th><th>verdict</th><th>rule</th><th>proto</th>
        <th>source</th><th>dest</th><th class="text-right">len</th>
      </tr>
    </thead>
    <tbody>
      {#each rows as e (e.seq)}
        <tr>
          <td class="opacity-60">{clock(e.ts)}</td>
          <td><span class="badge badge-ghost badge-sm">{e.dir}</span></td>
          <td>
            <span class="badge badge-sm {e.verdict === 'allow' ? 'badge-success' : 'badge-error'}">
              {e.verdict}
            </span>
          </td>
          <td>{e.rule >= 0 ? e.rule : e.verdict === 'allow' ? 'ct/def' : 'def'}</td>
          <td>{protoName(e.proto)}</td>
          <td class="font-mono">{endpoint(e.src, e.sport)}</td>
          <td class="font-mono">{endpoint(e.dst, e.dport)}</td>
          <td class="text-right">{e.len}</td>
        </tr>
      {/each}
      {#if rows.length === 0}
        <tr><td colspan="8" class="text-center opacity-50 py-6">no events</td></tr>
      {/if}
    </tbody>
  </table>
</div>
