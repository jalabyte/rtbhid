import { useState } from "react";
import { Link } from "react-router";
import type { Source } from "@/lib/api";
import { useAction, useResource } from "@/hooks/use-api";
import { Confirm, ErrorNote, Field, IconButton, TableState } from "@/components/app";
import { relativeTime } from "@/lib/format";
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
import { cn } from "@/lib/utils";
import { Pause, Play, Plus, RefreshCw, Trash2, X } from "lucide-react";

type Ops = {
  list: () => Promise<{ items: Source[] }>;
  add: (d: { name: string; url: string; interval: number }) => Promise<unknown>;
  toggle: (id: string, enabled: boolean) => Promise<unknown>;
  sync: (id: string) => Promise<unknown>;
  remove: (id: string) => Promise<unknown>;
};

const INTERVALS = [
  { sec: 300, label: "5 minutes" },
  { sec: 900, label: "15 minutes" },
  { sec: 1800, label: "30 minutes" },
  { sec: 3600, label: "1 hour" },
  { sec: 21600, label: "6 hours" },
  { sec: 43200, label: "12 hours" },
  { sec: 86400, label: "24 hours" },
];

function every(sec: number) {
  if (sec >= 172800 && sec % 86400 === 0) return `${sec / 86400}d`;
  if (sec % 3600 === 0) return `${sec / 3600}h`;
  if (sec % 60 === 0) return `${sec / 60}m`;
  return `${sec}s`;
}

type Health = "ok" | "failing" | "stale" | "never" | "paused";

// A source is stale when it missed three sync intervals.
function health(s: Source): Health {
  if (!s.enabled) return "paused";
  if (s.last_error) return "failing";
  if (!s.last_sync) return "never";
  const age = (Date.now() - new Date(s.last_sync).getTime()) / 1000;
  return age > Math.max(s.interval, 60) * 3 ? "stale" : "ok";
}

function nextSync(s: Source) {
  if (!s.last_sync || !s.enabled) return "";
  const due = new Date(s.last_sync).getTime() + s.interval * 1000 - Date.now();
  if (due <= 0) return "due now";
  const m = Math.round(due / 60000);
  return m < 60 ? `next in ${m}m` : `next in ${Math.round(m / 60)}h`;
}

// "fetch source: Get "<url>": dial tcp: ... no such host" -> "dial tcp: ... no such host"
function shortError(msg: string) {
  return msg.replace(/^fetch source: \w+ "[^"]*": /, "");
}

function splitURL(url: string) {
  try {
    const u = new URL(url);
    return { host: u.host, rest: u.pathname + u.search, insecure: u.protocol === "http:" };
  } catch {
    return { host: url, rest: "", insecure: false };
  }
}

const tone: Record<Health, string> = {
  ok: "text-signal",
  failing: "text-destructive",
  stale: "text-warn",
  never: "text-warn",
  paused: "text-muted-foreground",
};

function HealthCell({ s, syncing }: { s: Source; syncing: boolean }) {
  const h = health(s);
  const label = {
    ok: `Synced ${relativeTime(s.last_sync)}`,
    failing: "Sync failing",
    stale: `Stale, last ${relativeTime(s.last_sync)}`,
    never: "Never synced",
    paused: "Paused",
  }[h];
  return (
    <div className="min-w-0">
      <div className={cn("flex items-center gap-1.5 font-medium", tone[h])}>
        <span
          aria-hidden="true"
          className={cn(
            "size-2 shrink-0 rounded-full",
            h === "ok" && "bg-signal",
            h === "failing" && "bg-destructive",
            (h === "stale" || h === "never") && "bg-warn",
            h === "paused" && "border border-current",
          )}
        />
        {syncing ? "Syncing…" : label}
      </div>
      <div className="line-clamp-2 text-xs text-muted-foreground" title={s.last_error || undefined}>
        {h === "failing"
          ? `${shortError(s.last_error ?? "")}${s.last_sync ? ` · last good ${relativeTime(s.last_sync)}` : ""}`
          : `every ${every(s.interval)}${nextSync(s) ? ` · ${nextSync(s)}` : ""}`}
      </div>
    </div>
  );
}

