import { useEffect, useState } from 'react'
import { api, type Exclude } from '@/lib/api'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Trash2, Plus } from 'lucide-react'

export default function Excludes() {
  const [excludes, setExcludes] = useState<Exclude[]>([])
  const [cidr, setCidr] = useState('')
  const [note, setNote] = useState('')

  const load = () => api.getExcludes().then(r => setExcludes(r.items)).catch(console.error)

  useEffect(() => { load() }, [])

  const handleAdd = async () => {
    if (!cidr.trim()) return
    await api.addExclude({ cidr: cidr.trim(), note: note.trim() })
    setCidr('')
    setNote('')
    load()
  }

  const handleDelete = async (id: number) => {
    await api.deleteExclude(id)
    load()
  }

  return (
    <div className="space-y-6">
      <h2 className="text-2xl font-bold">Excludes</h2>

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
            <TableRow><TableCell colSpan={4} className="text-center text-muted-foreground">No excludes</TableCell></TableRow>
          )}
        </TableBody>
      </Table>
    </div>
  )
}
