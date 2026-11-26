import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, it, vi } from 'vitest'
import Registration from './Registration'

const root = { ID: 'root-two', Name: 'same', Path: 'C:/two' }
const cfg = { Roots: [{ Name: 'same', Path: 'C:/two', TasksDir: 'C:/tasks', BranchTemplate: '{branch}' }] }
function mockAPI() {
  const fetch = vi.fn().mockImplementation((url: string, options?: RequestInit) => {
    if (options?.method) return Promise.resolve(new Response('{}'))
    if (url === '/api/preferences') return Promise.resolve(new Response(JSON.stringify(cfg)))
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
