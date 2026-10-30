import { useMemo, useState, type ReactNode } from "react";
import { api } from "@/lib/api";
import { errorMessage } from "@/hooks/use-api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Confirm, ErrorNote, Field, PageHeader } from "@/components/app";
import { cn } from "@/lib/utils";
import { Ban, Check, Copy, Search, ShieldAlert } from "lucide-react";

const IRR_SOURCES = [
  "RADB",
  "RIPE",
  "ARIN",
  "APNIC",
  "AFRINIC",
  "LACNIC",
  "NTTCOM",
  "ALTDB",
  "BELL",
];

type Result = { asn: string; prefixes: string[] };

// ponytail: plain list capped at RENDER_CAP rows; virtualize if operators
// need to scroll 20k+ prefixes instead of filtering.
const RENDER_CAP = 2000;

// IPv4 address count covered by a prefix list; IPv6 is reported as a count of
// prefixes only (address totals there are not meaningful to an operator).
function v4Addresses(prefixes: string[]) {
  let n = 0;
  for (const p of prefixes) {
    if (p.includes(":")) continue;
    const bits = Number(p.split("/")[1] ?? 32);
    n += 2 ** (32 - bits);
  }
  return n;
}

function compact(n: number) {
  return new Intl.NumberFormat("en", { notation: "compact", maximumFractionDigits: 1 }).format(n);
}

// Toggle is a pressed/unpressed button used for families and IRR sources.
function Toggle({
  pressed,
  onClick,
  children,
}: {
  pressed: boolean;
  onClick: () => void;
  children: ReactNode;
}) {
  return (
    <button
      type="button"
      aria-pressed={pressed}
      onClick={onClick}
      className={cn(
        "inline-flex h-7 items-center gap-1.5 rounded-md border px-2 font-mono text-xs pointer-coarse:min-h-11",
        pressed
          ? "border-signal/60 bg-signal/10 text-foreground"
          : "text-muted-foreground hover:text-foreground",
      )}
    >
      <span
        aria-hidden="true"
        className={cn("size-1.5 rounded-full", pressed ? "bg-signal" : "border border-current")}
      />
      {children}
    </button>
  );
}

