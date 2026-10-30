import { useEffect, useRef, useState } from "react";
import { api, type LookupMatch, type LookupResult } from "@/lib/api";
import * as ipaddr from "ipaddr.js";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import { ErrorNote, PageHeader } from "@/components/app";
import { formatDate } from "@/lib/format";
import { cn } from "@/lib/utils";
import { Loader2, Search } from "lucide-react";

const MAX_QUERIES = 10;

function validIPOrCIDR(q: string) {
  try {
    if (q.includes("/")) {
      const [addr, bits] = q.split("/");
      if (!addr || !/^\d+$/.test(bits ?? "")) return false;
      const max = ipaddr.parse(addr).kind() === "ipv4" ? 32 : 128;
      return Number(bits) <= max;
    }
    ipaddr.parse(q);
    return true;
  } catch {
    return false;
  }
}

function bitsOf(cidr: string) {
  const b = cidr.split("/")[1];
  return b === undefined ? (cidr.includes(":") ? 128 : 32) : Number(b);
}

type Verdict = {
  tone: "blocked" | "partial" | "excluded" | "clear";
  label: string;
  detail: string;
};

// Mirrors the feed: manual prefixes always apply, source prefixes apply
// unless an exclude covers them (the API already drops those).
function verdictFor(r: LookupResult): Verdict {
  const qBits = bitsOf(r.query);
  const blocks = r.matches.filter((m) => m.type !== "exclude");
  const excludes = r.matches.filter((m) => m.type === "exclude");
  const covering = blocks.filter((m) => bitsOf(m.cidr) <= qBits);
  const who = (m: LookupMatch) =>
    m.type === "manual" || m.type === "prefix" ? "a manual prefix" : m.source_name || "a source";

  if (covering.length) {
    const more = covering.length > 1 ? ` and ${covering.length - 1} more` : "";
    return {
      tone: "blocked",
      label: "Blackholed",
      detail: `Traffic is dropped. Listed by ${who(covering[0])}${more}.`,
    };
  }
  if (blocks.length) {
    const many = blocks.length > 1;
    return {
      tone: "partial",
      label: "Partly blackholed",
      detail: `${blocks.length} more specific prefix${many ? "es" : ""} inside this range ${many ? "are" : "is"} blackholed.`,
    };
  }
  if (excludes.length) {
    const by =
      excludes.length > 1
        ? `${excludes.length} excludes`
        : excludes[0].source_name
          ? `exclude source ${excludes[0].source_name}`
          : "a manual exclude";
    return { tone: "excluded", label: "Not blackholed", detail: `Protected by ${by}; sources cannot blackhole it.` };
  }
  return { tone: "clear", label: "Not blackholed", detail: "No matches found in any prefix list or exclude." };
}

const toneClass: Record<Verdict["tone"], string> = {
  blocked: "text-destructive",
  partial: "text-warn",
  excluded: "text-signal",
  clear: "text-muted-foreground",
};

// "203.0.113.10" and "203.0.113.10/32" are the same thing to an operator.
function sameAsQuery(query: string, cidr: string) {
  return cidr === query || cidr === `${query}/${query.includes(":") ? 128 : 32}`;
}

const EVIDENCE_ROWS = 25;

function relation(query: string, cidr: string) {
  return bitsOf(cidr) <= bitsOf(query) ? "covers query" : "inside query";
}

