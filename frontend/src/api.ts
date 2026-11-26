export type Root = { ID: string; Name: string; Path: string }

const reconnect = 'Open the Dashboard URL printed by tasky serve to connect.'

async function check(response: Response): Promise<Response> {
  if (response.status === 401) throw new Error(reconnect)
  if (!response.ok) {
    const body = await response.json()
    throw new Error(body.error?.message ?? 'The local service could not complete the request.')
  }
  return response
}

export async function request<T>(path: string, method = 'GET', body?: unknown): Promise<T> {
  const options = method === 'GET' ? undefined : {
    method, headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body),
  }
  return (await check(await (options ? fetch(path, options) : fetch(path)))).json()
}

export async function connect(): Promise<void> {
  const token = new URLSearchParams(location.hash.slice(1)).get('token')
  // Remove the access credential before doing any asynchronous work.
  history.replaceState(null, '', location.pathname)
  if (token) {
    await check(await fetch('/api/session', {
      method: 'POST', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ token }),
    }))
  } else {
    await check(await fetch('/api/status'))
  }
}

export async function listRoots(): Promise<Root[]> {
  return (await check(await fetch('/api/roots'))).json()
}
