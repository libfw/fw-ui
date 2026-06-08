<script>
  import { onMount } from 'svelte'

  const themes = [
    'dark', 'light', 'night', 'dracula', 'synthwave', 'cyberpunk', 'business', 'emerald',
  ]
  let current = 'dark'

  function apply(t) {
    document.documentElement.setAttribute('data-theme', t)
    localStorage.setItem('fwui-theme', t)
  }
  onMount(() => {
    current = localStorage.getItem('fwui-theme') || 'dark'
    apply(current)
  })
</script>

<select
  class="select select-bordered select-xs"
  bind:value={current}
  on:change={() => apply(current)}
  title="theme"
>
  {#each themes as t (t)}
    <option value={t}>{t}</option>
  {/each}
</select>
