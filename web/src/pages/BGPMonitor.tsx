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
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Activity, Pause, Play, RefreshCw, Search } from "lucide-react";

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
  const [updatedAt, setUpdatedAt] = useState<Date | null>(null);
  const liveRef = useRef(live);
  liveRef.current = live;

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
      );
  };

  useEffect(() => {
    load();
    const timer = setInterval(() => {
      if (liveRef.current) load();
    }, REFRESH_MS);
    return () => clearInterval(timer);
  }, []);

  const sessions = overview?.items ?? [];
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
    { label: "Agents", value: overview?.agents ?? 0 },
    { label: "Sessions", value: overview?.sessions ?? 0 },
    { label: "Established", value: overview?.established ?? 0 },
    { label: "Prefixes received", value: overview?.prefixes_in ?? 0 },
  ];

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-4 flex-wrap">
        <h2 className="text-2xl font-bold flex items-center gap-2">
          <Activity className="h-6 w-6" />
          BGP Monitor
        </h2>
        <Badge variant={live ? "default" : "secondary"}>
          {live ? "live" : "paused"}
        </Badge>
        <span className="text-sm text-muted-foreground">
          {updatedAt ? `updated ${relativeTime(updatedAt.toISOString())}` : "-"}
        </span>
        <Button variant="outline" size="sm" onClick={load}>
          <RefreshCw className="h-4 w-4 mr-1" />
          Refresh
        </Button>
        <Button
          variant="outline"
          size="sm"
          onClick={() => setLive((v) => !v)}
        >
          {live ? (
            <>
              <Pause className="h-4 w-4 mr-1" /> Pause
            </>
          ) : (
            <>
              <Play className="h-4 w-4 mr-1" /> Resume
            </>
          )}
        </Button>
      </div>

      {error && (
        <p className="text-sm text-destructive">Telemetry error: {error}</p>
      )}

      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        {summary.map((item) => (
          <Card key={item.label}>
            <CardHeader className="pb-2">
              <CardTitle className="text-sm font-medium text-muted-foreground">
                {item.label}
              </CardTitle>
            </CardHeader>
            <CardContent>
              <div className="text-3xl font-bold">{item.value}</div>
            </CardContent>
          </Card>
        ))}
      </div>

      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-base">Agents</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="flex flex-wrap gap-2">
            {agents.map(([id, info]) => (
              <div
                key={id}
                className="rounded-md border px-3 py-2 text-sm flex items-center gap-2"
              >
                <span className="font-medium">{info.name}</span>
                <Badge variant="outline">{info.engine || "unknown"}</Badge>
                <Badge variant={info.up > 0 ? "default" : "secondary"}>
                  {info.up}/{info.total} up
                </Badge>
              </div>
            ))}
            {agents.length === 0 && (
              <span className="text-sm text-muted-foreground">
                No agents have reported BGP sessions yet. Heartbeats arrive
                every 15 seconds.
              </span>
            )}
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-base">Sessions</CardTitle>
        </CardHeader>
        <CardContent className="space-y-3">
          <div className="flex gap-2 flex-wrap items-center">
            <div className="flex items-center gap-2">
              <Search className="h-4 w-4 text-muted-foreground" />
              <Input
                placeholder="Filter by peer, agent, or ASN"
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                className="max-w-[300px]"
              />
            </div>
            <select
              value={stateFilter}
              onChange={(e) => setStateFilter(e.target.value)}
              className="h-9 rounded-md border bg-background px-3 text-sm"
            >
              {stateOptions.map((state) => (
                <option key={state} value={state}>
                  {state === "all" ? "All states" : state}
                </option>
              ))}
            </select>
          </div>

          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Agent</TableHead>
                <TableHead>Peer</TableHead>
                <TableHead>Remote AS</TableHead>
                <TableHead>Type</TableHead>
                <TableHead>State</TableHead>
                <TableHead>Uptime</TableHead>
                <TableHead>Pfx in</TableHead>
                <TableHead>Last report</TableHead>
                <TableHead>Last error</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {filtered.map((sess: BGPSession) => {
                const up = sess.state.toLowerCase() === "established";
                return (
                  <TableRow
                    key={`${sess.agent_id}-${sess.peer}`}
                    className={isStale(sess.updated_at) ? "opacity-60" : ""}
                  >
                    <TableCell>
                      <div className="font-medium">
                        {sess.agent_name || sess.agent_id}
                      </div>
                      <div className="text-xs text-muted-foreground">
                        {overview?.engines?.[sess.agent_id] || "-"}
                      </div>
                    </TableCell>
                    <TableCell>
                      <div className="font-mono">{sess.peer}</div>
                      {sess.name && (
                        <div className="text-xs text-muted-foreground">
                          {sess.name}
                        </div>
                      )}
                    </TableCell>
                    <TableCell className="font-mono">AS{sess.remote_as}</TableCell>
                    <TableCell>
                      <Badge variant="outline">
                        {sess.passive ? "passive" : "active"}
                      </Badge>
                    </TableCell>
                    <TableCell>
                      <Badge variant={up ? "default" : "secondary"}>
                        {sess.state || "unknown"}
                      </Badge>
                    </TableCell>
                    <TableCell>{formatUptime(sess.uptime_sec)}</TableCell>
                    <TableCell>{sess.prefixes_in}</TableCell>
                    <TableCell
                      className={
                        isStale(sess.updated_at) ? "text-destructive" : ""
                      }
                    >
                      {relativeTime(sess.updated_at)}
                    </TableCell>
                    <TableCell className="max-w-[220px] truncate text-xs text-muted-foreground">
                      {sess.last_error || "-"}
                    </TableCell>
                  </TableRow>
                );
              })}
              {filtered.length === 0 && (
                <TableRow>
                  <TableCell
                    colSpan={9}
                    className="text-center text-muted-foreground py-8"
                  >
                    No BGP sessions match the current filters.
                  </TableCell>
                </TableRow>
              )}
            </TableBody>
          </Table>
        </CardContent>
      </Card>
    </div>
  );
}