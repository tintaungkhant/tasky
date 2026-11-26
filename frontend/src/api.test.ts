import { afterEach, expect, it, vi } from 'vitest'
import { connect } from './api'

afterEach(() => history.replaceState(null, '', '/'))

it('removes the fragment before exchanging it and sends it only in the session body', async () => {
  history.replaceState(null, '', '/#token=test-access')
  const fetch = vi.fn().mockImplementation(() => {
    expect(location.hash).toBe('')
    return Promise.resolve(new Response(null, { status: 204 }))
  })
  vi.stubGlobal('fetch', fetch)
  await connect()
  expect(fetch).toHaveBeenCalledWith('/api/session', expect.objectContaining({
    method: 'POST', body: JSON.stringify({ token: 'test-access' }),
  }))
})

it('uses the existing cookie when no access fragment is present', async () => {
  const fetch = vi.fn().mockResolvedValue(new Response('{"status":"ok"}'))
  vi.stubGlobal('fetch', fetch)
  await connect()
  expect(fetch).toHaveBeenCalledWith('/api/status')
})

it('shows reconnect instructions for an invalid access link', async () => {
  history.replaceState(null, '', '/#token=invalid')
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response('{}', { status: 401 })))
  await expect(connect()).rejects.toThrow('Dashboard URL printed by tasky serve')
  expect(location.hash).toBe('')
})
