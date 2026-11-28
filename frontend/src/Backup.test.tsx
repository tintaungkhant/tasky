import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, it, vi } from 'vitest'
import Backup from './Backup'

it('uploads the selected snapshot and refreshes Roots while retaining location warnings', async () => {
  const user = userEvent.setup()
  let respond!: (response: Response) => void
  const fetch = vi.fn().mockReturnValue(new Promise<Response>((resolve) => { respond = resolve }))
  vi.stubGlobal('fetch', fetch)
  const refresh = vi.fn().mockResolvedValue(undefined)
  render(<Backup onRestored={refresh} />)

  const restore = screen.getByRole('button', { name: 'Restore mappings' })
  expect(restore).toBeDisabled()
  const file = new File(['{"version":1}'], 'cli-backup.json', { type: 'application/json' })
  await user.upload(screen.getByLabelText('Mapping snapshot'), file)
  restore.focus()
  await user.keyboard('{Enter}')
  expect(fetch).toHaveBeenCalledWith('/api/restore', {
    method: 'POST', headers: { 'Content-Type': 'application/json' }, body: file,
  })
  expect(screen.getByRole('status')).toHaveTextContent('Restoring mappings')
  expect(restore).toBeDisabled()
  expect(screen.getByRole('button', { name: 'Download mapping backup' })).toBeDisabled()
  respond(new Response(JSON.stringify({
    Warnings: [{ Kind: 'worktree', ID: 'w1', Path: 'C:/old', Status: 'missing', Detail: 'Inspect or reconnect this checkout.' }],
  })))
  expect(await screen.findByText(/C:\/old.*missing.*reconnect/)).toBeInTheDocument()
  expect(screen.getByText(/Existing mappings were preserved/)).toBeInTheDocument()
  expect(refresh).toHaveBeenCalledOnce()
})

it('shows recovery guidance for a blocked download and fetches a fresh snapshot on retry', async () => {
  const user = userEvent.setup()
  const fetch = vi.fn()
    .mockResolvedValueOnce(new Response(JSON.stringify({ error: { message: 'Resolve pending operations in Recovery.' } }), { status: 409 }))
    .mockResolvedValueOnce(new Response('{"version":1}'))
  vi.stubGlobal('fetch', fetch)
  vi.stubGlobal('URL', Object.assign(class extends URL {}, { createObjectURL: vi.fn(() => 'blob:backup'), revokeObjectURL: vi.fn() }))
  const downloads: string[] = []
  const click = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(function (this: HTMLAnchorElement) {
    downloads.push(this.download)
  })
  try {
    render(<Backup onRestored={async () => {}} />)
    const download = screen.getByRole('button', { name: 'Download mapping backup' })
    await user.click(download)
    expect(await screen.findByRole('alert')).toHaveTextContent('Resolve pending operations in Recovery.')
    expect(downloads).toEqual([])
    await user.click(download)
    expect(fetch.mock.calls).toEqual([['/api/backup'], ['/api/backup']])
    expect(downloads).toEqual(['tasky-mappings.json'])
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  } finally {
    click.mockRestore()
  }
})
