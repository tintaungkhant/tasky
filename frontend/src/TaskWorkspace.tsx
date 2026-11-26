import { useState } from 'react'
import { Button } from '@/components/ui/button'
import { request, type Root } from './api'

type Task = { ID: string; RootID: string; TaskID: string; Name: string }
type WorkspaceResponse = { Path: string; Current: boolean }

export default function TaskWorkspace({ root, task }: { root: Root; task: Task }) {
  const [path, setPath] = useState('')
  const [current, setCurrent] = useState(false)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  async function generate() {
    setBusy(true)
    setError('')
    setPath('')
    setCurrent(false)
    try {
      const result = await request<WorkspaceResponse>('/api/tasks/workspace', 'POST', { RootID: root.ID, TaskID: task.ID })
      setPath(result.Path)
      setCurrent(result.Current)
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : String(reason))
    } finally {
      setBusy(false)
    }
  }

  return <section aria-label="Task workspace file" className="space-y-2 rounded-md border p-3">
    <h4 className="font-medium">Workspace file</h4>
    <p>Generate a workspace file for this Task. Open the file in a compatible editor to use it.</p>
    <p>Identical files are retained, and differing user files are preserved.</p>
    <p>Move the previous file aside before regenerating it.</p>
    <Button variant="outline" disabled={busy} onClick={generate}>{busy ? 'Generating workspace…' : 'Generate editor workspace'}</Button>
    {busy && <p role="status">Generating editor workspace…</p>}
    {error && <p role="alert">{error}</p>}
    {path && <>
      <p role="status">{current ? 'The workspace file is already current.' : 'Generated a new workspace file.'}</p>
      <div className="space-y-1">
        <label htmlFor="workspace-file-path" className="block font-medium">Workspace file path</label>
        <input id="workspace-file-path" type="text" readOnly value={path} onFocus={(event) => event.currentTarget.select()} className="w-full rounded-md border p-2" />
        <p>Select the path above to copy it manually.</p>
      </div>
    </>}
  </section>
}
