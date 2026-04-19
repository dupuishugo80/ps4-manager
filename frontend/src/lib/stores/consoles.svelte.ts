import { DiscoveryEvent, listConsoles, onDiscoveryEvent, type Console } from '../api'

export function createConsolesStore() {
  let consoles = $state<Console[]>([])
  const unsubscribers: Array<() => void> = []

  function upsert(console: Console) {
    const index = consoles.findIndex((existing) => existing.ip === console.ip)
    if (index === -1) {
      consoles = [...consoles, console]
      return
    }
    consoles = consoles.map((existing, i) => (i === index ? console : existing))
  }

  function remove(console: Console) {
    consoles = consoles.filter((existing) => existing.ip !== console.ip)
  }

  async function bootstrap() {
    const snapshot = await listConsoles()
    consoles = snapshot ?? []
    unsubscribers.push(onDiscoveryEvent(DiscoveryEvent.Found, upsert))
    unsubscribers.push(onDiscoveryEvent(DiscoveryEvent.Lost, remove))
  }

  function dispose() {
    while (unsubscribers.length > 0) {
      const off = unsubscribers.pop()
      off?.()
    }
  }

  return {
    get value() {
      return consoles
    },
    bootstrap,
    dispose,
  }
}
