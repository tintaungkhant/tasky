import { useEffect, useState } from 'react'
import { Button } from '@/components/ui/button'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { connect, listRoots, type Root } from './api'
import Registration from './Registration'
import Tasks from './Tasks'

const sections = ['Tasks', 'Registration', 'Recovery', 'Backup']

export default function App({ connection = connect }: { connection?: () => Promise<void> }) {
  const [roots, setRoots] = useState<Root[]>([])
  const [rootID, setRootID] = useState('')
  const [attempt, setAttempt] = useState(0)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const selected = roots.find((root) => root.ID === rootID)

  useEffect(() => {
    let active = true
    connection().then(listRoots).then((values) => {
      if (!active) return
      setRoots(values)
      setRootID((current) => values.some((root) => root.ID === current) ? current : '')
    }).catch((reason: unknown) => {
      if (active) setError(reason instanceof Error ? reason.message : 'Cannot reach the local service. Check that tasky serve is running.')
    }).finally(() => { if (active) setLoading(false) })
    return () => { active = false }
  }, [connection, attempt])

  return (
    <div className="mx-auto max-w-4xl space-y-6 p-4 sm:p-8">
      <a href="#main" className="sr-only focus:not-sr-only focus:underline">Skip to content</a>
      <header className="space-y-4">
        <h1 className="text-2xl font-semibold">Tasky</h1>
        <div className="flex flex-wrap items-end gap-3">
          <div className="min-w-0 flex-1 space-y-1">
            <label htmlFor="root-select" className="block font-medium">Root</label>
            <select id="root-select" value={rootID} disabled={loading || !!error || roots.length === 0}
              onChange={(event) => setRootID(event.target.value)}
              className="w-full rounded-md border bg-background p-2 focus-visible:outline-2 focus-visible:outline-ring">
              <option value="">Select a Root</option>
              {roots.map((root) => <option key={root.ID} value={root.ID}>{root.Name} — {root.Path} ({root.ID})</option>)}
            </select>
          </div>
          <Button variant="outline" disabled={loading} onClick={() => {
            setLoading(true)
            setError('')
            setAttempt((value) => value + 1)
          }}>Refresh Roots</Button>
        </div>
        {loading && <p role="status">{attempt === 0 ? 'Connecting to the local service…' : 'Loading Roots…'}</p>}
        {error && <p role="alert">{error}</p>}
        {!loading && !error && roots.length === 0 && <p role="status">No registered Roots. Open Registration to configure preferences, register a Root, or import legacy data.</p>}
        {!loading && !error && roots.length > 0 && <p role="status" className="break-words">
          {selected ? `Selected Root: ${selected.Name} — ${selected.Path} (${selected.ID})` : 'Select a Root to continue.'}
        </p>}
      </header>
      <main id="main" tabIndex={-1}>
        <Tabs defaultValue="Tasks">
          <div className="overflow-x-auto pb-1">
            <TabsList aria-label="Dashboard sections">
              {sections.map((section) => <TabsTrigger key={section} value={section}>{section}</TabsTrigger>)}
            </TabsList>
          </div>
          {sections.map((section) => <TabsContent key={section} value={section} className="space-y-2 rounded-md border p-4">
            {section === 'Tasks' ? <Tasks root={selected} /> : section === 'Registration' ? <Registration key={rootID} root={selected} onChanged={async () => {
              const values = await listRoots()
              setRoots(values)
              setRootID((current) => values.some((root) => root.ID === current) ? current : '')
            }} /> : <>
            <h2 className="text-lg font-medium">{section}</h2>
            <p>{section === 'Tasks' && !selected ? 'Choose a Root above to scope Task operations.' : `${section} workflows will be added in the following dashboard tickets. Use the CLI for these operations today.`}</p>
            </>}
          </TabsContent>)}
        </Tabs>
      </main>
    </div>
  )
}
