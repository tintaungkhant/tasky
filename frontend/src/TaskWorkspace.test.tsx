import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, it, vi } from 'vitest'
import TaskWorkspace from './TaskWorkspace'

const root = { ID: 'root/internal', Name: 'Root', Path: 'C:/root' }
const task = { ID: 'task/internal', RootID: root.ID, TaskID: 'T-1', Name: 'Task' }

it('generates the workspace with internal IDs and exposes the returned path', async () => {
  const user = userEvent.setup()
  const fetch = vi.fn().mockResolvedValue(new Response(JSON.stringify({ Path: 'C:/root/.tasky/workspace.md', Current: false })))
  vi.stubGlobal('fetch', fetch)
  render(<TaskWorkspace root={root} task={task} />)
  await user.click(screen.getByRole('button', { name: 'Generate editor workspace' }))
  expect(fetch).toHaveBeenCalledWith('/api/tasks/workspace', expect.objectContaining({
    method: 'POST', body: JSON.stringify({ RootID: root.ID, TaskID: task.ID }),
  }))
  expect(await screen.findByLabelText('Workspace file path')).toHaveValue('C:/root/.tasky/workspace.md')
  expect(screen.getByText(/generated a new workspace file/i)).toBeInTheDocument()
  expect(screen.getByText(/Open the file in a compatible editor/)).toBeInTheDocument()
  expect(screen.getByText(/Identical files are retained/)).toBeInTheDocument()
  expect(screen.getByText(/Move the previous file aside before regenerating/)).toBeInTheDocument()
})

it('reports when the workspace file is already current', async () => {
  const user = userEvent.setup()
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(JSON.stringify({ Path: 'C:/workspace.md', Current: true }))))
  render(<TaskWorkspace root={root} task={task} />)
  await user.click(screen.getByRole('button', { name: 'Generate editor workspace' }))
  expect(await screen.findByText(/already current/i)).toBeInTheDocument()
  expect(screen.getByLabelText('Workspace file path')).toHaveValue('C:/workspace.md')
})

it('shows full request diagnostics and preserves safety guidance after failure', async () => {
  const user = userEvent.setup()
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(JSON.stringify({ error: { message: 'permission denied: C:/private/workspace.md' } }), { status: 500 })))
  render(<TaskWorkspace root={root} task={task} />)
  await user.click(screen.getByRole('button', { name: 'Generate editor workspace' }))
  expect(await screen.findByRole('alert')).toHaveTextContent('permission denied: C:/private/workspace.md')
  expect(screen.getByText(/differing user files are preserved/i)).toBeInTheDocument()
  expect(screen.getByText(/Move the previous file aside before regenerating/)).toBeInTheDocument()
})

it('clears stale success data when a later generation fails', async () => {
  const user = userEvent.setup()
  const fetch = vi.fn()
    .mockResolvedValueOnce(new Response(JSON.stringify({ Path: 'C:/old.md', Current: true })))
    .mockResolvedValueOnce(new Response(JSON.stringify({ error: { message: 'disk full' } }), { status: 500 }))
  vi.stubGlobal('fetch', fetch)
  render(<TaskWorkspace root={root} task={task} />)
  await user.click(screen.getByRole('button', { name: 'Generate editor workspace' }))
  expect(await screen.findByLabelText('Workspace file path')).toHaveValue('C:/old.md')
  await user.click(screen.getByRole('button', { name: 'Generate editor workspace' }))
  expect(await screen.findByRole('alert')).toHaveTextContent('disk full')
  expect(screen.queryByLabelText('Workspace file path')).not.toBeInTheDocument()
  expect(screen.queryByText(/already current/i)).not.toBeInTheDocument()
})

it('disables generation while the request is pending', async () => {
  const user = userEvent.setup()
  let resolve!: (response: Response) => void
  vi.stubGlobal('fetch', vi.fn().mockReturnValue(new Promise<Response>((done) => { resolve = done })))
  render(<TaskWorkspace root={root} task={task} />)
  await user.click(screen.getByRole('button', { name: 'Generate editor workspace' }))
  expect(screen.getByRole('button', { name: 'Generating workspace…' })).toBeDisabled()
  resolve(new Response(JSON.stringify({ Path: 'C:/workspace.md', Current: true })))
})
