import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import App from './App'

const roots = [
  { ID: 'one', Name: 'same', Path: 'C:/one' },
  { ID: 'two', Name: 'same', Path: 'C:/two' },
]

describe('dashboard shell', () => {
  it('connects through the production API sequence before showing registered Roots', async () => {
    history.replaceState(null, '', '/#token=test-access')
    const fetch = vi.fn().mockResolvedValueOnce(new Response(null, { status: 204 }))
      .mockResolvedValueOnce(new Response(JSON.stringify(roots)))
    vi.stubGlobal('fetch', fetch)
    render(<App />)
    expect(await screen.findByRole('combobox')).toBeEnabled()
    expect(fetch.mock.calls.map(([url]) => url)).toEqual(['/api/session', '/api/roots'])
    expect(location.hash).toBe('')
  })

  it('waits for the session, requires explicit identity selection, and supports keyboard navigation', async () => {
    const user = userEvent.setup()
    const fetch = vi.fn().mockResolvedValue(new Response(JSON.stringify(roots)))
    vi.stubGlobal('fetch', fetch)
    let connect!: () => void
    const connection = new Promise<void>((resolve) => { connect = resolve })
    render(<App connection={() => connection} />)
    expect(screen.getByRole('status')).toHaveTextContent('Connecting')
    expect(fetch).not.toHaveBeenCalled()
    connect()
    const select = await screen.findByRole('combobox', { name: 'Root' })
    expect(select).toHaveValue('')
    await user.selectOptions(select, 'two')
    expect(select).toHaveValue('two')
    expect(screen.getByText('Selected Root: same — C:/two (two)')).toBeInTheDocument()
    const tasks = screen.getByRole('tab', { name: 'Tasks' })
    tasks.focus()
    await user.keyboard('{ArrowRight}')
    expect(screen.getByRole('tab', { name: 'Registration' })).toHaveFocus()
    expect(screen.getByRole('tabpanel')).toHaveTextContent('Registration')
  })

  it('shows empty Roots and refreshes without keeping a removed selection', async () => {
    const user = userEvent.setup()
    const fetch = vi.fn().mockResolvedValueOnce(new Response(JSON.stringify(roots)))
      .mockResolvedValueOnce(new Response('[]'))
    vi.stubGlobal('fetch', fetch)
    render(<App connection={() => Promise.resolve()} />)
    await user.selectOptions(await screen.findByRole('combobox'), 'one')
    await user.click(screen.getByRole('button', { name: 'Refresh Roots' }))
    expect(await screen.findByText(/No registered Roots/)).toBeInTheDocument()
    expect(screen.queryByText(/^Selected Root:/)).not.toBeInTheDocument()
  })

  it('shows server errors and allows retry', async () => {
    const user = userEvent.setup()
    vi.stubGlobal('fetch', vi.fn().mockResolvedValueOnce(new Response(JSON.stringify({ error: { message: 'Database unavailable' } }), { status: 500 }))
      .mockResolvedValueOnce(new Response(JSON.stringify(roots))))
    render(<App connection={() => Promise.resolve()} />)
    expect(await screen.findByRole('alert')).toHaveTextContent('Database unavailable')
    await user.click(screen.getByRole('button', { name: 'Refresh Roots' }))
    expect(await screen.findByRole('combobox')).toBeEnabled()
  })

  it('gives reconnect instructions when the session expires', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response('{}', { status: 401 })))
    render(<App connection={() => Promise.resolve()} />)
    await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent('Dashboard URL printed by tasky serve'))
  })
})
