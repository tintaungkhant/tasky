import { useEffect, useState, type ComponentProps, type FormEvent } from 'react'
import { Button } from '@/components/ui/button'
import { request, type Root } from './api'
import FolderPicker from './FolderPicker'

type Preferences = { Roots: { Name: string; Path: string; TasksDir: string; BranchTemplate: string }[] }
type Repo = Root & { RootID: string }
type Snapshot = {
  Root: Root; Repos: Repo[]
  Tasks: { ID: string; TaskID: string; Name: string }[]
  Worktrees: { ID: string; TaskID: string; RepoID: string; Path: string; Identity: string }[]
}

function Field({ label, ...props }: ComponentProps<'input'> & { label: string }) {
  return <label className="block space-y-1">{label}<input required className="block w-full rounded-md border p-2 focus-visible:outline-2 focus-visible:outline-ring" {...props} /></label>
}

function RegisterForm({ kind, save }: { kind: 'Root' | 'Repo'; save: (values: { Name: string; Path: string }) => Promise<boolean> }) {
  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const form = event.currentTarget, values = new FormData(form)
    if (await save({ Name: String(values.get('Name')), Path: String(values.get('Path')) })) form.reset()
  }
  return <form aria-label={`Register ${kind}`} onSubmit={submit} className="space-y-3 rounded-md border p-4">
    <h3 className="font-medium">Register {kind}</h3>
    <Field label="Name" name="Name" /><FolderPicker label="Path" name="Path" required defaultValue="" placeholder="Absolute local directory" />
    <Button type="submit">Register {kind}</Button>
  </form>
}

