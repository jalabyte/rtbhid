import { LayoutDashboard, Server, List, Globe, ShieldOff, Network, ScrollText } from 'lucide-react'
import { NavLink } from 'react-router'

const navItems = [
  { to: '/', icon: LayoutDashboard, label: 'Dashboard' },
  { to: '/agents', icon: Server, label: 'Agents' },
  { to: '/prefixes', icon: List, label: 'Prefixes' },
  { to: '/sources', icon: Globe, label: 'Sources' },
  { to: '/excludes', icon: ShieldOff, label: 'Excludes' },
  { to: '/peers', icon: Network, label: 'Peers' },
  { to: '/events', icon: ScrollText, label: 'Events' },
]

export function Sidebar() {
  return (
    <aside className="w-64 border-r bg-muted/40 p-4 shrink-0">
      <h1 className="text-lg font-bold mb-6">RTBH Portal</h1>
      <nav className="space-y-1">
        {navItems.map(({ to, icon: Icon, label }) => (
          <NavLink
            key={to}
            to={to}
            end={to === '/'}
            className={({ isActive }: { isActive: boolean }) =>
              `flex items-center gap-3 px-3 py-2 rounded-md text-sm ${
                isActive ? 'bg-primary text-primary-foreground' : 'hover:bg-muted'
              }`
            }
          >
            <Icon className="h-4 w-4" />
            {label}
          </NavLink>
        ))}
      </nav>
    </aside>
  )
}
