<script>
  // A dependency-free SVG line chart for real-time series.
  // series: [{ label, color, data: number[] }]
  export let series = []
  export let height = 150
  export let unit = ''

  const W = 600
  $: maxv = Math.max(1, ...series.flatMap((s) => s.data))
  function path(data) {
    if (data.length < 2) return ''
    const dx = W / (data.length - 1)
    return data
      .map((v, i) => `${i ? 'L' : 'M'}${(i * dx).toFixed(1)},${(height - (v / maxv) * height).toFixed(1)}`)
      .join(' ')
  }
  const fmt = (v) =>
    v >= 1e6 ? (v / 1e6).toFixed(1) + 'M' : v >= 1e3 ? (v / 1e3).toFixed(1) + 'k' : v.toFixed(0)
</script>

<div class="relative">
  <svg viewBox={`0 0 ${W} ${height}`} class="w-full" style={`height:${height}px`} preserveAspectRatio="none">
    <line x1="0" y1={height - 0.5} x2={W} y2={height - 0.5} class="stroke-base-300" stroke-width="1" />
    {#each series as s (s.label)}
      <path d={path(s.data)} fill="none" stroke={s.color} stroke-width="2" />
    {/each}
  </svg>
  <span class="absolute top-0 right-0 text-xs opacity-60">{fmt(maxv)} {unit}</span>
</div>
<div class="flex flex-wrap gap-3 text-xs mt-1">
  {#each series as s (s.label)}
    <span class="flex items-center gap-1">
      <span class="inline-block w-3 h-2 rounded-sm" style={`background:${s.color}`}></span>{s.label}
    </span>
  {/each}
</div>
