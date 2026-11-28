import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import FolderPicker from './FolderPicker'

const listing = (path: string, parent: string, names: string[]) => ({
  Path: path, Parent: parent, Directories: names.map((name) => ({ Name: name, Path: `${path}/${name}` })),
  Locations: [{ Name: 'Home', Path: '/home/user' }, { Name: 'Start folder', Path: '/app' }, { Name: 'Filesystem', Path: '/' }],
})

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
afterEach(() => vi.unstubAllGlobals())

function mockFetch(handler: (url: string) => Response | Promise<Response>) {
  const fetch = vi.fn((url: string) => Promise.resolve(handler(url)))
  vi.stubGlobal('fetch', fetch)
  return fetch
}

it('browses exact paths, navigates into a directory and selects only after a successful listing', async () => {
  const fetch = mockFetch((url) => new Response(JSON.stringify(url === '/api/folders?path=%2Fhome%2Fuser%2Fprojects'
    ? listing('/home/user/projects', '/home/user', ['tasky'])
    : listing('/home/user/projects/tasky', '/home/user/projects', ['src']))))
  let selectedPath = ''
  const onChange = vi.fn((event: React.ChangeEvent<HTMLInputElement>) => { selectedPath = event.target.value })
  const user = userEvent.setup()
  render(<FolderPicker label="Path" value="/home/user/projects" onChange={onChange} />)
  await user.click(screen.getByRole('button', { name: 'Browse Path' }))
  const dialog = screen.getByRole('dialog', { name: 'Choose a folder for Path' })
  expect(await within(dialog).findByRole('button', { name: 'tasky' })).toBeInTheDocument()
  expect(fetch).toHaveBeenCalledWith('/api/folders?path=%2Fhome%2Fuser%2Fprojects')
  await user.click(within(dialog).getByRole('button', { name: 'tasky' }))
  await waitFor(() => expect(within(dialog).getByText('/home/user/projects/tasky')).toBeInTheDocument())
  await user.click(within(dialog).getByRole('button', { name: 'Select this folder' }))
  expect(onChange).toHaveBeenCalledOnce()
  expect(selectedPath).toBe('/home/user/projects/tasky')
  expect(dialog).not.toBeVisible()
})

it('keeps the typed path on cancel and reports an initial error while allowing a start-folder retry', async () => {
  mockFetch((url) => url === '/api/folders?path=bad'
    ? new Response(JSON.stringify({ error: { message: 'Directory not found' } }), { status: 400 })
    : new Response(JSON.stringify(listing('/app', '', ['data']))))
  const onChange = vi.fn()
  const user = userEvent.setup()
  render(<FolderPicker label="Path" value="bad" onChange={onChange} />)
  await user.click(screen.getByRole('button', { name: 'Browse Path' }))
  const dialog = screen.getByRole('dialog', { name: 'Choose a folder for Path' })
  expect(await within(dialog).findByRole('alert')).toHaveTextContent('Directory not found')
  expect(within(dialog).getByRole('button', { name: 'Select this folder' })).toBeDisabled()
  await user.click(within(dialog).getByRole('button', { name: 'Start folder' }))
  expect(await within(dialog).findByRole('button', { name: 'data' })).toBeInTheDocument()
  await user.click(within(dialog).getByRole('button', { name: 'Cancel' }))
  expect(onChange).not.toHaveBeenCalled()
})

it('does not submit a parent form when browsing or navigating', async () => {
  mockFetch((url) => new Response(JSON.stringify(listing(url.includes('path=') ? '/app' : '/app', '', ['data']))))
  const submit = vi.fn((event: React.FormEvent) => event.preventDefault())
  const user = userEvent.setup()
  render(<form onSubmit={submit}><FolderPicker label="Path" value="" onChange={() => {}} /><button>Submit</button></form>)
  await user.click(screen.getByRole('button', { name: 'Browse Path' }))
  const dialog = screen.getByRole('dialog')
  await user.click(await within(dialog).findByRole('button', { name: 'data' }))
  expect(submit).not.toHaveBeenCalled()
})

it('ignores a listing response from a canceled picker session', async () => {
  let finishOld: ((response: Response) => void) | undefined
  let calls = 0
  mockFetch(() => {
    calls += 1
    if (calls === 1) return new Promise<Response>((resolve) => { finishOld = resolve })
    return new Response(JSON.stringify(listing('/new', '', ['fresh'])))
  })
  const user = userEvent.setup()
  render(<FolderPicker label="Path" value="/start" onChange={() => {}} />)
  await user.click(screen.getByRole('button', { name: 'Browse Path' }))
  const dialog = screen.getByRole('dialog')
  await user.click(within(dialog).getByRole('button', { name: 'Cancel' }))
  await user.click(screen.getByRole('button', { name: 'Browse Path' }))
  expect(await within(dialog).findByRole('button', { name: 'fresh' })).toBeInTheDocument()
  finishOld?.(new Response(JSON.stringify(listing('/old', '', ['stale']))))
  await waitFor(() => expect(within(dialog).getByText('/new')).toBeInTheDocument())
  expect(within(dialog).queryByRole('button', { name: 'stale' })).not.toBeInTheDocument()
})
