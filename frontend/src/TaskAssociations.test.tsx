import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, it, vi } from 'vitest'
import TaskAssociations, { type Mapping } from './TaskAssociations'
const root = { ID: 'root-1', Name: 'Root', Path: '/root' }
const mapping: Mapping = { ID: 'map-1', TaskID: 'owner-internal', RepoID: 'repo-1', Path: '/old/path', Identity: 'verified' }
const review = { RootID: root.ID, RepoID: 'repo-1', TaskID: 'T-2', Name: 'new', TargetID: 'target-internal', Path: '/canonical/path', Identity: 'verified', Source: mapping, SourceTask: { ID: 'owner-internal', RootID: root.ID, TaskID: 'T-1', Name: 'owner' }, SourceRoot: { ID: root.ID, Name: 'Root', Path: '/root' } }
function setup(override?: (url: string, options?: RequestInit) => unknown) {
  const fetch = vi.fn().mockImplementation((url: string, options?: RequestInit) => {
    if (override) { const value = override(url, options); return Promise.resolve(value instanceof Response ? value : new Response(JSON.stringify(value))) }
    if (url.startsWith('/api/repos?')) return Promise.resolve(new Response(JSON.stringify([{ ID: 'repo-1', Name: 'api', Path: '/repo' }])))
    if (url === '/api/tasks/attach/preview') return Promise.resolve(new Response(JSON.stringify(review)))
    return Promise.resolve(new Response('{}'))
  }); vi.stubGlobal('fetch', fetch); return fetch
}
it('previews ownership and posts explicit reassignment using the source internal Task ID', async () => {
  const user = userEvent.setup(), fetch = setup(), changed = vi.fn(); render(<TaskAssociations root={root} onChanged={changed} />)
  await user.selectOptions(await screen.findByLabelText('Repo'), 'repo-1'); await user.type(screen.getByLabelText('Task ID'), 'T-2'); await user.type(screen.getByLabelText('Absolute checkout path'), '/requested')
  await user.click(screen.getByRole('button', { name: 'Review attachment' })); expect(await screen.findByText('/canonical/path')).toBeInTheDocument(); expect(screen.getByText(/owner-internal/)).toBeInTheDocument()
  const confirm = screen.getByRole('button', { name: 'Attach worktree' }); expect(confirm).toBeDisabled(); await user.click(screen.getByRole('checkbox', { name: /Reassign/ })); await user.click(confirm)
  await waitFor(() => expect(fetch).toHaveBeenCalledWith('/api/tasks/attach', expect.objectContaining({ method: 'POST', body: JSON.stringify({ Review: review, ReassignFrom: 'owner-internal' }) })))
  expect(changed).toHaveBeenCalled()
})
it('discards review on edits and requires a fresh review after failed attach', async () => {
  const user = userEvent.setup(), fetch = setup((url) => url === '/api/repos?root=root-1' ? [{ ID: 'repo-1', Name: 'api', Path: '/repo' }] : url === '/api/tasks/attach/preview' ? review : url === '/api/tasks/attach' ? new Response(JSON.stringify({ error: { message: 'commit failed: retry review' } }), { status: 500 }) : {})
  render(<TaskAssociations root={root} onChanged={vi.fn()} />); await user.selectOptions(await screen.findByLabelText('Repo'), 'repo-1'); await user.type(screen.getByLabelText('Task ID'), 'T-2'); await user.type(screen.getByLabelText('Absolute checkout path'), '/requested'); await user.click(screen.getByRole('button', { name: 'Review attachment' })); await screen.findByText('/canonical/path'); await user.click(screen.getByRole('checkbox', { name: /Reassign/ })); await user.click(screen.getByRole('button', { name: 'Attach worktree' }))
  await user.click(screen.getByRole('button', { name: 'Review attachment' }))
  expect(await screen.findByText('/canonical/path')).toBeInTheDocument()
  await user.type(screen.getByLabelText('Absolute checkout path'), '-edited')
  expect(screen.queryByText('/canonical/path')).not.toBeInTheDocument()
  await user.click(screen.getByRole('button', { name: 'Review attachment' }))
  await screen.findByText('/canonical/path')
  await user.click(screen.getByRole('checkbox', { name: /Reassign/ }))
  await user.click(screen.getByRole('button', { name: 'Attach worktree' }))
  expect(await screen.findByRole('alert')).toHaveTextContent('retry review'); expect(screen.queryByText('/canonical/path')).not.toBeInTheDocument(); expect(fetch).toHaveBeenCalledWith('/api/tasks/attach/preview', expect.objectContaining({ method: 'POST' }))
})
it('reconnects unverified Mapping with explicit enrollment and exact Mapping identity', async () => {
  const user = userEvent.setup(), fetch = setup((url) => url === '/api/tasks/reconnect' ? {} : []), changed = vi.fn(), unverified = { ...mapping, Identity: '' }
  render(<TaskAssociations root={root} mapping={unverified} onChanged={changed} />); await user.type(screen.getByLabelText('New checkout path'), '/new/path'); const reconnect = screen.getByRole('button', { name: 'Review and reconnect' }); expect(reconnect).toBeDisabled(); await user.click(screen.getByRole('checkbox', { name: /enroll/i })); expect(reconnect).toBeEnabled(); await user.click(reconnect)
  await waitFor(() => expect(fetch).toHaveBeenCalledWith('/api/tasks/reconnect', expect.objectContaining({ method: 'POST', body: JSON.stringify({ RootID: root.ID, Mapping: unverified, Path: '/new/path', AcceptIdentity: true }) }))); expect(changed).toHaveBeenCalled()
})
it('detaches exact Mapping and refreshes after success', async () => {
  const user = userEvent.setup(), changed = vi.fn(), fetch = setup(() => ({})); render(<TaskAssociations root={root} mapping={mapping} onChanged={changed} />)
  expect(screen.getByRole('button', { name: 'Detach worktree' })).toBeDisabled()
  await user.click(screen.getByRole('checkbox', { name: /Confirm detachment/ }))
  await user.click(screen.getByRole('button', { name: 'Detach worktree' }))
  await waitFor(() => expect(fetch).toHaveBeenCalledWith('/api/tasks/detach', expect.objectContaining({ method: 'POST', body: JSON.stringify({ RootID: root.ID, Mapping: mapping, Confirm: true }) }))); expect(await screen.findByText(/Files and branches are preserved/)).toBeInTheDocument(); expect(changed).toHaveBeenCalled()
})
it('keeps detach diagnostics and does not refresh after failure', async () => {
  const user = userEvent.setup(), changed = vi.fn(); setup((url) => url === '/api/tasks/detach' ? new Response(JSON.stringify({ error: { message: 'mapping is protected' } }), { status: 409 }) : [])
  render(<TaskAssociations root={root} mapping={mapping} onChanged={changed} />)
  await user.click(screen.getByRole('checkbox', { name: /Confirm detachment/ }))
  await user.click(screen.getByRole('button', { name: 'Detach worktree' }))
  expect(await screen.findByRole('alert')).toHaveTextContent('mapping is protected')
  expect(changed).not.toHaveBeenCalled()
})
