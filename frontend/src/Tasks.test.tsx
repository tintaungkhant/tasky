import { act, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, it, vi } from 'vitest'
import Tasks from './Tasks'

const root = { ID: 'root/two', Name: 'same', Path: 'C:/two' }
const task = { ID: 'internal', RootID: root.ID, TaskID: 'T-1', Name: 'မြန်မာ 日本語' }
const rows = [{ Task: task, Worktrees: [{ Mapping: { ID: 'wt', Path: 'C:/checkout' }, Repo: { ID: 'repo', Name: 'api', Path: 'C:/repo' }, State: { Status: 'ok', Branch: 'feature', Changes: '?? new.txt', Detail: '' } }] }]

it('requires an explicit Root before requesting Tasks', () => {
  const fetch = vi.fn()
  vi.stubGlobal('fetch', fetch)
  render(<Tasks />)
  expect(screen.getByText(/Choose a Root/)).toBeInTheDocument()
  expect(fetch).not.toHaveBeenCalled()
})

it('searches Unicode names and IDs, selects with keyboard, and inspects live changes', async () => {
  const user = userEvent.setup()
  const fetch = vi.fn().mockImplementation(() => Promise.resolve(new Response(JSON.stringify(rows))))
  vi.stubGlobal('fetch', fetch)
  render(<Tasks root={root} />)
  const select = await screen.findByRole('combobox', { name: 'Task' })
  expect(fetch).toHaveBeenCalledWith('/api/tasks?root=root%2Ftwo')
  expect(select).toHaveValue('')
  await user.type(screen.getByRole('searchbox'), '日本語')
  await user.tab()
  expect(select).toHaveFocus()
  await user.selectOptions(select, task.ID)
  expect(screen.getByText('C:/checkout')).toBeInTheDocument()
  expect(screen.getByText('C:/repo')).toBeInTheDocument()
  expect(screen.getByText('feature')).toBeInTheDocument()
  expect(screen.getByText(/Dirty or untracked changes/)).toBeInTheDocument()
  expect(screen.getByText('?? new.txt')).toBeInTheDocument()
  fetch.mockImplementation(() => Promise.resolve(new Response(JSON.stringify([{ ...rows[0], Worktrees: [{ ...rows[0].Worktrees[0], State: { Status: 'ok', Branch: '(detached HEAD)', Changes: '', Detail: '' } }] }]))))
  await user.click(screen.getByRole('button', { name: 'Refresh Tasks' }))
  expect(await screen.findByText('(detached HEAD)')).toBeInTheDocument()
  expect(screen.getByText('Clean')).toBeInTheDocument()
  await user.clear(screen.getByRole('searchbox'))
  await user.type(screen.getByRole('searchbox'), 't-1')
  expect(screen.getByRole('option', { name: /T-1/ })).toBeInTheDocument()
  await user.type(screen.getByRole('searchbox'), 'no-match')
  expect(screen.getByText('No matching Tasks.')).toBeInTheDocument()
  expect(screen.queryByText('C:/checkout')).not.toBeInTheDocument()
  await user.clear(screen.getByRole('searchbox'))
  expect(screen.getByText('C:/checkout')).toBeInTheDocument()
  fetch.mockImplementation(() => Promise.resolve(new Response('[]')))
  await user.click(screen.getByRole('button', { name: 'Refresh Tasks' }))
  expect(await screen.findByText(/No Tasks in this Root/)).toBeInTheDocument()
  expect(screen.queryByText('C:/checkout')).not.toBeInTheDocument()
})

it('shows loading, errors, retry, empty Tasks, and Tasks without worktrees', async () => {
  const user = userEvent.setup()
  const fetch = vi.fn().mockRejectedValueOnce(new Error('Database unavailable'))
    .mockImplementationOnce(() => Promise.resolve(new Response('[]')))
    .mockImplementationOnce(() => Promise.resolve(new Response(JSON.stringify([{ Task: task, Worktrees: [] }]))))
  vi.stubGlobal('fetch', fetch)
  render(<Tasks root={root} />)
  expect(screen.getByRole('status')).toHaveTextContent('Loading Tasks')
  expect(await screen.findByRole('alert')).toHaveTextContent('Database unavailable')
  await user.click(screen.getByRole('button', { name: 'Refresh Tasks' }))
  expect(await screen.findByText(/No Tasks in this Root/)).toBeInTheDocument()
  await user.click(screen.getByRole('button', { name: 'Refresh Tasks' }))
  await user.selectOptions(await screen.findByRole('combobox', { name: 'Task' }), task.ID)
  expect(screen.getByText('No associated worktrees.')).toBeInTheDocument()
})

