import { Link } from "react-router";
import { api } from "@/lib/api";
import { useResource } from "@/hooks/use-api";
import {
  ErrorNote,
  PageHeader,
  StatusBadge,
  TableState,
} from "@/components/app";
import { agentOnline, relativeTime } from "@/lib/format";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { cn } from "@/lib/utils";

const REFRESH_MS = 10000;

function n(v: number | undefined) {
  return (v ?? 0).toLocaleString();
}

// A source is stale when it missed three sync intervals.
function sourceStale(lastSync: string | null, intervalSec: number) {
  if (!lastSync) return true;
  const age = (Date.now() - new Date(lastSync).getTime()) / 1000;
  return age > Math.max(intervalSec, 60) * 3;
}

export default function Dashboard() {
  const status = useResource(api.getStatus, REFRESH_MS);
  const bgp = useResource(api.getBGPOverview, REFRESH_MS);
  const agents = useResource(api.getAgents, REFRESH_MS);
  const sources = useResource(api.getSources, REFRESH_MS);

  const s = status.data;
  const b = bgp.data;
  const online = (agents.data?.items ?? []).filter((a) => agentOnline(a.status)).length;
  const sourceList = sources.data?.items ?? [];

  // The one question this page answers: is the feed reaching routers?
  let tone: "live" | "warn" | "down" = "down";
  let headline = "Checking…";
  let detail = "";
  if (s && b && agents.data) {
    if (online === 0) {
      headline = "No agent online";
      detail = "Routes are not being announced. Check rtbhd on the agent hosts.";
    } else if (b.established === 0) {
      tone = "warn";
      headline = "Not reaching any router";
      detail =
        b.sessions === 0
          ? "No BGP sessions. Downstream routers must peer with the agent."
          : `0 of ${b.sessions} BGP sessions established.`;
    } else {
      tone = "live";
      headline = "Live";
      detail = `Announced on ${b.established} of ${b.sessions} BGP sessions.`;
    }
  }
  const loadError = status.error || bgp.error || agents.error;

  return (
    <div className="space-y-4">
      <PageHeader title="Dashboard" />

      <section
        aria-label="Blackhole feed status"
        className={cn(
          "rounded-lg border bg-card p-4",
          tone === "live" && "border-signal/50",
          tone === "warn" && "border-warn/60",
          tone === "down" && s && "border-destructive/60",
        )}
      >
        <div className="flex flex-wrap items-baseline gap-x-6 gap-y-2">
          <div>
            <div className="text-xs text-muted-foreground">Prefixes in blackhole feed</div>
            <div className="font-mono text-3xl font-medium">{s ? n(s.effective_prefixes) : "-"}</div>
          </div>
          <div className="min-w-0 flex-1">
            <div
              className={cn(
                "flex items-center gap-2 text-base font-semibold",
                tone === "live" && "text-signal",
                tone === "warn" && "text-warn",
                tone === "down" && s && "text-destructive",
              )}
            >
              <span
                aria-hidden="true"
                className={cn(
                  "size-2.5 rounded-full",
                  tone === "live" && "bg-signal",
                  tone === "warn" && "bg-warn",
                  tone === "down" && "bg-destructive",
                  !s && "bg-muted-foreground",
                )}
              />
              {headline}
            </div>
            <p className="text-muted-foreground">{detail}</p>
          </div>
          {b && b.prefixes_out !== undefined && b.established > 0 && (
            <div>
              <div className="text-xs text-muted-foreground">Routes announced (all sessions)</div>
              <div className="font-mono text-base">{n(b.prefixes_out)}</div>
            </div>
          )}
        </div>
        <ErrorNote>{loadError && `Some data could not be loaded: ${loadError}`}</ErrorNote>
      </section>

      <dl className="grid grid-cols-2 gap-px overflow-hidden rounded-lg border bg-border sm:grid-cols-3 lg:grid-cols-6">
        {[
          { label: "Agents online", value: s ? `${online} / ${n(s.agents)}` : "-", to: "/agents" },
          { label: "Manual prefixes", value: s ? n(s.prefixes) : "-", to: "/prefixes" },
          { label: "Sources", value: s ? n(s.sources) : "-", to: "/sources" },
          { label: "Excludes", value: s ? n(s.excludes) : "-", to: "/excludes" },
          { label: "Static peers", value: s ? n(s.peers) : "-", to: "/peers" },
          {
            label: "Whitelisted ASNs",
            value: s ? `${n(s.whitelist)}${s.auto_accept === false ? " (auto-accept off)" : ""}` : "-",
            to: "/whitelist",
          },
        ].map((item) => (
          <Link key={item.label} to={item.to} className="bg-card px-3 py-2 hover:bg-muted/60">
            <dt className="text-xs text-muted-foreground">{item.label}</dt>
            <dd className="font-mono text-base">{item.value}</dd>
          </Link>
        ))}
      </dl>

      <section aria-labelledby="feeds-h" className="space-y-2">
        <h2 id="feeds-h" className="font-semibold">
          Blocklist sources
        </h2>
        <div className="rounded-lg border bg-card">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Source</TableHead>
                <TableHead>Status</TableHead>
                <TableHead className="text-right">Prefixes</TableHead>
                <TableHead>Last sync</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {sourceList.map((src) => {
                const stale = src.enabled && !src.last_error && sourceStale(src.last_sync, src.interval);
                return (
                  <TableRow key={src.id}>
                    <TableCell className="font-medium">
                      <Link to={`/sources/${src.id}/prefixes`} className="hover:underline">
                        {src.name || src.id}
                      </Link>
                    </TableCell>
                    <TableCell>
                      {src.enabled && src.last_error ? (
                        <span className="font-medium text-destructive" title={src.last_error}>
                          Sync failing
                        </span>
                      ) : (
                        <StatusBadge on={src.enabled} onLabel="active" offLabel="paused" />
                      )}
                    </TableCell>
                    <TableCell className="text-right font-mono">{n(src.last_count)}</TableCell>
                    <TableCell className={cn(stale && "text-warn")}>
                      {relativeTime(src.last_sync)}
                      {stale && " (stale)"}
                    </TableCell>
                  </TableRow>
                );
              })}
              <TableState
                colSpan={4}
                loading={sources.loading}
                error={sources.error}
                empty={sourceList.length === 0}
                emptyText="No sources configured."
                onRetry={sources.reload}
              />
            </TableBody>
          </Table>
        </div>
      </section>
    </div>
  );
}
