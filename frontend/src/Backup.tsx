import { useState, type FormEvent } from 'react'
import { Button } from '@/components/ui/button'
import { downloadMappingBackup, restoreMappingBackup } from './api'

export default function Backup({ onRestored }: { onRestored: () => Promise<void> }) {
  const [file, setFile] = useState<File>()
  const [busy, setBusy] = useState<'download' | 'restore'>()
  const [error, setError] = useState('')
  const [report, setReport] = useState<Awaited<ReturnType<typeof restoreMappingBackup>>>()

  async function download() {
    setBusy('download')
    setError('')
    try {
      await downloadMappingBackup()
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : 'Could not download the mapping backup.')
    } finally {
      setBusy(undefined)
    }
  }

  async function restore(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!file || busy) return
    const form = event.currentTarget
    setBusy('restore')
    setError('')
    setReport(undefined)
    try {
      setReport(await restoreMappingBackup(file))
      form.reset()
      setFile(undefined)
      try {
        await onRestored()
      } catch {
        setError('Mappings were restored, but the Root list could not refresh. Use Refresh Roots to retry.')
      }
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : 'Could not restore the mapping snapshot.')
    } finally {
      setBusy(undefined)
    }
  }

  return <section className="space-y-4">
    <h2 className="text-lg font-medium">Mapping backup and restore</h2>
    <p>Backups include mappings for all Roots. They contain no code, branches, or YAML preferences.</p>
    <Button type="button" disabled={!!busy} onClick={download}>Download mapping backup</Button>
    <form onSubmit={restore} className="space-y-2 border-t pt-4">
      <p id="restore-help">Choose a Tasky JSON snapshot up to 64 MiB. Restore merges its mappings and stops if any existing record conflicts.</p>
      <label htmlFor="mapping-snapshot" className="block font-medium">Mapping snapshot</label>
      <input id="mapping-snapshot" type="file" accept=".json,application/json" aria-describedby="restore-help"
        aria-required="true" disabled={!!busy} onChange={(event) => setFile(event.target.files?.[0])} className="block max-w-full" />
      <Button type="submit" disabled={!file || !!busy}>Restore mappings</Button>
    </form>
    {busy && <p role="status">{busy === 'download' ? 'Preparing mapping backup…' : 'Restoring mappings…'}</p>}
    {error && <p role="alert">{error}</p>}
    {report && <div className="space-y-2">
      <p role="status">Mappings restored. Existing mappings were preserved.</p>
      {!!report.Warnings?.length && <>
        <h3 className="font-medium">Locations to review</h3>
        <p>Review these mappings in Registration or Tasks before using them.</p>
        <ul className="list-disc space-y-1 pl-5">
          {report.Warnings.map((warning) => <li key={`${warning.Kind}-${warning.ID}`} className="break-words">
            {warning.Kind} {warning.ID} at {warning.Path}{warning.Status && ` [${warning.Status}]`}: {warning.Detail || 'Check whether this location is accessible.'}
          </li>)}
        </ul>
      </>}
    </div>}
  </section>
}