it('shows unhealthy mapping guidance without inventing trusted Git state', async () => {
  const user = userEvent.setup()
  vi.stubGlobal('fetch', vi.fn().mockImplementation(() => Promise.resolve(new Response(JSON.stringify([{ Task: task, Worktrees: ['missing', 'inaccessible', 'invalid', 'unverified'].map((status) => ({ Mapping: { ID: status, Path: `C:/${status}` }, Repo: rows[0].Worktrees[0].Repo, State: { Status: status, Branch: '', Changes: '', Detail: `Review ${status} then reconnect` } })) }])))))
  render(<Tasks root={root} />)
  await user.selectOptions(await screen.findByRole('combobox', { name: 'Task' }), task.ID)
  for (const status of ['missing', 'inaccessible', 'invalid', 'unverified']) {
    expect(screen.getByText(`Review ${status} then reconnect`)).toBeInTheDocument()
  }
  expect(screen.queryByText('Clean')).not.toBeInTheDocument()
})

it('discards old Root responses and stale Task details on Root changes', async () => {
  let resolve!: (value: Response) => void
  const pending = new Promise<Response>((done) => { resolve = done })
  const fetch = vi.fn().mockReturnValueOnce(pending).mockImplementation(() => Promise.resolve(new Response('[]')))
  vi.stubGlobal('fetch', fetch)
  const view = render(<Tasks root={root} />)
  view.rerender(<Tasks root={{ ...root, ID: 'other' }} />)
  await screen.findByText(/No Tasks in this Root/)
  await act(async () => { resolve(new Response(JSON.stringify(rows))) })
  await waitFor(() => expect(screen.queryByRole('option', { name: /T-1/ })).not.toBeInTheDocument())
})

it('refreshes discovery after creation and refreshes the selected Task after rename', async () => {
  const user = userEvent.setup()
  let current: typeof rows = []
  const fetch = vi.fn().mockImplementation((url: string, options?: RequestInit) => {
    let body: unknown = current
    if (url.startsWith('/api/repos?')) body = [{ ID: 'repo', Name: 'api', Path: 'C:/repo' }]
    if (url.startsWith('/api/tasks/options?')) body = { Branches: ['main'], CurrentBranch: 'main', TasksDir: 'C:/tasks', BranchTemplate: '{branch}' }
    if (url === '/api/tasks/create') {
      current = rows
      body = { Path: 'C:/checkout', Branch: 'feature' }
    }
    if (url === '/api/tasks/rename') {
      const payload = JSON.parse(options!.body as string)
      current = [{ ...rows[0], Task: { ...task, Name: payload.Name } }]
      body = { Message: 'Renamed' }
    }
    return Promise.resolve(new Response(JSON.stringify(body)))
  })
  vi.stubGlobal('fetch', fetch)
  render(<Tasks root={root} />)
  await screen.findByText(/No Tasks in this Root/)
  await user.click(screen.getByRole('button', { name: 'Add worktree' }))
  await user.selectOptions(await screen.findByLabelText('Repo'), 'repo')
  await user.type(await screen.findByLabelText('Task ID'), 'T-1')
  await user.type(screen.getByLabelText('Branch'), 'feature')
  await user.click(screen.getByRole('button', { name: 'Create worktree' }))
  await user.selectOptions(await screen.findByRole('combobox', { name: 'Task' }), task.ID)
  await user.clear(screen.getByLabelText('Task name'))
  await user.type(screen.getByLabelText('Task name'), 'Renamed')
  await user.click(screen.getByRole('button', { name: 'Rename Task' }))
  expect(await screen.findByRole('heading', { name: 'T-1 — Renamed' })).toBeInTheDocument()
  expect(screen.getByText('C:/checkout')).toBeInTheDocument()
})