export default function Lookup() {
  // /lookup?q=192.0.2.0/24 (linked from Excludes) prefills and runs once.
  const [input, setInput] = useState(() => new URLSearchParams(window.location.search).get("q") ?? "");
  const [results, setResults] = useState<LookupResult[] | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [expanded, setExpanded] = useState<Set<string>>(new Set());

  const queries = input
    .split(/[\s,;]+/)
    .map((q) => q.trim())
    .filter(Boolean);

  const handleSearch = async () => {
    setError(null);
    if (queries.length === 0) return setError("Please enter at least one IP address or CIDR");
    if (queries.length > MAX_QUERIES) return setError(`Maximum ${MAX_QUERIES} IP addresses or CIDRs allowed`);
    const invalid = queries.filter((q) => !validIPOrCIDR(q));
    if (invalid.length) return setError(`Invalid IP address or CIDR: ${invalid.join(", ")}`);

    setLoading(true);
    setResults(null);
    try {
      const response = await api.lookup(queries);
      setResults(response.results);
      setExpanded(new Set());
      setInput(""); // each query is shown in its result card
    } catch (err) {
      setError(err instanceof Error ? err.message : "An error occurred");
    } finally {
      setLoading(false);
    }
  };

  const autoRan = useRef(false);
  const searchRef = useRef(handleSearch);
  useEffect(() => {
    searchRef.current = handleSearch;
  });
  useEffect(() => {
    if (autoRan.current || !input) return;
    autoRan.current = true;
    searchRef.current();
    // eslint-disable-next-line react-hooks/exhaustive-deps -- run once on mount
  }, []);

  const verdicts = results?.map((r) => ({ r, v: verdictFor(r) })) ?? [];
  const blocked = verdicts.filter(({ v }) => v.tone === "blocked").length;
  const partial = verdicts.filter(({ v }) => v.tone === "partial").length;

  return (
    <div className="space-y-4">
      <PageHeader
        title="IP Lookup"
        description="Is this address blackholed right now, and why? Checks prefixes, sources and excludes the way the feed is built."
      />

      <form
        className="rounded-lg border bg-card p-3"
        onSubmit={(e) => {
          e.preventDefault();
          handleSearch();
        }}
      >
        <label htmlFor="lookup-input" className="text-xs text-muted-foreground">
          IP addresses or CIDRs
        </label>
        <div className="mt-1 flex flex-col gap-2 sm:flex-row sm:items-start">
          <Textarea
            id="lookup-input"
            placeholder="Enter IP addresses or CIDRs (one per line, or separated by spaces)"
            value={input}
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) {
                e.preventDefault();
                handleSearch();
              }
            }}
            rows={2}
            className="min-h-0 flex-1 resize-y font-mono"
            disabled={loading}
            spellCheck={false}
          />
          <Button type="submit" disabled={loading} className="sm:h-[3.75rem] sm:px-5">
            {loading ? (
              <>
                <Loader2 aria-hidden="true" className="animate-spin" />
                Searching…
              </>
            ) : (
              <>
                <Search aria-hidden="true" />
                Search
              </>
            )}
          </Button>
        </div>
        <p className="mt-1.5 text-xs text-muted-foreground">
          <span className={cn(queries.length > MAX_QUERIES && "text-destructive")}>
            {queries.length}/{MAX_QUERIES}
          </span>{" "}
          · Ctrl/⌘ + Enter to search
        </p>
      </form>

      <ErrorNote>{error}</ErrorNote>

      {results && (
        <section aria-label="Lookup results" className="space-y-3">
          {results.length > 1 && (
            <p className="text-muted-foreground" aria-live="polite">
              {results.length} checked · <span className={cn(blocked > 0 && "text-destructive")}>{blocked} blackholed</span>
              {partial > 0 && <span className="text-warn"> · {partial} partly</span>} ·{" "}
              {results.length - blocked - partial} not blackholed
            </p>
          )}

          {verdicts.map(({ r, v }) => (
            <article key={r.query} className="overflow-hidden rounded-lg border bg-card">
              <header className="flex flex-wrap items-baseline gap-x-4 gap-y-1 px-4 py-3">
                <h2 className="font-mono text-base font-medium">{r.query}</h2>
                <span className={cn("flex items-center gap-1.5 font-semibold", toneClass[v.tone])}>
                  <span
                    aria-hidden="true"
                    className={cn(
                      "size-2 rounded-full",
                      v.tone === "blocked" && "bg-destructive",
                      v.tone === "partial" && "bg-warn",
                      v.tone === "excluded" && "bg-signal",
                      v.tone === "clear" && "border border-current",
                    )}
                  />
                  {v.label}
                </span>
                <span className="text-muted-foreground">{v.detail}</span>
              </header>

              {r.matches.length > 0 && (
                <div className="border-t">
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead className="w-24">Kind</TableHead>
                        <TableHead>Listed by</TableHead>
                        <TableHead>Prefix</TableHead>
                        <TableHead>ASN</TableHead>
                        <TableHead>Note</TableHead>
                        <TableHead>Added</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {(expanded.has(r.query) ? r.matches : r.matches.slice(0, EVIDENCE_ROWS)).map((m, idx) => (
                        <TableRow key={`${m.type}-${m.cidr}-${idx}`}>
                          <TableCell>
                            <span
                              className={cn(
                                "rounded-sm border px-1.5 py-0.5 text-xs",
                                m.type === "exclude" ? "text-signal" : "text-destructive",
                              )}
                            >
                              {m.type}
                            </span>
                          </TableCell>
                          <TableCell>{m.source_name || (m.type === "exclude" ? "Manual exclude" : "Manual")}</TableCell>
                          <TableCell className="font-mono" data-cidr>
                            {sameAsQuery(r.query, m.cidr) ? (
                              <span className="font-sans text-muted-foreground">exact match</span>
                            ) : (
                              <>
                                <span>{m.cidr}</span>
                                <span className="ml-2 font-sans text-xs text-muted-foreground">
                                  {relation(r.query, m.cidr)}
                                </span>
                              </>
                            )}
                          </TableCell>
                          <TableCell>
                            {m.asn ? (
                              <div>
                                <div className="font-mono">{m.asn}</div>
                                {m.as_name && (
                                  <div className="max-w-[12rem] truncate text-xs text-muted-foreground" title={m.as_name}>
                                    {m.as_name}
                                  </div>
                                )}
                              </div>
                            ) : (
                              "-"
                            )}
                          </TableCell>
                          <TableCell className="max-w-[16rem] truncate" title={m.note || undefined}>
                            {m.note || "-"}
                          </TableCell>
                          <TableCell className="text-muted-foreground">{formatDate(m.created_at)}</TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                  {r.matches.length > EVIDENCE_ROWS && !expanded.has(r.query) && (
                    <div className="border-t px-4 py-2">
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => setExpanded((prev) => new Set(prev).add(r.query))}
                      >
                        Show all {r.matches.length} matches
                      </Button>
                    </div>
                  )}
                </div>
              )}
            </article>
          ))}
        </section>
      )}
    </div>
  );
}
