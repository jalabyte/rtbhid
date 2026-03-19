import { useEffect, useState } from 'react'
import { api, type Exclude, type Source } from '@/lib/api'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Trash2, Plus, RefreshCw } from 'lucide-react'

export default function Excludes() {
  const [excludes, setExcludes] = useState<Exclude[]>([])
  const [cidr, setCidr] = useState('')
  const [note, setNote] = useState('')

  const [sources, setSources] = useState<Source[]>([])
  const [srcForm, setSrcForm] = useState({ name: '', url: '', interval: '300' })

  const loadExcludes = () => api.getExcludes().then(r => setExcludes(r?.items ?? [])).catch(() => setExcludes([]))
  const loadSources = () => api.getExcludeSources().then(r => setSources(r?.items ?? [])).catch(() => setSources([]))
  const load = () => { loadExcludes(); loadSources() }

  useEffect(() => { load() }, [])

  const handleAdd = async () => {
    if (!cidr.trim()) return
    await api.addExclude({ cidr: cidr.trim(), note: note.trim() })
    setCidr('')
    setNote('')
    loadExcludes()
  }

  const handleDelete = async (id: number) => {
    await api.deleteExclude(id)
    loadExcludes()
  }

  const handleAddSource = async () => {
    if (!srcForm.url.trim()) return
    await api.addExcludeSource({
      name: srcForm.name.trim(),
      url: srcForm.url.trim(),
      interval: parseInt(srcForm.interval, 10) || 300,
    })
    setSrcForm({ name: '', url: '', interval: '300' })
    loadSources()
  }

  const handleDeleteSource = async (id: string) => {
    await api.deleteExcludeSource(id)
    loadSources()
  }

  const handleSyncSource = async (id: string) => {
    await api.syncExcludeSource(id)
    loadSources()
  }

  return (
    <div className="space-y-8">
      <h2 className="text-2xl font-bold">Excludes</h2>

      {/* Manual excludes */}
      <section className="space-y-4">
        <h3 className="text-lg font-semibold">Manual Excludes</h3>
        <div className="flex gap-2">
          <Input placeholder="CIDR (e.g. 10.0.0.0/8)" value={cidr} onChange={e => setCidr(e.target.value)} className="max-w-xs" />
          <Input placeholder="Note (optional)" value={note} onChange={e => setNote(e.target.value)} className="max-w-xs" />
          <Button onClick={handleAdd}><Plus className="h-4 w-4 mr-1" />Add</Button>
        </div>

        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>CIDR</TableHead>
              <TableHead>Note</TableHead>
              <TableHead>Created</TableHead>
              <TableHead className="w-16"></TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {excludes.map(e => (
              <TableRow key={e.id}>
                <TableCell className="font-mono">{e.cidr}</TableCell>
                <TableCell>{e.note || '—'}</TableCell>
                <TableCell>{new Date(e.created_at).toLocaleString()}</TableCell>
                <TableCell>
                  <Button variant="ghost" size="icon" onClick={() => handleDelete(e.id)}>
                    <Trash2 className="h-4 w-4" />
                  </Button>
                </TableCell>
              </TableRow>
            ))}
            {excludes.length === 0 && (
              <TableRow><TableCell colSpan={4} className="text-center text-muted-foreground">No manual excludes</TableCell></TableRow>
            )}
          </TableBody>
        </Table>
      </section>

      {/* Remote exclude sources */}
      <section className="space-y-4">
        <h3 className="text-lg font-semibold">Remote Exclude Sources</h3>
        <div className="flex gap-2 flex-wrap">
          <Input placeholder="Name" value={srcForm.name} onChange={e => setSrcForm(f => ({ ...f, name: e.target.value }))} className="max-w-[160px]" />
          <Input placeholder="URL" value={srcForm.url} onChange={e => setSrcForm(f => ({ ...f, url: e.target.value }))} className="max-w-xs" />
          <Input placeholder="Interval (s)" value={srcForm.interval} onChange={e => setSrcForm(f => ({ ...f, interval: e.target.value }))} className="max-w-[120px]" />
          <Button onClick={handleAddSource}><Plus className="h-4 w-4 mr-1" />Add</Button>
        </div>

        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Name</TableHead>
              <TableHead>URL</TableHead>
              <TableHead>Interval</TableHead>
              <TableHead>Last Sync</TableHead>
              <TableHead>Count</TableHead>
              <TableHead className="w-24"></TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {sources.map(s => (
              <TableRow key={s.id}>
                <TableCell className="font-medium">{s.name || s.id}</TableCell>
                <TableCell className="max-w-[200px] truncate">{s.url}</TableCell>
                <TableCell>{s.interval}s</TableCell>
                <TableCell>{s.last_sync ? new Date(s.last_sync).toLocaleString() : '—'}</TableCell>
                <TableCell>{s.last_count}</TableCell>
                <TableCell className="flex gap-1">
                  <Button variant="ghost" size="icon" onClick={() => handleSyncSource(s.id)}>
                    <RefreshCw className="h-4 w-4" />
                  </Button>
                  <Button variant="ghost" size="icon" onClick={() => handleDeleteSource(s.id)}>
                    <Trash2 className="h-4 w-4" />
                  </Button>
                </TableCell>
              </TableRow>
            ))}
            {sources.length === 0 && (
              <TableRow><TableCell colSpan={6} className="text-center text-muted-foreground">No remote exclude sources</TableCell></TableRow>
            )}
          </TableBody>
        </Table>
      </section>
    </div>
  )
}
