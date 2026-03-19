import { useEffect, useState } from 'react'
import { api, type Peer } from '@/lib/api'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Trash2, Plus } from 'lucide-react'

export default function Peers() {
  const [peers, setPeers] = useState<Peer[]>([])
  const [form, setForm] = useState({
    name: '', neighbor_ip: '', remote_as: '', local_as: '', multihop: '0',
  })

  const load = () => api.getPeers().then(r => setPeers(r?.items ?? [])).catch(() => setPeers([]))

  useEffect(() => { load() }, [])

  const handleAdd = async () => {
    if (!form.name.trim() || !form.neighbor_ip.trim()) return
    await api.addPeer({
      name: form.name.trim(),
      neighbor_ip: form.neighbor_ip.trim(),
      remote_as: parseInt(form.remote_as, 10) || 0,
      local_as: parseInt(form.local_as, 10) || 0,
      multihop: parseInt(form.multihop, 10) || 0,
      enabled: true,
      std_communities: [],
      large_communities: [],
    })
    setForm({ name: '', neighbor_ip: '', remote_as: '', local_as: '', multihop: '0' })
    load()
  }

  const handleDelete = async (id: string) => {
    await api.deletePeer(id)
    load()
  }

  return (
    <div className="space-y-6">
      <h2 className="text-2xl font-bold">Peers</h2>

      <div className="flex gap-2 flex-wrap">
        <Input placeholder="Name" value={form.name} onChange={e => setForm(f => ({ ...f, name: e.target.value }))} className="max-w-[140px]" />
        <Input placeholder="Neighbor IP" value={form.neighbor_ip} onChange={e => setForm(f => ({ ...f, neighbor_ip: e.target.value }))} className="max-w-[160px]" />
        <Input placeholder="Remote AS" value={form.remote_as} onChange={e => setForm(f => ({ ...f, remote_as: e.target.value }))} className="max-w-[110px]" type="number" />
        <Input placeholder="Local AS" value={form.local_as} onChange={e => setForm(f => ({ ...f, local_as: e.target.value }))} className="max-w-[110px]" type="number" />
        <Input placeholder="Multihop" value={form.multihop} onChange={e => setForm(f => ({ ...f, multihop: e.target.value }))} className="max-w-[100px]" type="number" />
        <Button onClick={handleAdd}><Plus className="h-4 w-4 mr-1" />Add</Button>
      </div>

      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Name</TableHead>
            <TableHead>Neighbor</TableHead>
            <TableHead>Remote AS</TableHead>
            <TableHead>Local AS</TableHead>
            <TableHead>Multihop</TableHead>
            <TableHead>Status</TableHead>
            <TableHead className="w-16"></TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {peers.map(p => (
            <TableRow key={p.id}>
              <TableCell className="font-medium">{p.name}</TableCell>
              <TableCell className="font-mono">{p.neighbor_ip}</TableCell>
              <TableCell>{p.remote_as}</TableCell>
              <TableCell>{p.local_as}</TableCell>
              <TableCell>{p.multihop}</TableCell>
              <TableCell>
                <Badge variant={p.enabled ? 'default' : 'secondary'}>{p.enabled ? 'enabled' : 'disabled'}</Badge>
              </TableCell>
              <TableCell>
                <Button variant="ghost" size="icon" onClick={() => handleDelete(p.id)}>
                  <Trash2 className="h-4 w-4" />
                </Button>
              </TableCell>
            </TableRow>
          ))}
          {peers.length === 0 && (
            <TableRow><TableCell colSpan={7} className="text-center text-muted-foreground">No peers</TableCell></TableRow>
          )}
        </TableBody>
      </Table>
    </div>
  )
}
