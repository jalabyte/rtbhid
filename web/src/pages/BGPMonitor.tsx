import { useEffect, useMemo, useRef, useState } from "react";
import { api, type BGPOverview, type BGPSession } from "@/lib/api";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { ErrorNote, Field, PageHeader, StatusBadge, TableState } from "@/components/app";
import { cn } from "@/lib/utils";
import { Pause, Play, RefreshCw } from "lucide-react";

const REFRESH_MS = 5000;
const STALE_SEC = 60;

function formatUptime(seconds: number) {
  if (!seconds || seconds < 0) return "-";
  const d = Math.floor(seconds / 86400);
  const h = Math.floor((seconds % 86400) / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  const s = seconds % 60;
  if (d > 0) return `${d}d ${h}h`;
  if (h > 0) return `${h}h ${m}m`;
  if (m > 0) return `${m}m ${s}s`;
  return `${s}s`;
}

function relativeTime(iso: string) {
  if (!iso) return "-";
  const then = new Date(iso).getTime();
  if (Number.isNaN(then)) return "-";
  const diff = Math.floor((Date.now() - then) / 1000);
  if (diff < 5) return "just now";
  if (diff < 60) return `${diff}s ago`;
  if (diff < 3600) return `${Math.floor(diff / 60)}m ago`;
  return `${Math.floor(diff / 3600)}h ago`;
}

function isStale(iso: string) {
  if (!iso) return true;
  const then = new Date(iso).getTime();
  if (Number.isNaN(then)) return true;
  return (Date.now() - then) / 1000 > STALE_SEC;
}

export default function BGPMonitor() {
  const [overview, setOverview] = useState<BGPOverview | null>(null);
  const [query, setQuery] = useState("");
  const [stateFilter, setStateFilter] = useState("all");
  const [live, setLive] = useState(true);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  const [updatedAt, setUpdatedAt] = useState<Date | null>(null);
  const liveRef = useRef(live);
  useEffect(() => {
    liveRef.current = live;
  }, [live]);

  const load = () => {
    api
      .getBGPOverview()
      .then((r) => {
        setOverview(r);
        setError("");
        setUpdatedAt(new Date());
      })
      .catch((err) =>
        setError(err instanceof Error ? err.message : "Failed to load"),
      )
      .finally(() => setLoading(false));
  };

  useEffect(() => {
    load();
    const timer = setInterval(() => {
      if (liveRef.current) load();
    }, REFRESH_MS);
    return () => clearInterval(timer);
  }, []);

  const sessions = useMemo(() => overview?.items ?? [], [overview]);
  const agents = useMemo(() => {
    const map = new Map<string, { name: string; engine: string; total: number; up: number }>();
    for (const sess of sessions) {
      const entry =
        map.get(sess.agent_id) ??
        {
          name: sess.agent_name || sess.agent_id,
          engine: overview?.engines?.[sess.agent_id] || "",
          total: 0,
          up: 0,
        };
      entry.total++;
      if (sess.state.toLowerCase() === "established") entry.up++;
      map.set(sess.agent_id, entry);
    }
    return Array.from(map.entries()).sort((a, b) =>
      a[1].name.localeCompare(b[1].name),
    );
  }, [sessions, overview]);

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    return sessions.filter((sess) => {
      if (stateFilter !== "all" && sess.state.toLowerCase() !== stateFilter)
        return false;
      if (!q) return true;
      return (
        sess.peer.toLowerCase().includes(q) ||
        sess.name.toLowerCase().includes(q) ||
        sess.agent_name?.toLowerCase().includes(q) ||
        sess.agent_id.toLowerCase().includes(q) ||
        String(sess.remote_as).includes(q)
      );
    });
  }, [sessions, query, stateFilter]);

  const stateOptions = useMemo(() => {
    const set = new Set<string>();
    for (const sess of sessions) set.add(sess.state.toLowerCase());
    return ["all", ...Array.from(set).sort()];
  }, [sessions]);

  const summary = [
    { label: "Agents", value: overview?.agents },
    { label: "Sessions", value: overview?.sessions },
    { label: "Established", value: overview?.established },
    { label: "Routes announced", value: overview?.prefixes_out },
  ];

  return (
    <div className="space-y-4">
      <PageHeader
        title="BGP Monitor"
        description="Sessions reported by agents running the embedded BGP server. The server is announce-only."
      >
        <span className="text-muted-foreground">
          <StatusBadge on={live} onLabel="live" offLabel="paused" />{" "}
          {updatedAt ? `updated ${relativeTime(updatedAt.toISOString())}` : ""}
        </span>
        <Button variant="outline" size="sm" onClick={load}>
          <RefreshCw aria-hidden="true" />
          Refresh
        </Button>
        <Button variant="outline" size="sm" aria-pressed={!live} onClick={() => setLive((v) => !v)}>
          {live ? (
            <>
              <Pause aria-hidden="true" /> Pause
            </>
          ) : (
            <>
              <Play aria-hidden="true" /> Resume
            </>
          )}
        </Button>
      </PageHeader>

      <ErrorNote>{error && `Telemetry error: ${error}`}</ErrorNote>

      <dl className="grid grid-cols-2 gap-px overflow-hidden rounded-lg border bg-border lg:grid-cols-4">
        {summary.map((item) => (
          <div key={item.label} className="bg-card px-3 py-2">
            <dt className="text-xs text-muted-foreground">{item.label}</dt>
            <dd className="font-mono text-base">
              {item.value === undefined ? "-" : item.value.toLocaleString()}
            </dd>
          </div>
        ))}
      </dl>

      <section aria-labelledby="bgp-agents-h" className="space-y-2">
        <h2 id="bgp-agents-h" className="font-semibold">
          Agents
        </h2>
        <div className="flex flex-wrap gap-2">
          {agents.map(([id, info]) => (
            <div key={id} className="flex items-center gap-2 rounded-md border bg-card px-3 py-1.5">
              <span className="font-medium">{info.name}</span>
              <span className="text-xs text-muted-foreground">{info.engine || "unknown engine"}</span>
              <span className={cn("font-mono text-xs", info.up > 0 ? "text-signal" : "text-warn")}>
                {info.up}/{info.total} up
              </span>
            </div>
          ))}
          {agents.length === 0 && (
            <span className="text-muted-foreground">
              {loading
                ? "Loading…"
                : "No agents have reported BGP sessions yet. Heartbeats arrive every 15 seconds."}
            </span>
          )}
        </div>
      </section>

      <section aria-labelledby="bgp-sessions-h" className="space-y-2">
        <h2 id="bgp-sessions-h" className="font-semibold">
          Sessions
        </h2>
        <div className="flex flex-wrap items-end gap-2">
          <Field label="Filter" htmlFor="bgp-filter">
            <Input
              id="bgp-filter"
              type="search"
              placeholder="Peer, agent, or ASN"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              className="w-64"
            />
          </Field>
          <Field label="State" htmlFor="bgp-state">
            <select
              id="bgp-state"
              value={stateFilter}
              onChange={(e) => setStateFilter(e.target.value)}
              className="h-8 rounded-md border bg-card px-2 pointer-coarse:h-11"
            >
              {stateOptions.map((state) => (
                <option key={state} value={state}>
                  {state === "all" ? "All states" : state}
                </option>
              ))}
            </select>
          </Field>
        </div>

        <div className="rounded-lg border bg-card">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Agent</TableHead>
                <TableHead>Peer</TableHead>
                <TableHead>Remote AS</TableHead>
                <TableHead>Type</TableHead>
                <TableHead>State</TableHead>
                <TableHead>Uptime</TableHead>
                <TableHead className="text-right">Announced</TableHead>
                <TableHead>Last report</TableHead>
                <TableHead>Last error</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {filtered.map((sess: BGPSession) => {
                const up = sess.state.toLowerCase() === "established";
                const stale = isStale(sess.updated_at);
                return (
                  <TableRow
                    key={`${sess.agent_id}-${sess.peer}`}
                    className={stale ? "text-muted-foreground" : ""}
                  >
                    <TableCell>
                      <div className="font-medium">{sess.agent_name || sess.agent_id}</div>
                      <div className="text-xs text-muted-foreground">
                        {overview?.engines?.[sess.agent_id] || "-"}
                      </div>
                    </TableCell>
                    <TableCell>
                      <div className="font-mono">{sess.peer}</div>
                      {sess.name && <div className="text-xs text-muted-foreground">{sess.name}</div>}
                    </TableCell>
                    <TableCell className="font-mono">AS{sess.remote_as}</TableCell>
                    <TableCell>{sess.passive ? "inbound" : "outbound"}</TableCell>
                    <TableCell>
                      <span
                        className={cn(
                          "inline-flex items-center gap-1.5 font-medium",
                          up ? "text-signal" : "text-warn",
                        )}
                      >
                        <span
                          aria-hidden="true"
                          className={cn("size-2 rounded-full", up ? "bg-signal" : "border border-current")}
                        />
                        {sess.state || "unknown"}
                      </span>
                    </TableCell>
                    <TableCell>{formatUptime(sess.uptime_sec)}</TableCell>
                    <TableCell className="text-right font-mono">
                      {sess.prefixes_out === undefined ? "-" : sess.prefixes_out.toLocaleString()}
                    </TableCell>
                    <TableCell className={stale ? "text-warn" : ""}>
                      <span>{relativeTime(sess.updated_at)}</span>
                      {stale && <span> (stale)</span>}
                    </TableCell>
                    <TableCell
                      className="max-w-[16rem] truncate text-xs text-muted-foreground"
                      title={sess.last_error || undefined}
                    >
                      {sess.last_error || "-"}
                    </TableCell>
                  </TableRow>
                );
              })}
              <TableState
                colSpan={9}
                loading={loading}
                error={error && !overview ? error : ""}
                empty={filtered.length === 0}
                emptyText={
                  sessions.length === 0
                    ? "No BGP sessions. Downstream routers must peer with an agent on TCP/179."
                    : "No BGP sessions match the current filters."
                }
                onRetry={load}
              />
            </TableBody>
          </Table>
        </div>
      </section>
    </div>
  );
}
