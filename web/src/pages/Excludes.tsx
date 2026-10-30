import { useMemo, useState } from "react";
import { Link } from "react-router";
import { api, type Exclude } from "@/lib/api";
import { useAction, useResource } from "@/hooks/use-api";
import { Confirm, ErrorNote, Field, IconButton, PageHeader, TableState } from "@/components/app";
import { SourceManager } from "@/components/SourceManager";
import { formatDate, relativeTime } from "@/lib/format";
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
import { Textarea } from "@/components/ui/textarea";
import { Plus, Search, Trash2, X } from "lucide-react";

const remoteOps = {
  list: api.getExcludeSources,
  add: api.addExcludeSource,
  toggle: api.toggleExcludeSource,
  sync: api.syncExcludeSource,
  remove: api.deleteExcludeSource,
};

// "/24 · 256 addresses": makes a wide exclude (a /8 hides 16.8M addresses
// from every source) stand out from a single host.
function rangeSize(cidr: string) {
  const [addr, b] = cidr.split("/");
  const v6 = addr.includes(":");
  const bits = Number(b ?? (v6 ? 128 : 32));
  const host = v6 ? 128 - bits : 32 - bits;
  if (host === 0) return "single address";
  if (v6) return `/${bits}`;
  return `/${bits} · ${new Intl.NumberFormat("en", { notation: "compact" }).format(2 ** host)} addresses`;
}

