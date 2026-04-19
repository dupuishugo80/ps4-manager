import { describe, expect, it, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/svelte'
import App from './App.svelte'

type Handler = (payload: unknown) => void
const handlers: Record<string, Handler[]> = {}

vi.mock('../wailsjs/go/main/App.js', () => ({
  GetConsoles: vi.fn(async () => []),
}))

vi.mock('../wailsjs/runtime/runtime.js', () => ({
  EventsOn: vi.fn((event: string, handler: Handler) => {
    handlers[event] ??= []
    handlers[event].push(handler)
    return () => {
      handlers[event] = handlers[event].filter((registered) => registered !== handler)
    }
  }),
}))

function emit(event: string, payload: unknown) {
  for (const handler of handlers[event] ?? []) {
    handler(payload)
  }
}

describe('App', () => {
  beforeEach(() => {
    for (const key of Object.keys(handlers)) {
      delete handlers[key]
    }
  })

  it('renders the app title', () => {
    render(App)
    expect(screen.getByRole('heading', { name: /ps4 manager/i })).toBeInTheDocument()
  })

  it('shows the empty state when no console is detected', async () => {
    render(App)
    expect(await screen.findByTestId('consoles-empty')).toBeInTheDocument()
    expect(screen.getByTestId('console-count')).toHaveTextContent('0 console')
  })

  it('renders consoles returned by the initial snapshot', async () => {
    const { GetConsoles } = await import('../wailsjs/go/main/App.js')
    vi.mocked(GetConsoles).mockResolvedValueOnce([
      { ip: '192.168.1.10', port: 12800, seen_at: '', last_ping: '' },
    ] as never)
    render(App)
    expect(await screen.findByText('192.168.1.10')).toBeInTheDocument()
    expect(screen.getByTestId('console-count')).toHaveTextContent('1 console')
  })

  it('adds a console when a discovery:found event fires', async () => {
    render(App)
    await screen.findByTestId('consoles-empty')
    emit('discovery:found', { ip: '10.0.0.5', port: 12800, seen_at: '', last_ping: '' })
    expect(await screen.findByText('10.0.0.5')).toBeInTheDocument()
  })

  it('removes a console when a discovery:lost event fires', async () => {
    const { GetConsoles } = await import('../wailsjs/go/main/App.js')
    vi.mocked(GetConsoles).mockResolvedValueOnce([
      { ip: '10.0.0.5', port: 12800, seen_at: '', last_ping: '' },
    ] as never)
    render(App)
    await screen.findByText('10.0.0.5')
    emit('discovery:lost', { ip: '10.0.0.5', port: 12800, seen_at: '', last_ping: '' })
    await waitFor(() => expect(screen.queryByText('10.0.0.5')).not.toBeInTheDocument())
  })
})
