import { useEffect, useState } from "react";
import {
  Activity,
  Ban,
  BookOpen,
  CloudOff,
  Globe,
  LayoutDashboard,
  List,
  ListPlus,
  LogOut,
  Menu,
  Monitor,
  Moon,
  Network,
  ScrollText,
  Search,
  Server,
  ShieldCheck,
  Sun,
  X,
} from "lucide-react";
import { NavLink } from "react-router";
import { api } from "@/lib/api";
import { NullwellMark } from "@/components/app";
import { getTheme, setTheme, applyTheme, type Theme } from "@/lib/theme";
import { getTextSize, setTextSize, type TextSize } from "@/lib/text-size";
import { cn } from "@/lib/utils";

const navGroups = [
  {
    label: "Monitor",
    items: [
      { to: "/", icon: LayoutDashboard, label: "Dashboard" },
      { to: "/bgp", icon: Activity, label: "BGP Monitor" },
      { to: "/agents", icon: Server, label: "Agents" },
      { to: "/events", icon: ScrollText, label: "Events" },
    ],
  },
  {
    label: "Blocklist",
    items: [
      { to: "/prefixes", icon: List, label: "Prefixes" },
      { to: "/prefix-builder", icon: ListPlus, label: "Prefix Builder" },
      { to: "/sources", icon: Globe, label: "Sources" },
      { to: "/lookup", icon: Search, label: "Lookup" },
    ],
  },
  {
    label: "Exceptions",
    items: [
      { to: "/excludes", icon: Ban, label: "Excludes" },
      { to: "/exclude-prefixes", icon: CloudOff, label: "Remote Excludes" },
    ],
  },
  {
    label: "Peering",
    items: [
      { to: "/peers", icon: Network, label: "Peers" },
      { to: "/whitelist", icon: ShieldCheck, label: "ASN Whitelist" },
    ],
  },
  {
    label: "Help",
    items: [{ to: "/docs", icon: BookOpen, label: "Docs" }],
  },
];

const themes: { value: Theme; label: string; icon: typeof Sun }[] = [
  { value: "system", label: "System", icon: Monitor },
  { value: "light", label: "Light", icon: Sun },
  { value: "dark", label: "Dark", icon: Moon },
];

function ThemeSwitch() {
  const [theme, setThemeState] = useState<Theme>(getTheme);

  useEffect(() => {
    if (theme !== "system") return;
    const mql = window.matchMedia("(prefers-color-scheme: dark)");
    const onChange = () => applyTheme("system");
    mql.addEventListener("change", onChange);
    return () => mql.removeEventListener("change", onChange);
  }, [theme]);

  return (
    <div role="group" aria-label="Theme" className="flex rounded-md border p-0.5">
      {themes.map(({ value, label, icon: Icon }) => (
        <button
          key={value}
          type="button"
          aria-pressed={theme === value}
          title={`${label} theme`}
          onClick={() => {
            setTheme(value);
            setThemeState(value);
          }}
          className={cn(
            "flex flex-1 items-center justify-center gap-1 rounded-sm py-1 text-xs pointer-coarse:min-h-11",
            theme === value
              ? "bg-muted font-medium text-foreground"
              : "text-muted-foreground hover:text-foreground",
          )}
        >
          <Icon className="size-3.5" aria-hidden="true" />
          {label}
        </button>
      ))}
    </div>
  );
}

function TextSizeSwitch() {
  const [size, setSize] = useState<TextSize>(getTextSize);
  return (
    <div role="group" aria-label="Text size" className="flex rounded-md border p-0.5">
      {(["normal", "large"] as const).map((v) => (
        <button
          key={v}
          type="button"
          aria-pressed={size === v}
          onClick={() => {
            setTextSize(v);
            setSize(v);
          }}
          className={cn(
            "flex flex-1 items-center justify-center gap-1 rounded-sm py-1 text-xs pointer-coarse:min-h-11",
            size === v ? "bg-muted font-medium text-foreground" : "text-muted-foreground hover:text-foreground",
          )}
        >
          <span aria-hidden="true" className={v === "large" ? "text-sm font-semibold" : ""}>
            A
          </span>
          {v === "normal" ? "Normal text" : "Large text"}
        </button>
      ))}
    </div>
  );
}

export function Sidebar({ authEnabled, username }: { authEnabled: boolean; username: string }) {
  const [open, setOpen] = useState(false);

  const handleLogout = async () => {
    await api.logout().catch(() => {});
    window.location.href = "/login";
  };

  return (
    <aside className="shrink-0 border-b bg-card md:flex md:w-60 md:flex-col md:border-r md:border-b-0">
      <div className="flex h-12 items-center justify-between px-4 md:h-14">
        <div className="flex items-center gap-2.5 leading-tight">
          <NullwellMark className="size-7 shrink-0" />
          <div>
            <div className="font-semibold">Nullwell</div>
            <div className="text-xs text-muted-foreground">blackhole controller</div>
          </div>
        </div>
        <button
          type="button"
          className="-mr-2 rounded-md p-2 md:hidden pointer-coarse:min-h-11 pointer-coarse:min-w-11"
          aria-expanded={open}
          aria-controls="main-nav"
          aria-label={open ? "Close menu" : "Open menu"}
          onClick={() => setOpen((o) => !o)}
        >
          {open ? <X className="size-5" aria-hidden="true" /> : <Menu className="size-5" aria-hidden="true" />}
        </button>
      </div>

      <div
        id="main-nav"
        className={cn(
          "flex-1 flex-col overflow-y-auto px-2 pb-3 md:flex",
          open ? "flex" : "hidden",
        )}
      >
        <nav aria-label="Main" className="flex-1 space-y-3">
          {navGroups.map((group) => (
            <div key={group.label}>
              <div className="px-2 pb-1 text-xs text-muted-foreground">{group.label}</div>
              {group.items.map(({ to, icon: Icon, label }) => (
                <NavLink
                  key={to}
                  to={to}
                  end={to === "/"}
                  onClick={() => setOpen(false)}
                  className={({ isActive }) =>
                    cn(
                      "flex items-center gap-2.5 rounded-md px-2 py-1.5 pointer-coarse:min-h-11",
                      isActive
                        ? "bg-muted font-medium text-foreground"
                        : "text-muted-foreground hover:bg-muted/60 hover:text-foreground",
                    )
                  }
                >
                  {({ isActive }) => (
                    <>
                      <Icon
                        aria-hidden="true"
                        className={cn("size-4", isActive && "text-signal")}
                      />
                      {label}
                    </>
                  )}
                </NavLink>
              ))}
            </div>
          ))}
        </nav>

        <div className="mt-3 space-y-2 border-t px-1 pt-3">
          <ThemeSwitch />
          <TextSizeSwitch />
          {authEnabled && (
            <div className="flex items-center justify-between gap-2">
              <span className="truncate text-xs text-muted-foreground">{username}</span>
              <button
                type="button"
                onClick={handleLogout}
                className="flex items-center gap-1.5 rounded-md px-2 py-1 text-xs text-muted-foreground hover:bg-muted hover:text-foreground pointer-coarse:min-h-11"
              >
                <LogOut className="size-3.5" aria-hidden="true" />
                Log out
              </button>
            </div>
          )}
        </div>
      </div>
    </aside>
  );
}
