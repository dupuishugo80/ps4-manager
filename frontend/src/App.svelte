<script lang="ts">
  import { onMount } from 'svelte'
  import { createConsolesStore } from './lib/stores/consoles.svelte'

  const store = createConsolesStore()

  onMount(() => {
    store.bootstrap()
    return () => store.dispose()
  })
</script>

<main class="flex min-h-screen flex-col gap-6 p-8">
  <header class="flex items-baseline justify-between">
    <h1 class="text-3xl font-bold">PS4 Manager</h1>
    <span class="text-sm text-slate-400" data-testid="console-count">
      {store.value.length} console{store.value.length === 1 ? '' : 's'} detected
    </span>
  </header>

  <section aria-label="Detected consoles">
    {#if store.value.length === 0}
      <p class="text-slate-400" data-testid="consoles-empty">
        No PS4 detected. Launch GoldHEN and the Remote Package Installer on the console.
      </p>
    {:else}
      <ul class="flex flex-col gap-2" data-testid="consoles-list">
        {#each store.value as console (console.ip)}
          <li
            class="flex items-center justify-between rounded border border-slate-700 bg-slate-800 px-4 py-3"
            data-testid="console-item"
          >
            <div class="flex flex-col">
              <span class="font-mono text-lg">{console.ip}</span>
              <span class="text-xs text-slate-400">port {console.port}</span>
            </div>
            <span class="h-3 w-3 rounded-full bg-emerald-500" aria-label="online"></span>
          </li>
        {/each}
      </ul>
    {/if}
  </section>
</main>
