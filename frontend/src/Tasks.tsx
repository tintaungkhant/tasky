import { useEffect, useState } from 'react'
import { Button } from '@/components/ui/button'
import { request, type Root } from './api'

type Inspection = {
  Task: { ID: string; RootID: string; TaskID: string; Name: string }
  Worktrees: {
    Mapping: { ID: string; Path: string }
    Repo: { ID: string; Name: string; Path: string }
    State: { Status: string; Branch: string; Changes: string; Detail: string }
  }[]
}

export default function Tasks({ root }: { root?: Root }) {
  return root ? <RootTasks key={root.ID} root={root} /> : <p>Choose a Root above to scope Task operations.</p>
}

function RootTasks({ root }: { root: Root }) {
  const [rows, setRows] = useState<Inspection[]>([])
  const [search, setSearch] = useState('')
  const [taskID, setTaskID] = useState('')
  const [attempt, setAttempt] = useState(0)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    let active = true
    request<Inspection[]>(`/api/tasks?root=${encodeURIComponent(root.ID)}`).then((values) => {
      if (!active) return
      setRows(values)
      setTaskID((current) => values.some((row) => row.Task.ID === current) ? current : '')
    }).catch((reason: unknown) => {
      if (active) setError(reason instanceof Error ? reason.message : 'Cannot inspect Tasks. Check that tasky serve is running and retry.')
    }).finally(() => { if (active) setLoading(false) })
    return () => { active = false }
  }, [root.ID, attempt])

  const query = search.toLocaleLowerCase()
  const matches = rows.filter(({ Task }) => `${Task.TaskID}\n${Task.Name}`.toLocaleLowerCase().includes(query))
  const selected = matches.find(({ Task }) => Task.ID === taskID)

  return <section aria-label="Task discovery" className="space-y-4">
    <h2 className="text-lg font-medium">Tasks</h2>
    <Button variant="outline" disabled={loading} onClick={() => {
      setLoading(true)
      setError('')
      setAttempt((value) => value + 1)
    }}>Refresh Tasks</Button>
    <p>Refresh reads current Git state. Use the CLI to create Tasks or change associations.</p>
    {loading ? <p role="status">Loading Tasks and live Git state…</p> : error ? <p role="alert">{error}</p> : <>
      <div className="space-y-1">
        <label htmlFor="task-search" className="block font-medium">Search Tasks by Task ID or name</label>
        <input id="task-search" type="search" value={search} onChange={(event) => setSearch(event.target.value)} className="w-full rounded-md border p-2" />
      </div>
      {rows.length === 0 ? <p role="status">No Tasks in this Root. Use the CLI to create a Task or import legacy data in Registration.</p> : matches.length === 0 ? <p role="status">No matching Tasks.</p> : <>
        <div className="space-y-1">
          <label htmlFor="task-select" className="block font-medium">Task</label>
          <select id="task-select" value={selected?.Task.ID ?? ''} onChange={(event) => setTaskID(event.target.value)} className="w-full rounded-md border bg-background p-2">
            <option value="">Select a Task</option>
            {matches.map(({ Task }) => <option key={Task.ID} value={Task.ID}>{Task.TaskID}{Task.Name ? ` — ${Task.Name}` : ''}</option>)}
          </select>
        </div>
        {selected ? <section aria-label="Task inspection" className="space-y-4 break-words">
          <h3 className="font-medium">{selected.Task.TaskID}{selected.Task.Name ? ` — ${selected.Task.Name}` : ''}</h3>
          {selected.Worktrees.length === 0 && <p>No associated worktrees.</p>}
          {selected.Worktrees.map(({ Mapping, Repo, State }) => <article key={Mapping.ID} className="space-y-2 rounded-md border p-3">
            <h4 className="font-medium">Repo: {Repo.Name}</h4>
            <dl className="space-y-1">
              <dt>Repo path</dt><dd>{Repo.Path}</dd>
              <dt>Worktree path</dt><dd>{Mapping.Path}</dd>
              <dt>Location health</dt><dd>{State.Status}</dd>
              {State.Status === 'ok' && <>
                <dt>Current branch</dt><dd>{State.Branch}</dd>
                <dt>Working tree</dt><dd>{State.Changes ? 'Dirty or untracked changes (Git porcelain status):' : 'Clean'}</dd>
              </>}
            </dl>
            {State.Status === 'ok' && State.Changes && <pre className="overflow-auto whitespace-pre-wrap">{State.Changes}</pre>}
            {State.Detail && <p>{State.Detail}</p>}
          </article>)}
        </section> : <p role="status">Select a Task to inspect its associated worktrees.</p>}
      </>}
    </>}
  </section>
}
