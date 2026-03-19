import { useEffect, useState } from 'react'
import { api, type Event } from '@/lib/api'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Badge } from '@/components/ui/badge'

export default function Events() {
  const [events, setEvents] = useState<Event[]>([])

  useEffect(() => {
    api.getEvents().then(r => setEvents(r?.items ?? [])).catch(() => setEvents([]))
  }, [])

  return (
    <div className="space-y-6">
      <h2 className="text-2xl font-bold">Events</h2>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead className="w-48">Timestamp</TableHead>
            <TableHead className="w-32">Type</TableHead>
            <TableHead>Message</TableHead>
            <TableHead className="w-32">Agent</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {events.map(e => (
            <TableRow key={e.id}>
              <TableCell className="text-sm text-muted-foreground">{new Date(e.created_at).toLocaleString()}</TableCell>
              <TableCell>
                <Badge variant={e.type === 'error' ? 'destructive' : 'secondary'}>{e.type}</Badge>
              </TableCell>
              <TableCell>{e.message}</TableCell>
              <TableCell className="text-sm">{e.agent_id || '—'}</TableCell>
            </TableRow>
          ))}
          {events.length === 0 && (
            <TableRow><TableCell colSpan={4} className="text-center text-muted-foreground">No events</TableCell></TableRow>
          )}
        </TableBody>
      </Table>
    </div>
  )
}