export default function PrefixBuilder() {
  const [asn, setAsn] = useState("");
  const [ipv4, setIpv4] = useState(true);
  const [ipv6, setIpv6] = useState(true);
  const [sources, setSources] = useState<string[]>(IRR_SOURCES);
  const [result, setResult] = useState<Result | null>(null);
  const [dropped, setDropped] = useState<Set<string>>(new Set());
  const [filter, setFilter] = useState("");
  const [note, setNote] = useState("");
  const [loading, setLoading] = useState(false);
  const [adding, setAdding] = useState(false);
  const [error, setError] = useState("");
  const [status, setStatus] = useState("");
  const [copied, setCopied] = useState(false);

  const all = useMemo(() => result?.prefixes ?? [], [result]);
  const selected = useMemo(() => all.filter((p) => !dropped.has(p)), [all, dropped]);
  const shown = useMemo(() => {
    const q = filter.trim();
    return q ? all.filter((p) => p.includes(q)) : all;
  }, [all, filter]);
  const v6Count = selected.filter((p) => p.includes(":")).length;
  const v4Count = selected.length - v6Count;

  const toggleSource = (s: string) =>
    setSources((prev) => (prev.includes(s) ? prev.filter((x) => x !== s) : [...prev, s]));

  const handleBuild = async () => {
    setError("");
    setStatus("");
    const trimmed = asn.trim();
    if (!trimmed) return setError("Enter an ASN, for example AS7713.");
    if (!ipv4 && !ipv6) return setError("Pick IPv4, IPv6, or both.");
    if (sources.length === 0) return setError("Pick at least one IRR source.");
    setLoading(true);
    try {
      const r = await api.buildPrefixesFromASN({ asn: trimmed, ipv4, ipv6, sources });
      setResult({ asn: r.asn || trimmed, prefixes: r.prefixes ?? [] });
      setDropped(new Set());
      setFilter("");
      if (!note) setNote(`IRR ${(r.asn || trimmed).toUpperCase()}`);
    } catch (err) {
      setResult(null);
      setError(errorMessage(err, "IRR query failed"));
    } finally {
      setLoading(false);
    }
  };

  const commit = async (target: "prefixes" | "excludes") => {
    if (selected.length === 0) return;
    setAdding(true);
    setError("");
    setStatus("");
    try {
      const payload = { items: selected, note: note.trim() };
      const r =
        target === "prefixes" ? await api.bulkAddPrefixes(payload) : await api.bulkAddExcludes(payload);
      setStatus(
        `${target === "prefixes" ? "Blackholed" : "Excluded"}: ${r.created} added, ${r.skipped} already present, ${r.invalid} invalid.`,
      );
    } catch (err) {
      setError(errorMessage(err, "Failed to add prefixes"));
    } finally {
      setAdding(false);
    }
  };

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(selected.join("\n"));
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    } catch {
      setError("Clipboard is not available in this browser.");
    }
  };

  const setShown = (keep: boolean) =>
    setDropped((prev) => {
      const next = new Set(prev);
      for (const p of shown) {
        if (keep) next.delete(p);
        else next.add(p);
      }
      return next;
    });

  return (
    <div className="space-y-4">
      <PageHeader
        title="Prefix Builder"
        description="Expand an ASN into its IRR route objects, review the list, then blackhole or exclude it."
      />

      <div className="grid items-start gap-4 lg:grid-cols-[20rem_minmax(0,1fr)]">
        {/* 1. Query */}
        <form
          aria-labelledby="pb-query-h"
          className="space-y-4 rounded-lg border bg-card p-4 lg:sticky lg:top-0"
          onSubmit={(e) => {
            e.preventDefault();
            handleBuild();
          }}
        >
          <h2 id="pb-query-h" className="text-xs text-muted-foreground">
            1 · Query IRR
          </h2>
          <Field label="ASN" htmlFor="pb-asn">
            <Input
              id="pb-asn"
              placeholder="e.g. AS7713"
              value={asn}
              onChange={(e) => setAsn(e.target.value)}
              className="h-10 font-mono text-base"
              autoComplete="off"
              spellCheck={false}
            />
          </Field>

          <fieldset className="space-y-1.5">
            <legend className="text-xs text-muted-foreground">Address families</legend>
            <div className="flex gap-1.5">
              <Toggle pressed={ipv4} onClick={() => setIpv4((v) => !v)}>
                IPv4
              </Toggle>
              <Toggle pressed={ipv6} onClick={() => setIpv6((v) => !v)}>
                IPv6
              </Toggle>
            </div>
          </fieldset>

          <fieldset className="space-y-1.5">
            <legend className="flex w-full items-center justify-between text-xs text-muted-foreground">
              <span>
                IRR sources ({sources.length}/{IRR_SOURCES.length})
              </span>
            </legend>
            <div className="flex flex-wrap gap-1.5">
              {IRR_SOURCES.map((s) => (
                <Toggle key={s} pressed={sources.includes(s)} onClick={() => toggleSource(s)}>
                  {s}
                </Toggle>
              ))}
            </div>
            <div className="flex gap-3 text-xs">
              <button type="button" className="text-muted-foreground underline-offset-2 hover:text-foreground hover:underline" onClick={() => setSources(IRR_SOURCES)}>
                All
              </button>
              <button type="button" className="text-muted-foreground underline-offset-2 hover:text-foreground hover:underline" onClick={() => setSources([])}>
                None
              </button>
            </div>
          </fieldset>

          <Button type="submit" className="w-full" disabled={loading}>
            <Search aria-hidden="true" />
            {loading ? "Querying IRR…" : "Build prefix list"}
          </Button>
        </form>

        {/* 2. Review + 3. Commit */}
        <section aria-labelledby="pb-review-h" className="min-w-0 rounded-lg border bg-card">
          <div className="flex flex-wrap items-baseline gap-x-4 gap-y-1 border-b px-4 py-3">
            <h2 id="pb-review-h" className="text-xs text-muted-foreground">
              2 · Review
            </h2>
            {result && (
              <>
                <span className="font-mono text-base font-medium">{result.asn.toUpperCase()}</span>
                <span className="text-muted-foreground" aria-live="polite">
                  <span className="font-mono text-foreground">{selected.length.toLocaleString()}</span> of{" "}
                  {all.length.toLocaleString()} selected · {v4Count.toLocaleString()} IPv4
                  {v4Count > 0 && <> (≈{compact(v4Addresses(selected))} addresses)</>} ·{" "}
                  {v6Count.toLocaleString()} IPv6
                </span>
              </>
            )}
          </div>

          {!result && (
            <div className="px-4 py-10 text-center text-muted-foreground">
              {loading ? (
                <span aria-live="polite">Querying IRR for {asn.trim().toUpperCase()}…</span>
              ) : (
                <>
                  <p>Enter an ASN and build to see its registered prefixes here.</p>
                  <p className="text-xs">Nothing is added until you confirm in step 3.</p>
                </>
              )}
            </div>
          )}

          {result && all.length === 0 && (
            <p className="px-4 py-10 text-center text-muted-foreground">
              No route objects found for {result.asn.toUpperCase()} in the selected sources.
            </p>
          )}

          {result && all.length > 0 && (
            <>
              <div className="flex flex-wrap items-center gap-2 px-4 pt-3">
                <label htmlFor="pb-filter" className="sr-only">
                  Filter prefixes
                </label>
                <Input
                  id="pb-filter"
                  type="search"
                  placeholder="Filter, e.g. 103.10."
                  value={filter}
                  onChange={(e) => setFilter(e.target.value)}
                  className="w-56 font-mono"
                />
                <Button variant="ghost" size="sm" onClick={() => setShown(true)}>
                  Select {filter ? "shown" : "all"}
                </Button>
                <Button variant="ghost" size="sm" onClick={() => setShown(false)}>
                  Clear {filter ? "shown" : "all"}
                </Button>
                <Button variant="outline" size="sm" className="ml-auto" onClick={copy} disabled={!selected.length}>
                  {copied ? <Check aria-hidden="true" /> : <Copy aria-hidden="true" />}
                  {copied ? "Copied" : "Copy selected"}
                </Button>
              </div>

              <ul
                aria-label="Prefixes"
                className="m-4 max-h-[26rem] columns-[14rem] gap-x-4 overflow-y-auto rounded-md border bg-background p-2 font-mono text-xs"
              >
                {shown.slice(0, RENDER_CAP).map((p) => {
                  const on = !dropped.has(p);
                  return (
                    <li key={p} className="break-inside-avoid">
                      <label
                        className={cn(
                          "flex items-center gap-2 rounded-sm px-1.5 py-0.5 hover:bg-muted pointer-coarse:min-h-11",
                          !on && "text-muted-foreground line-through",
                        )}
                      >
                        <input
                          type="checkbox"
                          checked={on}
                          onChange={() =>
                            setDropped((prev) => {
                              const next = new Set(prev);
                              if (on) next.add(p);
                              else next.delete(p);
                              return next;
                            })
                          }
                          className="size-3.5 accent-[var(--signal)]"
                        />
                        {p}
                      </label>
                    </li>
                  );
                })}
                {shown.length > RENDER_CAP && (
                  <li className="px-1.5 py-2 text-muted-foreground">
                    Showing {RENDER_CAP.toLocaleString()} of {shown.length.toLocaleString()}. Filter to see the rest;
                    Select/Clear apply to all {shown.length.toLocaleString()}.
                  </li>
                )}
                {shown.length === 0 && (
                  <li className="px-1.5 py-2 text-muted-foreground">No prefixes match “{filter}”.</li>
                )}
              </ul>

              <div className="space-y-3 border-t px-4 py-3">
                <h3 className="text-xs text-muted-foreground">3 · Commit</h3>
                <div className="flex flex-wrap items-end gap-2">
                  <Field label="Note" htmlFor="pb-note" className="min-w-0 flex-1 basis-56">
                    <Input id="pb-note" value={note} onChange={(e) => setNote(e.target.value)} />
                  </Field>
                  <Confirm
                    title={`Blackhole ${selected.length.toLocaleString()} prefixes from ${result.asn.toUpperCase()}?`}
                    description={`They are announced to every downstream router on the next sync, and traffic to them is dropped. Covering excludes do not apply to manual prefixes.`}
                    confirmLabel={`Blackhole ${selected.length.toLocaleString()} prefixes`}
                    onConfirm={() => commit("prefixes")}
                  >
                    <Button variant="destructive" disabled={adding || selected.length === 0}>
                      <ShieldAlert aria-hidden="true" />
                      Blackhole {selected.length.toLocaleString()}
                    </Button>
                  </Confirm>
                  <Button variant="outline" disabled={adding || selected.length === 0} onClick={() => commit("excludes")}>
                    <Ban aria-hidden="true" />
                    Exclude {selected.length.toLocaleString()}
                  </Button>
                </div>
              </div>
            </>
          )}

          <div className="px-4 pb-3 empty:hidden">
            <ErrorNote>{error}</ErrorNote>
            {status && (
              <p role="status" className="text-signal">
                {status}
              </p>
            )}
          </div>
        </section>
      </div>
    </div>
  );
}