export default function Registration({ root, onChanged }: { root?: Root; onChanged: () => Promise<void> }) {
  const [original, setOriginal] = useState<Preferences>({ Roots: [] })
  const [draft, setDraft] = useState<Preferences>({ Roots: [] })
  const [repos, setRepos] = useState<Repo[]>([])
  const [preview, setPreview] = useState<Snapshot>()
  const [confirmed, setConfirmed] = useState(false)
  const [loading, setLoading] = useState(true)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [message, setMessage] = useState('')
  const [attempt, setAttempt] = useState(0)
  const rootID = root?.ID

  useEffect(() => {
    let active = true
    Promise.all([
      request<Preferences>('/api/preferences'),
      rootID ? request<Repo[]>(`/api/repos?root=${encodeURIComponent(rootID)}`) : Promise.resolve([]),
    ]).then(([cfg, values]) => {
      if (active) { setOriginal(cfg); setDraft(cfg); setRepos(values) }
    }).catch((reason: unknown) => { if (active) setError(reason instanceof Error ? reason.message : 'Could not load registration data.') })
      .finally(() => { if (active) setLoading(false) })
    return () => { active = false }
  }, [rootID, attempt])

  async function run(action: () => Promise<unknown>, success: string) {
    setBusy(true); setError(''); setMessage('')
    try { await action(); setMessage(success); return true }
    catch (reason) { setError(reason instanceof Error ? reason.message : 'Request failed.'); return false }
    finally { setBusy(false) }
  }

  async function refreshMappings() {
    try {
      await onChanged()
      if (rootID) setRepos(await request<Repo[]>(`/api/repos?root=${encodeURIComponent(rootID)}`))
    } catch (reason) { throw new Error(`Operation completed, but refresh failed. Reload registration. ${reason instanceof Error ? reason.message : ''}`) }
  }

  function edit(index: number, key: keyof Preferences['Roots'][number], value: string) {
    setDraft({ Roots: draft.Roots.map((item, i) => i === index ? { ...item, [key]: value } : item) })
  }

  return <section className="space-y-5" aria-label="Registration">
    <h2 className="text-lg font-medium">Registration</h2>
    <p>YAML preferences control Tasks Directory and Branch Template. Registered mappings track identity. Editing one does not add, remove, or relocate the other.</p>
    <Button variant="outline" disabled={busy || loading} onClick={() => {
      setLoading(true); setError(''); setPreview(undefined); setConfirmed(false); setAttempt((value) => value + 1)
    }}>Reload registration</Button>
    {(loading || busy) && <p role="status">{loading ? 'Loading registration…' : 'Saving…'}</p>}
    {error && <p role="alert">{error}</p>}
    {message && <p role="status">{message}</p>}
    <fieldset disabled={loading || busy} className="min-w-0 space-y-5">
      <form aria-label="YAML preferences" className="space-y-3 rounded-md border p-4" onSubmit={(event) => {
        event.preventDefault()
        void run(async () => {
          await request('/api/preferences', 'PUT', { expected: original, config: draft }); setOriginal(draft)
        }, 'YAML preferences saved. Registered mappings and Git data are unchanged.')
      }}>
        <h3 className="font-medium">YAML preferences</h3>
        <p>Saving rewrites YAML formatting and comments. Removing an entry here only removes its YAML preferences.</p>
        {draft.Roots.length === 0 && <p>No YAML Roots configured.</p>}
        {draft.Roots.map((item, index) => <fieldset key={index} className="space-y-2 rounded-md border p-3">
          <legend>YAML Root {index + 1}</legend>
          <Field label="Root name" value={item.Name} onChange={(e) => edit(index, 'Name', e.target.value)} />
          <FolderPicker label="Root path" value={item.Path} onChange={(e) => edit(index, 'Path', e.target.value)} />
          <FolderPicker label="Tasks Directory" value={item.TasksDir} onChange={(e) => edit(index, 'TasksDir', e.target.value)} />
          <Field label="Branch Template" value={item.BranchTemplate} onChange={(e) => edit(index, 'BranchTemplate', e.target.value)} />
          <Button type="button" variant="outline" onClick={() => setDraft({ Roots: draft.Roots.filter((_, i) => i !== index) })}>Remove YAML entry {index + 1}</Button>
        </fieldset>)}
        <div className="flex flex-wrap gap-2">
          <Button type="button" variant="outline" onClick={() => setDraft({ Roots: [...draft.Roots, { Name: '', Path: '', TasksDir: '', BranchTemplate: '{branch}' }] })}>Add YAML Root</Button>
          <Button type="submit">Save YAML preferences</Button>
        </div>
      </form>
      <section className="space-y-3 rounded-md border p-4">
        <h3 className="font-medium">Legacy import</h3>
        <p>Import saved YAML Roots, direct-child Repos, and verified legacy Tasks into mappings. Existing files, branches, worktrees, and editor workspace files are preserved. Repeating import preserves matching identities. Save preference edits first.</p>
        <Button onClick={() => void run(async () => { await request('/api/import', 'POST', {}); await refreshMappings() }, 'Legacy import completed.')}>Import legacy data</Button>
      </section>
      <p>Register existing directories without moving Git data. For YAML Roots with legacy Tasks, import first.</p>
      <RegisterForm kind="Root" save={(values) => run(async () => {
        await request('/api/registration/roots', 'POST', values); await refreshMappings()
      }, 'Root registered. Select its identity above to manage Repos.')} />
      {!root && <p>Select a registered Root above to list or register Repos and preview mapping removal.</p>}
      {root && <>
        <h3 className="break-words font-medium">Registered Root: {root.Name} — {root.Path} ({root.ID})</h3>
        {repos.length === 0 ? <p>No registered Repos in this Root.</p> : <ul className="space-y-2 break-words">{repos.map((repo) => <li key={repo.ID}>{repo.Name} — {repo.Path} ({repo.ID})</li>)}</ul>}
        <RegisterForm kind="Repo" save={(values) => run(async () => {
          await request('/api/registration/repos', 'POST', { ...values, RootID: root.ID }); await refreshMappings()
        }, 'Repo registered. Files and Git data are unchanged.')} />
        <section className="space-y-3 rounded-md border p-4">
          <h3 className="font-medium">Forget Root mappings</h3>
          <p>Forgets this Root and every Repo, Task, and worktree mapping in it. Files, Git worktrees, branches, and YAML entries remain. Forgotten Task identities cannot be recovered by re-registering.</p>
          <Button variant="outline" onClick={() => void run(async () => {
            setPreview(undefined); setConfirmed(false)
            setPreview(await request<Snapshot>(`/api/registration/removal?root=${encodeURIComponent(root.ID)}`))
          }, 'Review every affected mapping before confirming.')}>Preview Root removal</Button>
          {preview && <div className="space-y-3 break-words">
            <p>Root: {preview.Root.Name} — {preview.Root.Path} ({preview.Root.ID})</p>
            <p>{preview.Repos.length} Repo mappings, {preview.Tasks.length} Task mappings, {preview.Worktrees.length} worktree mappings will be forgotten.</p>
            <ul>{preview.Repos.map((repo) => <li key={repo.ID}>Repo: {repo.Name} — {repo.Path} ({repo.ID})</li>)}</ul>
            <ul>{preview.Tasks.map((task) => <li key={task.ID}>Task: {task.TaskID} — {task.Name} ({task.ID})</li>)}</ul>
            <ul>{preview.Worktrees.map((wt) => <li key={wt.ID}>Worktree: {wt.Path} ({wt.ID}); Task {wt.TaskID}, Repo {wt.RepoID}</li>)}</ul>
            <label className="block"><input type="checkbox" checked={confirmed} onChange={(e) => setConfirmed(e.target.checked)} /> I confirm forgetting exactly these mappings, while preserving files and Git data.</label>
            <Button variant="destructive" disabled={!confirmed} onClick={() => void run(async () => {
              await request('/api/registration/remove', 'POST', { expected: preview, confirm: confirmed })
              setPreview(undefined); setConfirmed(false); await onChanged()
            }, 'Root mappings forgotten. Files, Git data, and YAML entries are preserved.')}>Forget these mappings</Button>
          </div>}
        </section>
      </>}
    </fieldset>
  </section>
}
