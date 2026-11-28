import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, expect, it, vi } from 'vitest'
import Registration from './Registration'

const root = { ID: 'root-two', Name: 'same', Path: 'C:/two' }
const cfg = { Roots: [{ Name: 'same', Path: 'C:/two', TasksDir: 'C:/tasks', BranchTemplate: '{branch}' }] }
beforeEach(() => {
  Object.defineProperty(HTMLDialogElement.prototype, 'showModal', {
    configurable: true, value(this: HTMLDialogElement) { this.setAttribute('open', '') },
  })
  Object.defineProperty(HTMLDialogElement.prototype, 'close', {
    configurable: true, value(this: HTMLDialogElement) {
      this.removeAttribute('open')
      this.dispatchEvent(new Event('close'))
    },
  })
})
function mockAPI() {
  const fetch = vi.fn().mockImplementation((url: string, options?: RequestInit) => {
    if (options?.method) return Promise.resolve(new Response('{}'))
    if (url === '/api/preferences') return Promise.resolve(new Response(JSON.stringify(cfg)))
    if (url.startsWith('/api/folders')) return Promise.resolve(new Response(JSON.stringify({
      Path: 'C:/tasks/new', Parent: 'C:/tasks', Directories: [],
      Locations: [{ Name: 'Start folder', Path: 'C:/app' }],
    })))
    if (url.includes('/removal')) return Promise.resolve(new Response(JSON.stringify({ Root: root, Repos: [], Tasks: [{ ID: 'task-one', TaskID: 'T-1', Name: 'Keep code' }], Worktrees: [] })))
    return Promise.resolve(new Response('[]'))
  })
  vi.stubGlobal('fetch', fetch)
  return fetch
}

it('edits YAML preferences independently and sends the original snapshot', async () => {
  const fetch = mockAPI(), user = userEvent.setup()
  render(<Registration root={root} onChanged={async () => {}} />)
  const form = await screen.findByRole('form', { name: 'YAML preferences' })
  const template = within(form).getByLabelText('Branch Template')
  await user.clear(template); await user.type(template, 'task/')
  await user.click(within(form).getByRole('button', { name: 'Save YAML preferences' }))
  const call = fetch.mock.calls.find(([, options]) => options?.method === 'PUT')!
  expect(call[0]).toBe('/api/preferences')
  expect(JSON.parse(call[1].body as string)).toEqual({ expected: cfg, config: { Roots: [{ ...cfg.Roots[0], BranchTemplate: 'task/' }] } })
})

it('browses a YAML Tasks Directory without changing other preferences or adding a Branch Template browser', async () => {
  const fetch = mockAPI(), user = userEvent.setup()
  render(<Registration onChanged={async () => {}} />)
  const form = await screen.findByRole('form', { name: 'YAML preferences' })
  await user.click(within(form).getByRole('button', { name: 'Browse Tasks Directory' }))
  const dialog = screen.getByRole('dialog', { name: 'Choose a folder for Tasks Directory' })
  await within(dialog).findByRole('button', { name: 'Start folder' })
  await user.click(within(dialog).getByRole('button', { name: 'Select this folder' }))
  expect(within(form).getByLabelText('Tasks Directory')).toHaveValue('C:/tasks/new')
  expect(within(form).getByLabelText('Root path')).toHaveValue('C:/two')
  expect(within(form).queryByRole('button', { name: 'Browse Branch Template' })).not.toBeInTheDocument()
  await user.click(within(form).getByRole('button', { name: 'Save YAML preferences' }))
  const call = fetch.mock.calls.find(([, options]) => options?.method === 'PUT')!
  expect(JSON.parse(call[1].body as string)).toEqual({ expected: cfg, config: { Roots: [{ ...cfg.Roots[0], TasksDir: 'C:/tasks/new' }] } })
})

it('keeps the selected registration path in FormData and resets it after a successful registration', async () => {
  const fetch = mockAPI(), user = userEvent.setup()
  render(<Registration onChanged={async () => {}} />)
  const form = await screen.findByRole('form', { name: 'Register Root' })
  await user.type(within(form).getByLabelText('Name'), 'new root')
  await user.click(within(form).getByRole('button', { name: 'Browse Path' }))
  const dialog = screen.getByRole('dialog', { name: 'Choose a folder for Path' })
  await within(dialog).findByRole('button', { name: 'Start folder' })
  await user.click(within(dialog).getByRole('button', { name: 'Select this folder' }))
  expect(within(form).getByLabelText('Path')).toHaveValue('C:/tasks/new')
  await user.click(within(form).getByRole('button', { name: 'Register Root' }))
  const call = fetch.mock.calls.find(([url]) => url === '/api/registration/roots')!
  expect(JSON.parse(call[1].body as string)).toEqual({ Name: 'new root', Path: 'C:/tasks/new' })
  expect(within(form).getByLabelText('Path')).toHaveValue('')
})

