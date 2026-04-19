<script lang="ts">
  import { Greet } from '../wailsjs/go/main/App.js'

  let name = $state('')
  let greeting = $state('')

  async function greet() {
    greeting = await Greet(name)
  }
</script>

<main class="flex min-h-screen flex-col items-center justify-center gap-6 p-8">
  <h1 class="text-3xl font-bold">PS4 Manager</h1>

  <form
    class="flex gap-2"
    onsubmit={(event) => {
      event.preventDefault()
      greet()
    }}
  >
    <input
      type="text"
      bind:value={name}
      autocomplete="off"
      placeholder="Your name"
      aria-label="Your name"
      class="rounded border border-slate-600 bg-slate-800 px-3 py-2 outline-none focus:border-blue-500"
    />
    <button type="submit" class="rounded bg-blue-600 px-4 py-2 font-semibold hover:bg-blue-700">
      Greet
    </button>
  </form>

  {#if greeting}
    <p data-testid="greeting" class="text-lg">{greeting}</p>
  {/if}
</main>
