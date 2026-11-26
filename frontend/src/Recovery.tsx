import { useEffect, useState } from 'react'
import { Button } from '@/components/ui/button'
import { request, type Root } from './api'

type Creation = { ID: string; RootID: string; TaskID: string; RepoID: string; Branch: string; Path: string }
type Cleanup = Creation & { WorktreeID: string; Identity: string; RepoIdentity: string; Revision: string; DeleteBranch: boolean; ForceWorktree: boolean; ForceBranch: boolean; Removed: boolean }
type Pending = { Creations: Creation[]; Cleanups: Cleanup[] }
type RecoveryOptions = { Forget: boolean; ForceWorktree: boolean; ForceBranch: boolean }
const empty: Pending = { Creations: [], Cleanups: [] }

export default function Recovery({ root }: { root?: Root }) {
  return root ? <RootRecovery key={root.ID} root={root} /> : <section aria-label="Recovery">
    <h2 className="text-lg font-medium">Recovery</h2>
    <p role="status">Select a Root above to view pending operations.</p>
  </section>
}

function RootRecovery({ root }: { root: Root }) {
  const [pending, setPending] = useState<Pending>(empty)
  const [loading, setLoading] = useState(true)
  const [attempt, setAttempt] = useState(0)
  const [error, setError] = useState('')
  const [checked, setChecked] = useState<Record<string, boolean>>({})
  const [options, setOptions] = useState<Record<string, RecoveryOptions>>({})
  const [busy, setBusy] = useState('')
  const [message, setMessage] = useState('')
  function refresh() {
    setChecked({})
    setLoading(true); setError('')
    setAttempt((value) => value + 1)
  }
  useEffect(() => {
    let active = true
    request<Pending>(`/api/pending?root=${encodeURIComponent(root.ID)}`)
      .then((value) => { if (active) setPending(value) })
      .catch((reason: unknown) => { if (active) { setPending(empty); setError(reason instanceof Error ? reason.message : 'Could not load pending operations.') } })
      .finally(() => { if (active) setLoading(false) })
    return () => { active = false }
  }, [root.ID, attempt])
  const update = (id: string, value: Partial<RecoveryOptions>) => {
    setChecked((current) => ({ ...current, [id]: false }))
    setOptions((current) => {
      const existing = current[id] ?? { Forget: false, ForceWorktree: false, ForceBranch: false }
      return { ...current, [id]: { ...existing, ...value } }
    })
  }
  const recover = async (id: string) => {
    if (!checked[id] || loading || busy) return
    const choice = options[id] ?? { Forget: false, ForceWorktree: false, ForceBranch: false }
    setBusy(id); setError(''); setMessage('')
    try {
      await request('/api/recover', 'POST', { RootID: root.ID, ID: id, Options: choice, Confirm: true })
      setMessage(`${choice.Forget ? 'Forgot' : 'Recovered'} operation ${id}.`)
      refresh()
    } catch (reason) { setChecked((current) => ({ ...current, [id]: false })); setError(reason instanceof Error ? reason.message : `Could not recover operation ${id}. Inspect Git and the pending operation before retrying.`) }
    finally { setBusy('') }
  }
  const action = (id: string, cleanup: boolean, path: string) => {
    const choice = options[id] ?? { Forget: false, ForceWorktree: false, ForceBranch: false }
    return <div className="space-y-2 border-t pt-2">
      <fieldset disabled={!!busy || loading} className="space-y-2"><legend>Operation action</legend>
      <label className="flex gap-2"><input type="radio" name={`intent-${id}`} checked={!choice.Forget} onChange={() => update(id, { Forget: false })} />Recover</label>
      <label className="flex gap-2"><input type="radio" name={`intent-${id}`} checked={choice.Forget} onChange={() => update(id, { Forget: true, ForceWorktree: false, ForceBranch: false })} />Forget intent</label>
      {cleanup && !choice.Forget && <fieldset className="space-y-1"><legend>Additional recovery overrides</legend>
        <label className="flex gap-2"><input type="checkbox" checked={choice.ForceWorktree} onChange={(event) => update(id, { ForceWorktree: event.target.checked })} />Also force dirty worktree</label>
        <label className="flex gap-2"><input type="checkbox" checked={choice.ForceBranch} onChange={(event) => update(id, { ForceBranch: event.target.checked })} />Also force unmerged branch</label>
      </fieldset>}
      <p>{choice.Forget ? `Forgetting abandons intent only. Files, mappings, and branches at ${path} are preserved; you may need to detach a stale mapping manually.` : 'Recovery continues the recorded operation with the selected overrides.'}</p>
      <label className="flex gap-2"><input type="checkbox" checked={!!checked[id]} onChange={(event) => setChecked((current) => ({ ...current, [id]: event.target.checked }))} />Confirm recovery action for {id}</label></fieldset>
      <Button disabled={!checked[id] || !!busy || loading} onClick={() => void recover(id)}>{choice.Forget ? `Forget ${id}` : `Recover ${id}`}</Button>
    </div>
  }
  return <section className="space-y-4" aria-label="Recovery">
    <h2 className="text-lg font-medium">Recovery</h2>
    <p>Root: {root.Path} ({root.ID})</p><p>Recorded force overrides remain active when retrying an operation. Forcing worktree removal discards uncommitted and untracked files inside that checkout; Repo identity checks are never bypassed.</p><Button variant="outline" disabled={loading || !!busy} onClick={refresh}>Refresh pending operations</Button>
      {loading && <p role="status">Loading pending operations…</p>}{busy && <p role="status">Working on operation {busy}…</p>}{error && <p role="alert">{error}</p>}{message && <p role="status">{message}</p>}
      {!loading && !error && pending.Creations.length === 0 && pending.Cleanups.length === 0 && <p role="status">No pending operations for this Root.</p>}
      <div className="space-y-3"><h3 className="font-medium">Pending creations</h3>{pending.Creations.map((item) => <article key={item.ID} className="space-y-2 rounded border p-3 break-all"><p>Operation: {item.ID}</p><p>Path: {item.Path}</p><p>Branch: {item.Branch}</p><p>Creation may have stopped before the checkout and mapping were fully recorded. Inspect the path and Git state before recovering.</p>{action(item.ID, false, item.Path)}</article>)}</div>
      <div className="space-y-3"><h3 className="font-medium">Pending cleanups</h3>{pending.Cleanups.map((item) => <article key={item.ID} className="space-y-2 rounded border p-3 break-all"><p>Operation: {item.ID}; Task: {item.TaskID}; Repo: {item.RepoID}; worktree: {item.WorktreeID}</p><p>Path: {item.Path}</p><p>Branch: {item.Branch || 'detached HEAD'}; revision: {item.Revision}; identity: {item.Identity}; Repo identity: {item.RepoIdentity}</p><p>Phase: worktree {item.Removed ? 'removed' : 'not recorded removed'}; branch {item.DeleteBranch && item.Branch ? 'deletion requested' : 'preserved'}.</p><p>Recorded overrides: dirty worktree {String(item.ForceWorktree)}; unmerged branch {String(item.ForceBranch)}. Inspect Git state before continuing.</p>{action(item.ID, true, item.Path)}</article>)}</div>
  </section>
}
