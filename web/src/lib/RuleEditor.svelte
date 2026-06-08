<script>
  import { onMount } from 'svelte'
  import { getRules, setACL, reload, resetStats } from './api.js'

  let source = ''
  let format = 'json'
  let msg = null // { ok, text }
  let busy = false

  async function load() {
    const r = await getRules()
    format = r.format
    source = r.source || ''
  }
  onMount(load)

  function flash(ok, text) {
    msg = { ok, text }
    setTimeout(() => (msg = null), 4000)
  }

  async function apply() {
    busy = true
    try {
      const a = await setACL(source)
      if (a.ok) flash(true, `applied — ${a.rules} rule(s) active`)
      else flash(false, a.error || 'rejected')
    } catch (e) {
      flash(false, String(e))
    }
    busy = false
  }
  async function doReload() {
    const a = await reload()
    if (a.ok) await load()
    flash(a.ok, a.ok ? 'reloaded from --acl file' : a.error || 'reload failed')
  }
  async function doReset() {
    const a = await resetStats()
    flash(a.ok, a.ok ? 'counters reset' : a.error || 'failed')
  }
</script>

<div class="flex items-center gap-2 mb-2">
  <span class="badge badge-outline">{format}</span>
  <div class="flex-1"></div>
  <button class="btn btn-sm btn-primary" class:loading={busy} on:click={apply}>Apply</button>
  <button class="btn btn-sm" on:click={doReload}>Reload file</button>
  <button class="btn btn-sm btn-ghost" on:click={doReset}>Reset stats</button>
</div>
<textarea
  class="textarea textarea-bordered w-full font-mono text-xs h-72"
  bind:value={source}
  spellcheck="false"
  placeholder="ACL ruleset (JSON) — edit and Apply to hot-swap"
></textarea>
{#if msg}
  <div class="alert {msg.ok ? 'alert-success' : 'alert-error'} mt-2 py-2 text-sm">{msg.text}</div>
{/if}
