import { useEffect, useState } from 'react'
import { Button } from '@/components/ui/button'
import { request, type Root } from './api'

type Repo = { ID: string; Name: string; Path: string }
type Options = { Branches: string[]; CurrentBranch: string; TasksDir: string; BranchTemplate: string }

export default function TaskCreation({ root, onCreated }: { root: Root; onCreated: () => void }) {
  const [repos, setRepos] = useState<Repo[]>([])
  const [repoID, setRepoID] = useState('')
  const [attempt, setAttempt] = useState(0)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const repo = repos.find((item) => item.ID === repoID)
  useEffect(() => {
    let active = true
    request<Repo[]>(`/api/repos?root=${encodeURIComponent(root.ID)}`).then((values) => {
      if (active) setRepos(values)
    }).catch((reason: unknown) => {
      if (active) setError(reason instanceof Error ? reason.message : 'Cannot load Repos. Retry after checking the service.')
    }).finally(() => { if (active) setLoading(false) })
    return () => { active = false }
  }, [root.ID, attempt])
  return <section aria-label="Worktree creation" className="space-y-3 rounded-md border p-3">
    <h3 className="font-medium">Add a worktree to a new or existing Task</h3>
    <Button variant="outline" disabled={loading} onClick={() => {
      setLoading(true); setError(''); setRepoID(''); setAttempt((value) => value + 1)
    }}>Reload Repos</Button>
    {loading ? <p role="status">Loading Repos…</p> : error ? <p role="alert">{error}</p> : repos.length === 0 ? <p>No Repos registered. Add a Repo in Registration first.</p> : <>
      <label htmlFor="create-repo" className="block">Repo</label>
      <select id="create-repo" value={repoID} onChange={(event) => setRepoID(event.target.value)} className="w-full rounded-md border bg-background p-2">
        <option value="">Select a Repo</option>
        {repos.map((item) => <option key={item.ID} value={item.ID}>{item.Name} — {item.Path} ({item.ID})</option>)}
      </select>
      {repo && <CreationForm key={repo.ID} root={root} repo={repo} onCreated={onCreated} />}
    </>}
  </section>
}

function CreationForm({ root, repo, onCreated }: { root: Root; repo: Repo; onCreated: () => void }) {
  const [options, setOptions] = useState<Options>()
  const [attempt, setAttempt] = useState(0)
  const [loading, setLoading] = useState(true)
  const [loadError, setLoadError] = useState('')
  const [taskID, setTaskID] = useState('')
  const [name, setName] = useState('')
  const [branch, setBranch] = useState('')
  const [base, setBase] = useState('')
  const [asIs, setAsIs] = useState(false)
  const [busy, setBusy] = useState(false)
  const [message, setMessage] = useState('')
  const [error, setError] = useState('')
  useEffect(() => {
    let active = true
    request<Options>(`/api/tasks/options?root=${encodeURIComponent(root.ID)}&repo=${encodeURIComponent(repo.ID)}`).then((value) => {
      if (active) setOptions(value)
    }).catch((reason: unknown) => {
      if (active) setLoadError(reason instanceof Error ? reason.message : 'Cannot load branch options. Refresh and retry.')
    }).finally(() => { if (active) setLoading(false) })
    return () => { active = false }
  }, [root.ID, repo.ID, attempt])
  return <div className="space-y-3">
    <Button variant="outline" disabled={loading || busy} onClick={() => {
      setLoading(true); setLoadError(''); setAttempt((value) => value + 1)
    }}>Refresh branch options</Button>
    {loading ? <p role="status">Loading branch options…</p> : loadError ? <p role="alert">{loadError}</p> : options && <form aria-label="Create worktree" className="space-y-3" onSubmit={async (event) => {
      event.preventDefault()
      if (busy) return
      setBusy(true); setError(''); setMessage('')
      try {
        const result = await request<{ Path: string; Branch: string }>('/api/tasks/create', 'POST', { RootID: root.ID, RepoID: repo.ID, TaskID: taskID, Name: name, Branch: branch, Base: base, AsIs: asIs })
        setMessage(`Created worktree ${result.Path} on branch ${result.Branch}.`)
        onCreated()
      } catch (reason) {
        setError(reason instanceof Error ? reason.message : 'Creation failed. Inspect tasky pending before retrying; some effects may have completed.')
      } finally { setBusy(false) }
    }}>
      <dl><dt>Tasks Directory</dt><dd className="break-words">{options.TasksDir}</dd><dt>Branch Template</dt><dd>{options.BranchTemplate}</dd></dl>
      <p>Existing branches are reused verbatim. New branches use the Branch Template unless checked below. For an existing Task ID, its saved name is kept. Repeat this form to add more worktrees or Repos.</p>
      <fieldset disabled={busy} className="space-y-2">
        <label className="block">Task ID<input required value={taskID} onChange={(event) => setTaskID(event.target.value)} className="block w-full rounded-md border p-2" /></label>
        <label className="block">Task name (new Tasks only)<input value={name} onChange={(event) => setName(event.target.value)} className="block w-full rounded-md border p-2" /></label>
        <label className="block">Branch<input required list="create-branches" value={branch} onChange={(event) => setBranch(event.target.value)} className="block w-full rounded-md border p-2" /></label>
        <label className="block"><input type="checkbox" checked={asIs} onChange={(event) => setAsIs(event.target.checked)} /> Use branch verbatim (skip Branch Template)</label>
        <label className="block">Base branch<input list="create-branches" value={base} onChange={(event) => setBase(event.target.value)} className="block w-full rounded-md border p-2" /></label>
        <datalist id="create-branches">{options.Branches.map((item) => <option key={item} value={item} />)}</datalist>
        <p>Leave Base branch empty to use the Repo's current branch ({options.CurrentBranch || 'detached HEAD; supply a base explicitly'}). The base is ignored when reusing an existing branch.</p>
        <Button type="submit">Create worktree</Button>
      </fieldset>
    </form>}
    {busy && <p role="status">Creating worktree…</p>}
    {error && <p role="alert" className="whitespace-pre-wrap break-words">{error}</p>}
    {message && <p role="status" className="break-words">{message}</p>}
  </div>
}
