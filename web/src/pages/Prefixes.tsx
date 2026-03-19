import { useEffect, useState } from 'react'
import { api, type Prefix } from '@/lib/api'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Trash2, Plus } from 'lucide-react'

export default function Prefixes() {
  const [prefixes, setPrefixes] = useState<Prefix[]>([])
  const [cidr, setCidr] = useState('')
  const [note, setNote] = useState('')

  const load = () => api.getPrefixes().then(r => setPrefixes(r.items)).catch(console.error)

  useEffect(() => { load() }, [])

  const handleAdd = async () => {
    if (!cidr.trim()) return
    await api.addPrefix({ cidr: cidr.trim(), note: note.trim() })
    setCidr('')
    setNote('')
    load()
  }

  const handleDelete = async (id: number) => {
    await api.deletePrefix(id)
    load()
  }

  const handleToggle = async (p: Prefix) => {
    await api.togglePrefix(p.id, !p.enabled)
    load()
  }

  return (
    <div className="space-y-6">
      <h2 className="text-2xl font-bold">Prefixes</h2>

      <div className="flex gap-2">
        <Input placeholder="CIDR (e.g. 192.0.2.0/24)" value={cidr} onChange={e => setCidr(e.target.value)} className="max-w-xs" />
        <Input placeholder="Note (optional)" value={note} onChange={e => setNote(e.target.value)} className="max-w-xs" />
        <Button onClick={handleAdd}><Plus className="h-4 w-4 mr-1" />Add</Button>
      </div>

      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>CIDR</TableHead>
            <TableHead>Status</TableHead>
            <TableHead>Note</TableHead>
            <TableHead>Created</TableHead>
            <TableHead className="w-24"></TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {prefixes.map(p => (
            <TableRow key={p.id}>
              <TableCell className="font-mono">{p.cidr}</TableCell>
              <TableCell>
                <Badge variant={p.enabled ? 'default' : 'secondary'} className="cursor-pointer" onClick={() => handleToggle(p)}>
                  {p.enabled ? 'enabled' : 'disabled'}
                </Badge>
              </TableCell>
              <TableCell>{p.note || '—'}</TableCell>
              <TableCell>{new Date(p.created_at).toLocaleString()}</TableCell>
              <TableCell>
                <Button variant="ghost" size="icon" onClick={() => handleDelete(p.id)}>
                  <Trash2 className="h-4 w-4" />
                </Button>
              </TableCell>
            </TableRow>
          ))}
          {prefixes.length === 0 && (
            <TableRow><TableCell colSpan={5} className="text-center text-muted-foreground">No prefixes</TableCell></TableRow>
          )}
        </TableBody>
      </Table>
    </div>
  )
}
