import { useEffect, useState } from "react";
import {
  LayoutDashboard,
  Server,
  List,
  Globe,
  ShieldOff,
  Network,
  ScrollText,
  LogOut,
  BookOpen,
  Search,
} from "lucide-react";
import { NavLink } from "react-router";
import { api } from "@/lib/api";
import { Button } from "@/components/ui/button";

const navItems = [
  { to: "/", icon: LayoutDashboard, label: "Dashboard" },
  { to: "/agents", icon: Server, label: "Agents" },
  { to: "/prefixes", icon: List, label: "Prefixes" },
  { to: "/sources", icon: Globe, label: "Sources" },
  { to: "/excludes", icon: ShieldOff, label: "Excludes" },
  { to: "/peers", icon: Network, label: "Peers" },
  { to: "/events", icon: ScrollText, label: "Events" },
  { to: "/lookup", icon: Search, label: "Lookup" },
  { to: "/docs", icon: BookOpen, label: "Docs", external: true },
];

export function Sidebar() {
  const [authEnabled, setAuthEnabled] = useState(false);
  const [username, setUsername] = useState("");

  useEffect(() => {
    api
      .getMe()
      .then((r) => {
        setAuthEnabled(r.enabled);
        setUsername(r.username ?? "");
      })
      .catch(() => {});
  }, []);

  const handleLogout = async () => {
    await api.logout().catch(() => {});
    window.location.href = "/login";
  };

  return (
    <aside className="w-64 border-r bg-muted/40 p-4 shrink-0 flex flex-col">
      <h1 className="text-lg font-bold mb-6">RTBH Portal</h1>
      <nav className="space-y-1 flex-1">
        {navItems.map(({ to, icon: Icon, label }) => (
          <NavLink
            key={to}
            to={to}
            end={to === "/"}
            className={({ isActive }: { isActive: boolean }) =>
              `flex items-center gap-3 px-3 py-2 rounded-md text-sm ${
                isActive
                  ? "bg-primary text-primary-foreground"
                  : "hover:bg-muted"
              }`
            }
          >
            <Icon className="h-4 w-4" />
            {label}
          </NavLink>
        ))}
      </nav>
      {authEnabled && (
        <div className="border-t pt-3 mt-3">
          <div className="text-xs text-muted-foreground mb-2 px-3">
            {username}
          </div>
          <Button
            variant="ghost"
            size="sm"
            className="w-full justify-start"
            onClick={handleLogout}
          >
            <LogOut className="h-4 w-4 mr-2" />
            Logout
          </Button>
        </div>
      )}
    </aside>
  );
}
