<script>
  import { protoName } from './api.js'
  export let events = []
  // newest first, capped for display
  $: rows = [...events].reverse().slice(0, 200)
  const endpoint = (ip, port) => (ip ? (port ? `${ip}:${port}` : ip) : '—')
  const clock = (ts) => new Date(ts * 1000).toLocaleTimeString()
</script>

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
        <tr><td colspan="8" class="text-center opacity-50 py-6">no events yet</td></tr>
      {/if}
    </tbody>
  </table>
</div>
