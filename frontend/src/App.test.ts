import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/svelte'
import userEvent from '@testing-library/user-event'
import App from './App.svelte'

vi.mock('../wailsjs/go/main/App.js', () => ({
  Greet: vi.fn(async (name: string) => `Hello ${name}`),
}))

describe('App', () => {
  it('renders the app title', () => {
    render(App)
    expect(screen.getByRole('heading', { name: /ps4 manager/i })).toBeInTheDocument()
  })

  it('calls the Greet binding and displays the greeting', async () => {
    const user = userEvent.setup()
    render(App)

    await user.type(screen.getByLabelText(/your name/i), 'world')
    await user.click(screen.getByRole('button', { name: /greet/i }))

    expect(await screen.findByTestId('greeting')).toHaveTextContent('Hello world')
  })
})
