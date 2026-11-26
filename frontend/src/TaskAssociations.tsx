import { useEffect, useState } from 'react'
import { Button } from '@/components/ui/button'
import { request, type Root } from './api'

export type Mapping = { ID: string; TaskID: string; RepoID: string; Path: string; Identity: string }
type Repo = { ID: string; Name: string; Path: string }
type Review = {
  RootID: string; RepoID: string; TaskID: string; Name: string; TargetID: string
  Path: string; Identity: string; Source: Mapping | null
  SourceTask: { ID: string; RootID: string; TaskID: string; Name: string } | null
  SourceRoot: { ID: string; Name: string; Path: string } | null
}

export default function TaskAssociations({ root, mapping, onChanged }: { root: Root; mapping?: Mapping; onChanged: () => void }) {
  if (mapping) return <MappingActions key={JSON.stringify(mapping)} root={root} mapping={mapping} onChanged={onChanged} />
  return <AttachForm root={root} onChanged={onChanged} />
}

function AttachForm({ root, onChanged }: { root: Root; onChanged: () => void }) {
  const [repos, setRepos] = useState<Repo[]>([])
  const [repoAttempt, setRepoAttempt] = useState(0)
  const [repoLoading, setRepoLoading] = useState(true)
  const [repoID, setRepoID] = useState('')
  const [taskID, setTaskID] = useState('')
  const [name, setName] = useState('')
  const [path, setPath] = useState('')
  const [review, setReview] = useState<Review>()
  const [reassign, setReassign] = useState(false)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [message, setMessage] = useState('')
  const repo = repos.find((item) => item.ID === repoID)
  useEffect(() => {
    let active = true
    request<Repo[]>(`/api/repos?root=${encodeURIComponent(root.ID)}`).then((value) => { if (active) { setRepos(value); setError('') } })
      .catch((reason: unknown) => { if (active) { setRepos([]); setError(reason instanceof Error ? reason.message : 'Cannot load Repos.') } })
      .finally(() => { if (active) setRepoLoading(false) })
    return () => { active = false }
  }, [root.ID, repoAttempt])
  function edit(action: () => void) { action(); setReview(undefined); setReassign(false); setError(''); setMessage('') }
  const needsReassign = Boolean(review?.Source && review.Source.TaskID !== review.TargetID)
  return <section aria-label="Task attachment" className="space-y-3 rounded-md border p-3">
    <h3 className="font-medium">Attach an existing worktree</h3>
    <p>Attaching changes Tasky ownership only. Files, branches, and the Git worktree remain in place.</p>
    <Button type="button" variant="outline" disabled={busy || repoLoading} onClick={() => { setRepoLoading(true); setRepoAttempt((value) => value + 1) }}>Reload Repos</Button>
    {repoLoading && <p role="status">Loading Repos…</p>}
    <fieldset disabled={busy || repoLoading} className="space-y-3">
      <label className="block">Repo<select aria-label="Repo" value={repoID} onChange={(event) => edit(() => setRepoID(event.target.value))} className="block w-full rounded-md border bg-background p-2"><option value="">Select a Repo</option>{repos.map((repo) => <option key={repo.ID} value={repo.ID}>{repo.Name} — {repo.Path} ({repo.ID})</option>)}</select></label>
      <label className="block">Task ID<input value={taskID} onChange={(event) => edit(() => setTaskID(event.target.value))} className="block w-full rounded-md border p-2" /></label>
      <label className="block">Task name (new Tasks only)<input value={name} onChange={(event) => edit(() => setName(event.target.value))} className="block w-full rounded-md border p-2" /></label>
      <label className="block">Absolute checkout path<input value={path} onChange={(event) => edit(() => setPath(event.target.value))} className="block w-full rounded-md border p-2" /></label>
      <Button type="button" disabled={!repoID || !taskID.trim() || !path.trim() || busy} onClick={async () => {
        setBusy(true); setError(''); setMessage(''); setReview(undefined)
        try { setReview(await request<Review>('/api/tasks/attach/preview', 'POST', { RootID: root.ID, RepoID: repoID, TaskID: taskID, Name: name, Path: path })) }
        catch (reason) { setError(reason instanceof Error ? reason.message : 'Review failed. Check the Repo and checkout path.') }
        finally { setBusy(false) }
      }}>Review attachment</Button>
    </fieldset>
    {review && <section aria-label="Attachment review" className="space-y-2 rounded-md border p-3">
      <h4 className="font-medium">Review ownership change</h4>
      <dl><dt>Repo</dt><dd>{repo?.Name} ({review.RepoID}) — {repo?.Path}</dd><dt>Canonical checkout path</dt><dd className="break-words">{review.Path}</dd><dt>Verified identity</dt><dd className="break-words">{review.Identity || 'Not enrolled'}</dd><dt>Target internal Task ID</dt><dd>{review.TargetID || 'New Task'}</dd></dl>
      {review.Source && <div><p>Current owner:</p><p>Root: {review.SourceRoot?.Name} ({review.SourceRoot?.ID}) — {review.SourceRoot?.Path}</p><p>Task: {review.SourceTask?.TaskID} — {review.SourceTask?.Name} (internal ID {review.Source.TaskID})</p><p>Mapping: {review.Source.ID}; Repo {review.Source.RepoID}; path {review.Source.Path}; identity {review.Source.Identity || 'not enrolled'}</p></div>}
      {needsReassign && <label className="block"><input type="checkbox" checked={reassign} onChange={(event) => setReassign(event.target.checked)} /> Reassign this Mapping from its current Task owner</label>}
      <Button type="button" disabled={busy || (needsReassign && !reassign)} onClick={async () => {
        if (busy) return
        setBusy(true); setError(''); setMessage('')
        try { await request('/api/tasks/attach', 'POST', { Review: review, ReassignFrom: needsReassign && reassign ? review.Source?.TaskID ?? '' : '' }); setReview(undefined); setMessage('Worktree attached.'); onChanged() }
        catch (reason) { setError(reason instanceof Error ? reason.message : 'Attach failed. Review the worktree again before retrying.'); setReview(undefined); setReassign(false) }
        finally { setBusy(false) }
      }}>Attach worktree</Button>
    </section>}
    {busy && <p role="status">Working…</p>}{error && <p role="alert" className="whitespace-pre-wrap break-words">{error}</p>}{message && <p role="status">{message}</p>}
  </section>
}

