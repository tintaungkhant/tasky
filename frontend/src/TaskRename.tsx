import { useState } from 'react'
import { Button } from '@/components/ui/button'
import { request, type Root } from './api'

export default function TaskRename({ root, task, onRenamed }: {
  root: Root; task: { ID: string; TaskID: string; Name: string }; onRenamed: () => void
}) {
  const [name, setName] = useState(task.Name)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [message, setMessage] = useState('')
  return <form aria-label="Rename Task" className="space-y-2" onSubmit={async (event) => {
    event.preventDefault()
    if (busy) return
    setBusy(true); setError(''); setMessage('')
    try {
      await request('/api/tasks/rename', 'POST', { RootID: root.ID, ID: task.ID, Name: name })
      setMessage('Task renamed. Identity, branches, paths, and associations are preserved.')
      onRenamed()
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : 'Rename failed. Refresh Tasks before retrying.')
    } finally { setBusy(false) }
  }}>
    <label className="block">Task name<input value={name} disabled={busy} onChange={(event) => setName(event.target.value)} className="block w-full rounded-md border p-2" /></label>
    <p>An empty name removes the optional label. Task ID stays {task.TaskID}.</p>
    <Button type="submit" disabled={busy}>Rename Task</Button>
    {busy && <p role="status">Renaming Task…</p>}
    {error && <p role="alert">{error}</p>}
    {message && <p role="status">{message}</p>}
  </form>
}