it('requires a preview and explicit confirmation before forgetting the selected Root identity', async () => {
  const fetch = mockAPI(), user = userEvent.setup()
  render(<Registration root={root} onChanged={async () => {}} />)
  await screen.findByRole('form', { name: 'YAML preferences' })
  await user.click(screen.getByRole('button', { name: 'Preview Root removal' }))
  expect(await screen.findByText(/T-1.*Keep code/)).toBeInTheDocument()
  const remove = screen.getByRole('button', { name: 'Forget these mappings' })
  expect(remove).toBeDisabled()
  await user.click(screen.getByRole('checkbox', { name: /I confirm/ }))
  await user.click(remove)
  const call = fetch.mock.calls.find(([url]) => url === '/api/registration/remove')!
  expect(JSON.parse(call[1].body as string)).toMatchObject({ confirm: true, expected: { Root: { ID: 'root-two' } } })
})

it('shows mutation errors without discarding entered values, and supports repeatable import', async () => {
  const fetch = mockAPI(), user = userEvent.setup()
  render(<Registration onChanged={async () => {}} />)
  const form = await screen.findByRole('form', { name: 'Register Root' })
  await user.type(within(form).getByLabelText('Name'), 'same')
  await user.type(within(form).getByLabelText('Path'), 'C:/root')
  fetch.mockResolvedValueOnce(new Response(JSON.stringify({ error: { message: 'Root directory is unavailable' } }), { status: 400 }))
  await user.click(within(form).getByRole('button', { name: 'Register Root' }))
  expect(await screen.findByRole('alert')).toHaveTextContent('Root directory is unavailable')
  expect(within(form).getByLabelText('Path')).toHaveValue('C:/root')
  await user.click(screen.getByRole('button', { name: 'Import legacy data' }))
  await user.click(screen.getByRole('button', { name: 'Import legacy data' }))
  expect(fetch.mock.calls.filter(([url]) => url === '/api/import')).toHaveLength(2)
})

it('registers a Repo in the selected Root using its identity', async () => {
  const fetch = mockAPI(), user = userEvent.setup()
  render(<Registration root={root} onChanged={async () => {}} />)
  const form = await screen.findByRole('form', { name: 'Register Repo' })
  await user.type(within(form).getByLabelText('Name'), 'nested')
  await user.type(within(form).getByLabelText('Path'), 'C:/elsewhere/repo')
  await user.click(within(form).getByRole('button', { name: 'Register Repo' }))
  const call = fetch.mock.calls.find(([url]) => url === '/api/registration/repos')!
  expect(JSON.parse(call[1].body as string)).toEqual({ RootID: root.ID, Name: 'nested', Path: 'C:/elsewhere/repo' })
})

it('shows loading and load errors, and reloads registration data', async () => {
  const fetch = mockAPI(), user = userEvent.setup()
  fetch.mockRejectedValueOnce(new Error('Preferences unavailable'))
  render(<Registration onChanged={async () => {}} />)
  expect(screen.getByRole('status')).toHaveTextContent('Loading registration')
  expect(await screen.findByRole('alert')).toHaveTextContent('Preferences unavailable')
  await user.click(screen.getByRole('button', { name: 'Reload registration' }))
  expect(await screen.findByLabelText('Tasks Directory')).toHaveValue('C:/tasks')
})

it('removes only the chosen YAML entry when names repeat', async () => {
  const fetch = mockAPI(), user = userEvent.setup()
  const second = { ...cfg.Roots[0], Path: 'C:/other', TasksDir: 'C:/other-tasks' }
  const original = { Roots: [...cfg.Roots, second] }
  fetch.mockResolvedValueOnce(new Response(JSON.stringify(original)))
  render(<Registration onChanged={async () => {}} />)
  await screen.findByDisplayValue('C:/other')
  await user.click(screen.getByRole('button', { name: 'Remove YAML entry 1' }))
  await user.click(screen.getByRole('button', { name: 'Save YAML preferences' }))
  const call = fetch.mock.calls.find(([, options]) => options?.method === 'PUT')!
  expect(JSON.parse(call[1].body as string)).toEqual({ expected: original, config: { Roots: [second] } })
  expect(fetch.mock.calls.some(([url]) => url === '/api/registration/remove')).toBe(false)
})
