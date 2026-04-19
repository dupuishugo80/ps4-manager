import { GetConsoles } from '../../wailsjs/go/main/App.js'
import { EventsOn } from '../../wailsjs/runtime/runtime.js'
import type { discovery } from '../../wailsjs/go/models'

export type Console = discovery.Console

export const DiscoveryEvent = {
  Found: 'discovery:found',
  Lost: 'discovery:lost',
} as const

export function listConsoles(): Promise<Console[]> {
  return GetConsoles()
}

export function onDiscoveryEvent(
  event: (typeof DiscoveryEvent)[keyof typeof DiscoveryEvent],
  handler: (console: Console) => void,
): () => void {
  return EventsOn(event, (payload: Console) => handler(payload))
}
