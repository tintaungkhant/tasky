import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, it, vi } from 'vitest'
import TaskCreation from './TaskCreation'
import TaskRename from './TaskRename'

const root = { ID: 'root-two', Name: 'same', Path: 'C:/root' }
const repos = [{ ID: 'repo-two', Name: 'api', Path: 'C:/external' }]
function mockAPI() {
  const fetch = vi.fn().mockImplementation((url: string, options?: RequestInit) => {
    if (options?.method) return Promise.resolve(new Response(JSON.stringify({ Path: 'C:/tasks/.tasky/task/wt', Branch: 'task/T-1/fix' })))
    return Promise.resolve(new Response(JSON.stringify(url.startsWith('/api/repos?') ? repos : { Branches: ['main', 'existing'], CurrentBranch: 'main', TasksDir: 'C:/tasks', BranchTemplate: 'task/{task-id}/{branch}' })))
  })
  vi.stubGlobal('fetch', fetch)
  return fetch
}

it('creates using explicit identities, template or verbatim branch, and base without losing inputs on failure', async () => {
  const user = userEvent.setup(), fetch = mockAPI(), changed = vi.fn()
  render(<TaskCreation root={root} onCreated={changed} />)
  await user.selectOptions(await screen.findByLabelText('Repo'), repos[0].ID)
  const form = await screen.findByRole('form', { name: 'Create worktree' })
  expect(await screen.findByText('C:/tasks')).toBeInTheDocument()
  expect(screen.getByText('task/{task-id}/{branch}')).toBeInTheDocument()
  expect(within(form).getByLabelText('Base branch')).toHaveValue('')
  await user.type(within(form).getByLabelText('Task ID'), 'T-1')
  await user.type(within(form).getByLabelText('Task name (new Tasks only)'), '日本語')
  await user.type(within(form).getByLabelText('Branch'), 'fix')
  await user.type(within(form).getByLabelText('Base branch'), 'existing')
  fetch.mockResolvedValueOnce(new Response(JSON.stringify({ error: { message: 'creation op-1 pending at C:/partial; tasky recover op-1' } }), { status: 500 }))
  await user.click(within(form).getByRole('button', { name: 'Create worktree' }))
  expect(await screen.findByRole('alert')).toHaveTextContent('tasky recover op-1')
  expect(within(form).getByLabelText('Branch')).toHaveValue('fix')
  expect(changed).not.toHaveBeenCalled()
  await user.click(within(form).getByRole('checkbox', { name: /Use branch verbatim/ }))
  await user.click(within(form).getByRole('button', { name: 'Create worktree' }))
  expect(await screen.findByText(/Created worktree.*C:\/tasks/)).toBeInTheDocument()
  expect(changed).toHaveBeenCalledOnce()
  const calls = fetch.mock.calls.filter(([url]) => url === '/api/tasks/create')
  expect(JSON.parse(calls[0][1].body as string)).toEqual({ RootID: root.ID, RepoID: repos[0].ID, TaskID: 'T-1', Name: '日本語', Branch: 'fix', Base: 'existing', AsIs: false })
  expect(JSON.parse(calls[1][1].body as string).AsIs).toBe(true)
})

it('handles registration loading failures with retry and empty Repos', async () => {
  const user = userEvent.setup(), fetch = mockAPI()
  fetch.mockRejectedValueOnce(new Error('Repos unavailable'))
  render(<TaskCreation root={root} onCreated={() => {}} />)
  expect(screen.getByRole('status')).toHaveTextContent('Loading Repos')
  expect(await screen.findByRole('alert')).toHaveTextContent('Repos unavailable')
  fetch.mockImplementation(() => Promise.resolve(new Response('[]')))
  await user.click(screen.getByRole('button', { name: 'Reload Repos' }))
  expect(await screen.findByText(/No Repos registered/)).toBeInTheDocument()
})

it('blocks creation when options fail and reloads effective settings', async () => {
  const user = userEvent.setup(), fetch = mockAPI()
  render(<TaskCreation root={root} onCreated={() => {}} />)
  await screen.findByRole('option', { name: /api/ })
  fetch.mockRejectedValueOnce(new Error('Repo missing'))
  await user.selectOptions(screen.getByLabelText('Repo'), repos[0].ID)
  expect(await screen.findByRole('alert')).toHaveTextContent('Repo missing')
  expect(screen.queryByRole('button', { name: 'Create worktree' })).not.toBeInTheDocument()
  await user.click(screen.getByRole('button', { name: 'Refresh branch options' }))
  expect(await screen.findByRole('button', { name: 'Create worktree' })).toBeEnabled()
})

it('renames the exact selected internal Task identity and allows an empty name', async () => {
  const user = userEvent.setup(), fetch = mockAPI(), changed = vi.fn()
  render(<TaskRename root={root} task={{ ID: 'internal', TaskID: 'T-1', Name: '日本語' }} onRenamed={changed} />)
  const input = screen.getByLabelText('Task name')
  await user.clear(input)
  fetch.mockResolvedValueOnce(new Response(JSON.stringify({ error: { message: 'Task no longer exists; refresh' } }), { status: 404 }))
  await user.click(screen.getByRole('button', { name: 'Rename Task' }))
  expect(await screen.findByRole('alert')).toHaveTextContent('Task no longer exists')
  expect(input).toHaveValue('')
  expect(changed).not.toHaveBeenCalled()
  await user.click(screen.getByRole('button', { name: 'Rename Task' }))
  expect(changed).toHaveBeenCalledOnce()
  const call = fetch.mock.calls.find(([url]) => url === '/api/tasks/rename')!
  expect(JSON.parse(call[1].body as string)).toEqual({ RootID: root.ID, ID: 'internal', Name: '' })
})
