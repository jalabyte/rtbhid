import { useEffect, useState } from 'react'
import { api, type Status } from '@/lib/api'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Server, List, Globe, ShieldOff, Network } from 'lucide-react'

export default function Dashboard() {
  const [status, setStatus] = useState<Status | null>(null)

  useEffect(() => {
    api.getStatus().then(setStatus).catch(console.error)
    const id = setInterval(() => api.getStatus().then(setStatus).catch(console.error), 10000)
    return () => clearInterval(id)
  }, [])

  if (!status) return <p className="text-muted-foreground">Loading...</p>

  const cards = [
    { label: 'Agents', value: status.agents, icon: Server },
    { label: 'Prefixes', value: status.prefixes, icon: List },
    { label: 'Sources', value: status.sources, icon: Globe },
    { label: 'Excludes', value: status.excludes, icon: ShieldOff },
    { label: 'Peers', value: status.peers, icon: Network },
  ]

  return (
    <div className="space-y-6">
      <h2 className="text-2xl font-bold">Dashboard</h2>
      <div className="grid gap-4 grid-cols-2 lg:grid-cols-5">
        {cards.map(({ label, value, icon: Icon }) => (
          <Card key={label}>
            <CardHeader className="flex flex-row items-center justify-between pb-2">
              <CardTitle className="text-sm font-medium">{label}</CardTitle>
              <Icon className="h-4 w-4 text-muted-foreground" />
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold">{value}</div>
            </CardContent>
          </Card>
        ))}
      </div>
    </div>
  )
}
