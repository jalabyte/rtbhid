import { useEffect, useState } from 'react'
import { api, type Agent } from '@/lib/api'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Trash2 } from 'lucide-react'

export default function Agents() {
  const [agents, setAgents] = useState<Agent[]>([])

  const load = () => api.getAgents().then(r => setAgents(r?.items ?? [])).catch(() => setAgents([]))

  useEffect(() => { load() }, [])

  const handleDelete = async (id: string) => {
    await api.deleteAgent(id)
    load()
  }

  return (
    <div className="space-y-6">
      <h2 className="text-2xl font-bold">Agents</h2>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Name</TableHead>
            <TableHead>Status</TableHead>
            <TableHead>Bird Version</TableHead>
            <TableHead>Hostname</TableHead>
            <TableHead>Last Seen</TableHead>
            <TableHead className="w-16"></TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {agents.map(a => (
            <TableRow key={a.id}>
              <TableCell className="font-medium">{a.name}</TableCell>
              <TableCell>
                <Badge variant={a.status === 'online' ? 'default' : 'secondary'}>{a.status}</Badge>
              </TableCell>
              <TableCell>{a.bird_version}</TableCell>
              <TableCell>{a.hostname}</TableCell>
              <TableCell>{a.last_seen ? new Date(a.last_seen).toLocaleString() : '—'}</TableCell>
              <TableCell>
                <Button variant="ghost" size="icon" onClick={() => handleDelete(a.id)}>
                  <Trash2 className="h-4 w-4" />
                </Button>
              </TableCell>
            </TableRow>
          ))}
          {agents.length === 0 && (
            <TableRow><TableCell colSpan={6} className="text-center text-muted-foreground">No agents registered</TableCell></TableRow>
          )}
        </TableBody>
      </Table>
    </div>
  )
}