export default function Excludes() {
  const { data, error, loading, reload } = useResource(api.getExcludes);
  const action = useAction(reload);
  const [adding, setAdding] = useState(false);
  const [text, setText] = useState("");
  const [note, setNote] = useState("");
  const [filter, setFilter] = useState("");
  const [result, setResult] = useState("");
  const excludes: Exclude[] = useMemo(() => data?.items ?? [], [data]);

  const entries = text.split(/[\s,;]+/).map((s) => s.trim()).filter(Boolean);
  const shown = useMemo(() => {
    const q = filter.trim().toLowerCase();
    return q ? excludes.filter((e) => e.cidr.includes(q) || e.note.toLowerCase().includes(q)) : excludes;
  }, [excludes, filter]);

  const handleAdd = async () => {
    if (entries.length === 0) return action.setError("Enter at least one CIDR or IP");
    setResult("");
    let msg = "";
    const ok = await action.run(async () => {
      if (entries.length === 1) {
        await api.addExclude({ cidr: entries[0], note: note.trim() });
        msg = `Added ${entries[0]}.`;
      } else {
        const r = await api.bulkAddExcludes({ items: entries, note: note.trim() });
        msg = `Added ${r.created}, ${r.skipped} already present, ${r.invalid} invalid.`;
      }
    });
    if (ok) {
      setResult(msg);
      setText("");
      setNote("");
      setAdding(false);
    }
  };

  return (
    <div className="space-y-6">
      <PageHeader
        title="Excludes"
        description="Excluded ranges are never blackholed from sources, and a covering CIDR also excludes everything inside it. Excludes do not override manual prefixes."
      />

      <section aria-labelledby="manual-ex-h" className="space-y-3">
        <div className="flex flex-wrap items-center gap-x-3 gap-y-2">
          <h2 id="manual-ex-h" className="font-semibold">
            Manual excludes
          </h2>
          <span className="mr-auto text-muted-foreground">{excludes.length} ranges</span>
          {excludes.length > 8 && (
            <>
              <label htmlFor="ex-filter" className="sr-only">
                Filter excludes
              </label>
              <Input
                id="ex-filter"
                type="search"
                placeholder="Filter"
                value={filter}
                onChange={(e) => setFilter(e.target.value)}
                className="w-48"
              />
            </>
          )}
          <Button
            size="sm"
            variant={adding ? "outline" : "default"}
            aria-expanded={adding}
            aria-controls="ex-add"
            onClick={() => setAdding((v) => !v)}
          >
            {adding ? <X aria-hidden="true" /> : <Plus aria-hidden="true" />}
            {adding ? "Cancel" : "Add excludes"}
          </Button>
        </div>

        {adding && (
          <form
            id="ex-add"
            className="grid gap-3 rounded-lg border bg-card p-3 sm:grid-cols-[minmax(0,1fr)_16rem]"
            onSubmit={(e) => {
              e.preventDefault();
              handleAdd();
            }}
          >
            <Field label="CIDRs or IPs (one per line, or separated by spaces)" htmlFor="ex-cidrs">
              <Textarea
                id="ex-cidrs"
                rows={3}
                placeholder={"192.0.2.0/24\n2001:db8::/32"}
                value={text}
                onChange={(e) => setText(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) {
                    e.preventDefault();
                    handleAdd();
                  }
                }}
                className="min-h-0 font-mono"
                spellCheck={false}
                autoFocus
              />
            </Field>
            <div className="flex flex-col gap-2">
              <Field label="Note" htmlFor="ex-note">
                <Input
                  id="ex-note"
                  placeholder="e.g. own ranges, upstream DNS"
                  value={note}
                  onChange={(e) => setNote(e.target.value)}
                />
              </Field>
              <Button type="submit" disabled={action.busy || entries.length === 0} className="mt-auto">
                Add {entries.length > 1 ? `${entries.length} excludes` : "exclude"}
              </Button>
            </div>
          </form>
        )}
        <ErrorNote>{action.error}</ErrorNote>
        {result && (
          <p role="status" className="text-signal">
            {result}
          </p>
        )}

        <div className="rounded-lg border bg-card">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Range</TableHead>
                <TableHead>Note</TableHead>
                <TableHead className="w-32">Added</TableHead>
                <TableHead className="w-20 text-right">Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {shown.map((e) => (
                <TableRow key={e.id}>
                  <TableCell>
                    <div className="font-mono">{e.cidr}</div>
                    <div className="text-xs text-muted-foreground">{rangeSize(e.cidr)}</div>
                  </TableCell>
                  <TableCell className="max-w-md truncate" title={e.note || undefined}>
                    {e.note || <span className="text-muted-foreground">-</span>}
                  </TableCell>
                  <TableCell title={formatDate(e.created_at)}>{relativeTime(e.created_at)}</TableCell>
                  <TableCell>
                    <div className="flex justify-end gap-1">
                      <Button asChild variant="ghost" size="icon-sm">
                        <Link
                          to={`/lookup?q=${encodeURIComponent(e.cidr)}`}
                          aria-label={`Check ${e.cidr} in Lookup`}
                          title="Check in Lookup"
                        >
                          <Search className="size-4" aria-hidden="true" />
                        </Link>
                      </Button>
                      <Confirm
                        title={`Delete exclude ${e.cidr}?`}
                        description="Source prefixes inside it become blackholed again on the next sync."
                        confirmLabel="Delete exclude"
                        onConfirm={() => action.run(() => api.deleteExclude(e.id))}
                      >
                        <IconButton icon={Trash2} label={`Delete exclude ${e.cidr}`} />
                      </Confirm>
                    </div>
                  </TableCell>
                </TableRow>
              ))}
              <TableState
                colSpan={4}
                loading={loading}
                error={error}
                empty={shown.length === 0}
                emptyText={
                  filter
                    ? "No excludes match the filter."
                    : "No manual excludes. Add your own ranges so a bad feed entry can never blackhole them."
                }
                onRetry={reload}
              />
            </TableBody>
          </Table>
        </div>
      </section>

      <section aria-labelledby="remote-ex-h" className="space-y-3">
        <div>
          <h2 id="remote-ex-h" className="font-semibold">
            Remote exclude sources
          </h2>
          <p className="text-muted-foreground">
            Lists fetched on an interval, such as a CDN's published ranges.{" "}
            <Link to="/exclude-prefixes" className="underline-offset-2 hover:underline">
              View all synced exclude prefixes
            </Link>
          </p>
        </div>
        <SourceManager
          ops={remoteOps}
          idPrefix="exsrc"
          noun="exclude source"
          removeHint="Prefixes it excluded become blackholed again on the next sync."
        />
      </section>
    </div>
  );
}
