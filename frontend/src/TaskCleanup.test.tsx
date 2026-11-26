import { act, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, it, vi } from 'vitest'
import TaskCleanup from './TaskCleanup'

const root = { ID: 'root-1', Name: 'Root', Path: '/root' }
const mappings = [{ ID: 'map-1', TaskID: 'task-1', RepoID: 'repo-1', Path: '/checkout', Identity: 'verified' }]
const review = { RootID: root.ID, Targets: [{ Mapping: mappings[0], Branch: 'feature', Revision: 'abc123', RepoIdentity: 'repo-proof' }], Options: { DeleteBranch: false, ForceWorktree: false, ForceBranch: false } }

it('requires fresh explicit consent for the exact preview and shows partial cleanup results', async () => {
  const user = userEvent.setup()
  const fetch = vi.fn().mockImplementation((url: string) => Promise.resolve(new Response(JSON.stringify(url.endsWith('/preview') ? review : { Completed: [], Failed: mappings[0], Error: 'dirty checkout', Remaining: mappings }))))
  vi.stubGlobal('fetch', fetch)
  const onChanged = vi.fn()
  render(<TaskCleanup root={root} mappings={mappings} onChanged={onChanged} />)
  await user.click(screen.getByRole('button', { name: /remove worktrees/i }))
  await user.click(screen.getByLabelText(/\/checkout/))
  await user.click(screen.getByRole('button', { name: /review cleanup/i }))
  expect(await screen.findByText(/repo-proof/)).toBeInTheDocument()
  await user.click(screen.getByLabelText(/I understand/i))
  await user.click(screen.getByRole('button', { name: /clean up selected/i }))
  expect(await screen.findByText(/dirty checkout/)).toBeInTheDocument()
  expect(screen.getByText('/checkout')).toBeInTheDocument()
  expect(onChanged).not.toHaveBeenCalled()
  const body = JSON.parse(fetch.mock.calls[1][1].body)
  expect(body).toMatchObject({ Confirm: true, Review: review })
})

it('invalidates preview consent when selection changes', async () => {
  const user = userEvent.setup()
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(JSON.stringify(review))))
  render(<TaskCleanup root={root} mappings={[...mappings, { ...mappings[0], ID: 'map-2', Path: '/second' }]} onChanged={vi.fn()} />)
  await user.click(screen.getByRole('button', { name: /remove worktrees/i }))
  await user.click(screen.getByLabelText(/\/checkout/))
  await user.click(screen.getByRole('button', { name: /review cleanup/i }))
  await screen.findByText(/repo-proof/)
  await user.click(screen.getByLabelText(/I understand/i))
  await user.click(screen.getByLabelText(/\/second/))
  expect(screen.queryByLabelText(/I understand/i)).not.toBeInTheDocument()
})

it('invalidates consent when an override changes', async () => {
  const user = userEvent.setup()
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(JSON.stringify(review))))
  render(<TaskCleanup root={root} mappings={mappings} onChanged={vi.fn()} />)
  await user.click(screen.getByRole('button', { name: /remove worktrees/i }))
  await user.click(screen.getByLabelText(/\/checkout/))
  await user.click(screen.getByRole('button', { name: /review cleanup/i }))
  await screen.findByText(/repo-proof/)
  await user.click(screen.getByLabelText(/I understand/i))
  await user.click(screen.getByLabelText(/delete each selected/i))
  expect(screen.queryByLabelText(/I understand/i)).not.toBeInTheDocument()
})

it('locks options during preview and requires a new review after a failed cleanup attempt', async () => {
  const user = userEvent.setup()
  let finishPreview!: (response: Response) => void
  const previewPending = new Promise<Response>((resolve) => { finishPreview = resolve })
  const fetch = vi.fn().mockImplementation((url: string) => url.endsWith('/preview') ? previewPending : Promise.resolve(new Response(JSON.stringify({ error: { message: 'pending cleanup; inspect recovery' } }), { status: 500 })))
  vi.stubGlobal('fetch', fetch)
  render(<TaskCleanup root={root} mappings={mappings} onChanged={vi.fn()} />)
  await user.click(screen.getByRole('button', { name: /remove worktrees/i }))
  await user.click(screen.getByLabelText(/\/checkout/))
  await user.click(screen.getByRole('button', { name: /review cleanup/i }))
  expect(screen.getByLabelText(/delete each selected/i)).toBeDisabled()
  await act(async () => { finishPreview(new Response(JSON.stringify(review))) })
  await user.click(screen.getByLabelText(/I understand/i))
  await user.click(screen.getByRole('button', { name: /clean up selected/i }))
  expect(await screen.findByRole('alert')).toHaveTextContent(/pending cleanup/)
  expect(screen.queryByLabelText(/I understand/i)).not.toBeInTheDocument()
  expect(screen.getByRole('button', { name: /review cleanup/i })).toBeEnabled()
})