// SourceManager is the list + add panel shared by blocklist sources and
// remote exclude sources.
export function SourceManager({
  ops,
  idPrefix,
  noun,
  removeHint,
  viewHref,
}: {
  ops: Ops;
  idPrefix: string;
  noun: string;
  removeHint: string;
  viewHref?: (s: Source) => string;
}) {
  // A manual sync runs in the background on the portal. A source counts as
  // syncing until it reports a new sync time or an error (or 60s pass).
  const [syncing, setSyncing] = useState<Record<string, string | null>>({});
  const isSyncing = (s: Source) =>
    s.id in syncing && s.last_sync === syncing[s.id] && !s.last_error;
  const [pollFast, setPollFast] = useState(false);
  const { data, error, loading, reload } = useResource(ops.list, pollFast ? 2000 : 30000);
  const action = useAction(reload);
  const [adding, setAdding] = useState(false);
  const [form, setForm] = useState({ name: "", url: "", interval: "3600" });
  const sources = data?.items ?? [];
  const busySync = sources.some(isSyncing);
  if (pollFast && !busySync && data) setPollFast(false);

  const total = sources.filter((s) => s.enabled).reduce((n, s) => n + s.last_count, 0);
  const failing = sources.filter((s) => health(s) === "failing").length;
  const attention = sources.filter((s) => ["stale", "never"].includes(health(s))).length;

  const handleAdd = async () => {
    if (!form.url.trim()) return action.setError("URL is required");
    const ok = await action.run(() =>
      ops.add({
        name: form.name.trim(),
        url: form.url.trim(),
        interval: parseInt(form.interval, 10) || 3600,
      }),
    );
    if (ok) {
      setForm({ name: "", url: "", interval: "3600" });
      setAdding(false);
    }
  };

  const dropSyncing = (id: string) =>
    setSyncing((prev) => {
      const next = { ...prev };
      delete next[id];
      return next;
    });

  const handleSync = async (s: Source) => {
    setSyncing((prev) => ({ ...prev, [s.id]: s.last_sync }));
    setPollFast(true);
    setTimeout(() => dropSyncing(s.id), 60000);
    const ok = await action.run(() => ops.sync(s.id));
    if (!ok) dropSyncing(s.id);
  };

  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-center gap-x-4 gap-y-2">
        <p className="mr-auto text-muted-foreground" aria-live="polite">
          {sources.length > 0 && (
            <>
              <span className="text-foreground">{sources.length}</span> {noun}
              {sources.length === 1 ? "" : "s"} ·{" "}
              <span className="font-mono text-foreground">{total.toLocaleString()}</span> prefixes
              {failing > 0 && <span className="text-destructive"> · {failing} failing</span>}
              {attention > 0 && <span className="text-warn"> · {attention} need attention</span>}
              {failing + attention === 0 && <span className="text-signal"> · all healthy</span>}
            </>
          )}
        </p>
        <Button
          variant={adding ? "outline" : "default"}
          size="sm"
          aria-expanded={adding}
          aria-controls={`${idPrefix}-add`}
          onClick={() => setAdding((v) => !v)}
        >
          {adding ? <X aria-hidden="true" /> : <Plus aria-hidden="true" />}
          {adding ? "Cancel" : `Add ${noun}`}
        </Button>
      </div>

      {adding && (
        <form
          id={`${idPrefix}-add`}
          className="flex flex-wrap items-end gap-2 rounded-lg border bg-card p-3"
          onSubmit={(e) => {
            e.preventDefault();
            handleAdd();
          }}
        >
          <Field label="Name" htmlFor={`${idPrefix}-name`}>
            <Input
              id={`${idPrefix}-name`}
              value={form.name}
              onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))}
              className="w-40"
              autoFocus
            />
          </Field>
          <Field label="List URL (one IP or CIDR per line)" htmlFor={`${idPrefix}-url`} className="min-w-0 flex-1 basis-72">
            <Input
              id={`${idPrefix}-url`}
              type="url"
              required
              placeholder="https://example.net/list.txt"
              value={form.url}
              onChange={(e) => setForm((f) => ({ ...f, url: e.target.value }))}
              className="font-mono"
            />
          </Field>
          <Field label="Fetch every" htmlFor={`${idPrefix}-interval`}>
            <select
              id={`${idPrefix}-interval`}
              value={form.interval}
              onChange={(e) => setForm((f) => ({ ...f, interval: e.target.value }))}
              className="h-8 rounded-md border bg-card px-2 pointer-coarse:h-11"
            >
              {INTERVALS.map((i) => (
                <option key={i.sec} value={i.sec}>
                  {i.label}
                </option>
              ))}
            </select>
          </Field>
          <Button type="submit" disabled={action.busy}>
            Add and sync
          </Button>
        </form>
      )}
      <ErrorNote>{action.error}</ErrorNote>

      <div className="rounded-lg border bg-card">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Source</TableHead>
              <TableHead className="w-64">Health</TableHead>
              <TableHead className="w-56">Prefixes</TableHead>
              <TableHead className="w-44 text-right">Actions</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {sources.map((s) => {
              const name = s.name || s.id;
              const url = splitURL(s.url);
              const share = total && s.enabled ? s.last_count / total : 0;
              const syncingNow = isSyncing(s);
              return (
                <TableRow key={s.id} className={cn(!s.enabled && "text-muted-foreground")}>
                  <TableCell className="max-w-0 whitespace-normal">
                    <div className="font-medium">
                      {viewHref ? (
                        <Link to={viewHref(s)} className="hover:underline">
                          {name}
                        </Link>
                      ) : (
                        name
                      )}
                    </div>
                    <div className="truncate font-mono text-xs text-muted-foreground" title={s.url}>
                      {url.insecure && (
                        <span className="text-warn" title="Fetched over plain HTTP: the list can be altered in transit">
                          http{" "}
                        </span>
                      )}
                      <span className="text-foreground/80">{url.host}</span>
                      {url.rest}
                    </div>
                  </TableCell>
                  <TableCell className="max-w-64 whitespace-normal">
                    <HealthCell s={s} syncing={syncingNow} />
                  </TableCell>
                  <TableCell>
                    <div className="flex items-baseline justify-between gap-2">
                      {viewHref ? (
                        <Link to={viewHref(s)} className="font-mono hover:underline" title="View prefixes">
                          {s.last_count.toLocaleString()}
                        </Link>
                      ) : (
                        <span className="font-mono">{s.last_count.toLocaleString()}</span>
                      )}
                      {s.enabled && total > 0 && (
                        <span className="text-xs text-muted-foreground">{Math.round(share * 100)}%</span>
                      )}
                    </div>
                    {/* share of the active feed: real counts, not decoration */}
                    <div className="mt-1 h-1 overflow-hidden rounded-full bg-muted" aria-hidden="true">
                      <div className="h-full bg-signal/70" style={{ width: `${share * 100}%` }} />
                    </div>
                  </TableCell>
                  <TableCell>
                    <div className="flex justify-end gap-1">
                      <Button
                        variant="ghost"
                        size="sm"
                        disabled={syncingNow || !s.enabled}
                        onClick={() => handleSync(s)}
                        aria-label={`Sync ${name} now`}
                      >
                        <RefreshCw aria-hidden="true" className={cn(syncingNow && "animate-spin")} />
                        Sync
                      </Button>
                      <IconButton
                        icon={s.enabled ? Pause : Play}
                        label={`${s.enabled ? "Pause" : "Resume"} ${name}`}
                        disabled={action.busy}
                        onClick={() => action.run(() => ops.toggle(s.id, !s.enabled))}
                      />
                      <Confirm
                        title={`Delete ${noun} ${name}?`}
                        description={removeHint}
                        confirmLabel={`Delete ${noun}`}
                        onConfirm={() => action.run(() => ops.remove(s.id))}
                      >
                        <IconButton icon={Trash2} label={`Delete ${name}`} />
                      </Confirm>
                    </div>
                  </TableCell>
                </TableRow>
              );
            })}
            <TableState
              colSpan={4}
              loading={loading}
              error={error}
              empty={sources.length === 0}
              emptyText={`No ${noun}s yet. Add one with its list URL.`}
              onRetry={reload}
            />
          </TableBody>
        </Table>
      </div>
    </div>
  );
}
