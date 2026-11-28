import { useEffect, useId, useRef, useState, type ComponentProps, type MouseEvent } from 'react'
import { Button } from '@/components/ui/button'
import { request } from './api'

type DirectoryListing = {
  Path: string
  Parent: string
  Directories: { Name: string; Path: string }[]
  Locations: { Name: string; Path: string }[]
}

type Props = Omit<ComponentProps<'input'>, 'type'> & { label: string }

export default function FolderPicker({ label, id, className, value, defaultValue, onChange, ...props }: Props) {
  const generatedID = useId()
  const inputID = id ?? generatedID
  const dialogID = `${inputID}-folder-picker`
  const inputRef = useRef<HTMLInputElement>(null)
  const dialogRef = useRef<HTMLDialogElement>(null)
  const sequence = useRef(0)
  const [open, setOpen] = useState(false)
  const [path, setPath] = useState('')
  const [listing, setListing] = useState<DirectoryListing>()
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    const dialog = dialogRef.current
    if (open && dialog && !dialog.open) dialog.showModal()
  }, [open])

  useEffect(() => () => { sequence.current += 1 }, [])

  async function load(nextPath: string) {
    const requestID = ++sequence.current
    setPath(nextPath)
    setLoading(true)
    setError('')
    setListing(undefined)
    try {
      const query = nextPath ? `?path=${encodeURIComponent(nextPath)}` : ''
      const result = await request<DirectoryListing>(`/api/folders${query}`)
      if (sequence.current === requestID) { setListing(result); setPath(result.Path) }
    } catch (reason) {
      if (sequence.current === requestID) setError(reason instanceof Error ? reason.message : 'Could not list this folder.')
    } finally {
      if (sequence.current === requestID) setLoading(false)
    }
  }

  function browse(event: MouseEvent<HTMLButtonElement>) {
    event.preventDefault()
    setOpen(true)
    void load(String(inputRef.current?.value ?? ''))
  }

  function close() {
    sequence.current += 1
    dialogRef.current?.close()
    setOpen(false)
  }

  function select() {
    if (!listing || loading || error) return
    const input = inputRef.current
    if (input) {
      const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')?.set
      setter?.call(input, listing.Path)
      input.dispatchEvent(new Event('input', { bubbles: true }))
      input.dispatchEvent(new Event('change', { bubbles: true }))
    }
    close()
  }

  return <div className="space-y-1">
    <label htmlFor={inputID} className="block">{label}</label>
    <div className="flex gap-2">
      <input {...props} id={inputID} ref={inputRef} className={`block min-w-0 flex-1 rounded-md border p-2 focus-visible:outline-2 focus-visible:outline-ring ${className ?? ''}`} value={value} defaultValue={defaultValue} onChange={onChange} />
      <Button type="button" variant="outline" aria-haspopup="dialog" aria-controls={dialogID} onClick={browse}>Browse {label}</Button>
    </div>
    <dialog id={dialogID} ref={dialogRef} aria-labelledby={`${dialogID}-title`} onClose={() => { sequence.current += 1; setOpen(false) }} className="w-[min(36rem,calc(100%-2rem))] rounded-md border p-4">
      {open && <div className="space-y-3">
        <h2 id={`${dialogID}-title`} className="text-lg font-medium">Choose a folder for {label}</h2>
        <div className="flex flex-wrap gap-2" aria-label="Folder locations">
          {!listing && <Button type="button" variant="outline" disabled={loading} onClick={() => void load('')}>Start folder</Button>}
          {listing?.Locations.map((location) => <Button type="button" key={location.Path} variant="outline" disabled={loading} onClick={() => void load(location.Path)}>{location.Name}</Button>)}
        </div>
        <p aria-live="polite" className="break-all">{path || 'Starting folder'}</p>
        {loading && <p role="status">Loading folders…</p>}
        {error && <p role="alert">{error}</p>}
        {listing && <>
          {listing.Parent && <Button type="button" variant="outline" disabled={loading} onClick={() => void load(listing.Parent)}>Parent folder</Button>}
          <ul aria-label="Folders" className="max-h-64 space-y-1 overflow-auto">
            {listing.Directories.map((directory) => <li key={directory.Path}><Button type="button" variant="ghost" className="w-full justify-start" disabled={loading} onClick={() => void load(directory.Path)}>{directory.Name}</Button></li>)}
            {listing.Directories.length === 0 && <li>No subfolders</li>}
          </ul>
        </>}
        <div className="flex justify-end gap-2">
          <Button type="button" variant="outline" onClick={close}>Cancel</Button>
          <Button type="button" disabled={!listing || loading || Boolean(error)} onClick={select}>Select this folder</Button>
        </div>
      </div>}
    </dialog>
  </div>
}
