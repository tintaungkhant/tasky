import { useState } from 'react'
import { Button } from '@/components/ui/button'
import { request, type Root } from './api'
import type { Mapping } from './TaskAssociations'

type Options = { DeleteBranch: boolean; ForceWorktree: boolean; ForceBranch: boolean }
type Target = { Mapping: Mapping; Branch: string; Revision: string; RepoIdentity: string }
type Review = { RootID: string; Targets: Target[]; Options: Options }
type Result = { Completed: Mapping[]; Failed: Mapping | null; Error: string; Remaining: Mapping[] }

export default function TaskCleanup({ root, mappings, onChanged }: { root: Root; mappings: Mapping[]; onChanged: () => void }) {
  const [selected, setSelected] = useState<string[]>([])
  const [options, setOptions] = useState<Options>({ DeleteBranch: false, ForceWorktree: false, ForceBranch: false })
  const [review, setReview] = useState<Review | null>(null)
  const [consent, setConsent] = useState(false)
  const [result, setResult] = useState<Result | null>(null)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [expanded, setExpanded] = useState(false)
  const invalidate = (next: string[], nextOptions = options) => { setSelected(next); setOptions(nextOptions); setReview(null); setConsent(false); setResult(null); setError('') }
  const toggle = (key: keyof Options) => invalidate(selected, { ...options, [key]: !options[key] })
  const preview = async () => {
    setBusy(true); setError(''); setReview(null); setConsent(false); setResult(null)
    try { setReview(await request<Review>('/api/tasks/cleanup/preview', 'POST', { RootID: root.ID, Mappings: mappings.filter((mapping) => selected.includes(mapping.ID)), Options: options })); setConsent(false) }
    catch (reason) { setError(reason instanceof Error ? reason.message : 'Could not preview cleanup.') }
    finally { setBusy(false) }
  }
  const submit = async () => {
    if (!review || !consent) return
    const submittedReview = review
    setBusy(true); setError('')
    try { setResult(await request<Result>('/api/tasks/cleanup', 'POST', { Review: submittedReview, Confirm: true })) }
    catch (reason) { setError(reason instanceof Error ? reason.message : 'Cleanup failed. Inspect pending operations before retrying.') }
    finally { setReview(null); setConsent(false); setBusy(false) }
  }
  return <section className="space-y-3" aria-label="Task cleanup">
    <Button variant="outline" aria-expanded={expanded} disabled={busy} onClick={() => setExpanded((value) => !value)}>Remove worktrees</Button>
    {expanded && <div className="space-y-3" aria-busy={busy}>
    <p>Select the exact worktrees to remove. Their mappings are removed when cleanup completes.</p>
    <fieldset disabled={busy} className="space-y-2"><legend className="font-medium">Worktrees</legend>
      {mappings.map((mapping) => <label key={mapping.ID} className="flex gap-2 break-all"><input type="checkbox" checked={selected.includes(mapping.ID)} onChange={() => invalidate(selected.includes(mapping.ID) ? selected.filter((id) => id !== mapping.ID) : [...selected, mapping.ID])} />{mapping.Path} <span className="text-muted-foreground">({mapping.ID})</span></label>)}
    </fieldset>
    <fieldset disabled={busy} className="space-y-2"><legend className="font-medium">Cleanup options</legend>
      <label className="flex gap-2"><input type="checkbox" checked={options.DeleteBranch} onChange={() => toggle('DeleteBranch')} />Delete each selected worktree’s branch</label>
      <label className="flex gap-2"><input type="checkbox" checked={options.ForceWorktree} onChange={() => toggle('ForceWorktree')} />Force removal of dirty worktrees</label>
      <label className="flex gap-2"><input type="checkbox" checked={options.ForceBranch} onChange={() => toggle('ForceBranch')} />Force deletion of unmerged branches</label>
      <p>Forcing worktree removal discards uncommitted and untracked files inside that checkout. Identity checks still apply. Files outside the selected checkout are retained.</p>
    </fieldset>
    <Button variant="outline" disabled={busy || selected.length === 0} onClick={preview}>Review cleanup</Button>
    {busy && <p role="status">Working on cleanup…</p>}
    {error && <p role="alert">{error}</p>}
    {review && <div className="space-y-2 rounded border p-3" aria-label="Cleanup review">
      <h5 className="font-medium">Review these exact effects</h5>
      {review.Targets.map(({ Mapping: mapping, Branch, Revision, RepoIdentity }) => <article key={mapping.ID} className="break-all">
        <p>{mapping.Path} <span>({mapping.ID})</span></p><p>Branch: {Branch || 'detached HEAD'}; revision: {Revision}; Repo identity: {RepoIdentity}</p>
        <p>Worktree: remove checkout and mapping. Branch: {review.Options.DeleteBranch && Branch ? 'delete' : 'preserve'}.</p>
      </article>)}
      <p>Recorded overrides: delete branch {String(review.Options.DeleteBranch)}, force dirty worktree {String(review.Options.ForceWorktree)}, force unmerged branch {String(review.Options.ForceBranch)}. Identity checks are never bypassed.</p>
      <label className="flex gap-2"><input type="checkbox" checked={consent} onChange={(event) => setConsent(event.target.checked)} />I understand and consent to these cleanup effects</label>
      <Button disabled={busy || !consent} onClick={submit}>Clean up selected</Button>
    </div>}
    {result && <div className="space-y-2" role="status"><h5 className="font-medium">Cleanup result</h5>
      <p>Completed: {result.Completed.length ? result.Completed.map((item) => item.Path).join(', ') : 'none'}</p>
      <p>Failed: {result.Failed?.Path ?? 'none'}</p><p>Remaining: {result.Remaining.length ? result.Remaining.map((item) => item.Path).join(', ') : 'none'}</p>
      {result.Error && <p role="alert">{result.Error}</p>}
      <p>Refresh Task discovery to see the current mappings.</p><Button variant="outline" onClick={onChanged}>Refresh Tasks</Button>
    </div>}
    </div>}
  </section>
}
