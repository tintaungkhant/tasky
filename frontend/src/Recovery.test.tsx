import { act, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, it, vi } from 'vitest'
import Recovery from './Recovery'

const root = { ID: 'root-1', Name: 'Root', Path: '/root' }
it('lists pending creation and cleanup; requires consent and sends separate cleanup overrides', async () => {
  const user = userEvent.setup()
  const pending = { Creations: [{ ID: 'create-1', RootID: root.ID, TaskID: 'task', RepoID: 'repo', Branch: 'feature', Path: '/new' }], Cleanups: [{ ID: 'cleanup-1', RootID: root.ID, TaskID: 'task', RepoID: 'repo', WorktreeID: 'wt', Identity: 'identity', RepoIdentity: 'repo-proof', Branch: 'old', Path: '/old', Revision: 'abc', DeleteBranch: true, ForceWorktree: false, ForceBranch: true, Removed: false }] }
  const fetch = vi.fn().mockImplementation((url: string) => Promise.resolve(new Response(JSON.stringify(url === '/api/pending?root=root-1' ? pending : {}))))
  vi.stubGlobal('fetch', fetch)
  render(<Recovery root={root} />)
  expect(await screen.findByText(/Path: \/new/)).toBeInTheDocument()
  expect(screen.getByText(/Path: \/old/)).toBeInTheDocument()
  await user.click(screen.getByLabelText('Also force dirty worktree'))
  await user.click(screen.getByLabelText('Confirm recovery action for cleanup-1'))
  await user.click(screen.getByRole('button', { name: /recover cleanup-1/i }))
  await waitFor(() => expect(fetch.mock.calls.some((call: unknown[]) => call[0] === '/api/recover')).toBe(true))
  expect(JSON.parse(fetch.mock.calls[1][1].body)).toMatchObject({ RootID: root.ID, ID: 'cleanup-1', Confirm: true, Options: { Forget: false, ForceWorktree: true, ForceBranch: false } })
})

it('resets action confirmation when Root changes and supports forgetting intent', async () => {
  const user = userEvent.setup()
  const fetch = vi.fn().mockImplementation(() => Promise.resolve(new Response(JSON.stringify({ Creations: [], Cleanups: [{ ID: 'c', Path: '/pending', Branch: 'b' }] }))))
  vi.stubGlobal('fetch', fetch)
  const { rerender } = render(<Recovery root={root} />)
  await screen.findByText(/Path: \/pending/)
  await user.click(screen.getByLabelText('Confirm recovery action for c'))
  rerender(<Recovery root={{ ...root, ID: 'root-2' }} />)
  expect(await screen.findByLabelText('Confirm recovery action for c')).not.toBeChecked()
  await user.click(screen.getByLabelText(/forget intent/i))
  await user.click(screen.getByLabelText('Confirm recovery action for c'))
  await user.click(screen.getByRole('button', { name: /forget c/i }))
  await waitFor(() => expect(fetch.mock.calls.some((call: unknown[]) => call[0] === '/api/recover')).toBe(true))
  const actionCall = fetch.mock.calls.find((call: unknown[]) => call[0] === '/api/recover')
  expect(actionCall).toBeDefined()
  expect(JSON.parse(actionCall![1].body)).toMatchObject({ RootID: 'root-2', ID: 'c', Options: { Forget: true, ForceWorktree: false, ForceBranch: false } })
})

it('ignores a pending list response from the previous Root', async () => {
  let finishOld!: (response: Response) => void
  const oldPending = new Promise<Response>((resolve) => { finishOld = resolve })
  const fetch = vi.fn((url: string) => url.includes('root-1') ? oldPending : Promise.resolve(new Response(JSON.stringify({ Creations: [], Cleanups: [] }))))
  vi.stubGlobal('fetch', fetch)
  const { rerender } = render(<Recovery root={root} />)
  rerender(<Recovery root={{ ...root, ID: 'root-2' }} />)
  await waitFor(() => expect(fetch.mock.calls.some((call: unknown[]) => call[0] === '/api/pending?root=root-2')).toBe(true))
  await act(async () => { finishOld(new Response(JSON.stringify({ Creations: [{ ID: 'stale', Path: '/stale' }], Cleanups: [] }))) })
  expect(screen.queryByText(/\/stale/)).not.toBeInTheDocument()
  expect(screen.getByText(/No pending operations/)).toBeInTheDocument()
})

it('clears confirmation after a failed recovery attempt', async () => {
  const user = userEvent.setup()
  const fetch = vi.fn().mockImplementation((url: string) => url === '/api/recover'
    ? Promise.resolve(new Response(JSON.stringify({ error: { message: 'inspect pending operation' } }), { status: 409 }))
    : Promise.resolve(new Response(JSON.stringify({ Creations: [{ ID: 'failed-create', Path: '/failed', Branch: 'b' }], Cleanups: [] }))))
  vi.stubGlobal('fetch', fetch)
  render(<Recovery root={root} />)
  await screen.findByText(/Path: \/failed/)
  await user.click(screen.getByLabelText('Confirm recovery action for failed-create'))
  await user.click(screen.getByRole('button', { name: /recover failed-create/i }))
  expect(await screen.findByRole('alert')).toHaveTextContent(/inspect pending operation/)
  expect(screen.getByLabelText('Confirm recovery action for failed-create')).not.toBeChecked()
})