function MappingActions({ root, mapping, onChanged }: { root: Root; mapping: Mapping; onChanged: () => void }) {
  const [path, setPath] = useState('')
  const [acceptIdentity, setAcceptIdentity] = useState(false)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [message, setMessage] = useState('')
  return <div className="space-y-2">
    <Button type="button" variant="outline" disabled={busy} onClick={async () => {
      setBusy(true); setError(''); setMessage('')
      try { await request('/api/tasks/detach', 'POST', { RootID: root.ID, Mapping: mapping }); setMessage('Mapping detached. Files and branches are preserved.'); onChanged() }
      catch (reason) { setError(reason instanceof Error ? reason.message : 'Detach failed.') }
      finally { setBusy(false) }
    }}>Detach worktree</Button>
    <p>Detach removes the Tasky association only. Files, branches, and the Git worktree are preserved.</p>
    <form aria-label="Reconnect worktree" className="space-y-2" onSubmit={async (event) => {
      event.preventDefault(); if (busy || !path.trim() || (mapping.Identity === '' && !acceptIdentity)) return
      setBusy(true); setError(''); setMessage('')
      try { await request('/api/tasks/reconnect', 'POST', { RootID: root.ID, Mapping: mapping, Path: path, AcceptIdentity: mapping.Identity === '' && acceptIdentity }); setMessage('Worktree reconnected.'); onChanged() }
      catch (reason) { setError(reason instanceof Error ? reason.message : 'Reconnect failed.') }
      finally { setBusy(false) }
    }}>
      <label className="block">New checkout path<input value={path} onChange={(event) => { setPath(event.target.value); setError(''); setMessage('') }} className="block w-full rounded-md border p-2" /></label>
      {mapping.Identity === '' && <label className="block"><input type="checkbox" checked={acceptIdentity} onChange={(event) => setAcceptIdentity(event.target.checked)} /> Review and enroll this worktree identity</label>}
      <Button type="submit" disabled={busy || !path.trim() || (mapping.Identity === '' && !acceptIdentity)}>Review and reconnect</Button>
    </form>
    {busy && <p role="status">Working…</p>}{error && <p role="alert" className="whitespace-pre-wrap break-words">{error}</p>}{message && <p role="status">{message}</p>}
  </div>
}
