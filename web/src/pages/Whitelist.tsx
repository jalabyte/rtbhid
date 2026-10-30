import { useMemo, useState } from "react";
import { api, type WhitelistASN } from "@/lib/api";
import { useAction, useResource } from "@/hooks/use-api";
import {
  Confirm,
  ErrorNote,
  Field,
  FormRow,
  IconButton,
  PageHeader,
  StatusBadge,
  TableState,
} from "@/components/app";
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
import { Download, Plus, Power, Trash2 } from "lucide-react";

const PAGE_SIZE = 100;

export default function Whitelist() {
  const { data, error, loading, reload } = useResource(api.getWhitelist);
  const action = useAction(reload);
  const [allowNonID, setAllowNonID] = useState(false);
  const [form, setForm] = useState({ asn: "", name: "" });
  const [query, setQuery] = useState("");
  const [page, setPage] = useState(0);
  const [message, setMessage] = useState("");

  const items: WhitelistASN[] = useMemo(() => data?.items ?? [], [data]);
  const autoAccept = data?.auto_accept ?? true;
  const strictID = data?.strict_id ?? true;
  const registryCount = data?.id_registry_count ?? 0;

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return items;
    return items.filter(
      (item) =>
        String(item.asn).includes(q) ||
        item.name.toLowerCase().includes(q) ||
        item.source.toLowerCase().includes(q),
    );
  }, [items, query]);

  const pageCount = Math.max(1, Math.ceil(filtered.length / PAGE_SIZE));
  const visible = filtered.slice(page * PAGE_SIZE, (page + 1) * PAGE_SIZE);

  const handleAdd = async () => {
    const asn = parseInt(form.asn, 10);
    if (!asn) return action.setError("Enter an ASN number");
    setMessage("");
    const ok = await action.run(() =>
      api.addWhitelistASN({ asn, name: form.name.trim(), allow_non_id: allowNonID }),
    );
    if (ok) setForm({ asn: "", name: "" });
  };

  const handleImport = () => {
    setMessage("");
    return action.run(async () => {
      const r = await api.importIndonesianASNs();
      setMessage(
        `Imported ${r.created} Indonesian ASNs from RIR data (${r.total} found, ${r.id_registry_count} cached).` +
          (r.warnings?.length
            ? ` ${r.warnings.length} source(s) failed: ${r.warnings.join("; ")}`
            : ""),
      );
    });
  };

  const toggleAutoAccept = () => action.run(() => api.setAutoAccept(!autoAccept));

  return (
    <div className="space-y-4">
      <PageHeader
        title="ASN Whitelist"
        description="Inbound BGP sessions are accepted only from enabled ASNs in this list."
      >
        <span className="rounded-md border px-2 py-1 text-xs">{items.length} ASNs</span>
        <span className="rounded-md border px-2 py-1 text-xs">{registryCount} ID ASNs cached</span>
        {strictID && <span className="rounded-md border px-2 py-1 text-xs">strict Indonesian only</span>}
      </PageHeader>

      <section
        aria-label="Auto-accept"
        className="flex flex-wrap items-center gap-3 rounded-lg border bg-card px-4 py-3"
      >
        <StatusBadge on={autoAccept} onLabel="Auto-accept on" offLabel="Auto-accept off" />
        <p className="min-w-0 flex-1 text-muted-foreground">
          {autoAccept
            ? "Routers from whitelisted ASNs can peer without a static peer entry."
            : "Only static peers can connect. Whitelisted ASNs are rejected."}
        </p>
        {autoAccept ? (
          <Confirm
            title="Turn off auto-accept?"
            description="Every inbound session that is not a static peer is rejected, and those routers stop receiving blackhole routes."
            confirmLabel="Turn off auto-accept"
            onConfirm={toggleAutoAccept}
          >
            <Button variant="outline" size="sm" disabled={action.busy}>
              <Power aria-hidden="true" />
              Turn off
            </Button>
          </Confirm>
        ) : (
          <Button variant="outline" size="sm" onClick={toggleAutoAccept} disabled={action.busy}>
            <Power aria-hidden="true" />
            Turn on
          </Button>
        )}
      </section>

      <FormRow onSubmit={handleAdd}>
        <Field label="ASN" htmlFor="wl-asn">
          <Input
            id="wl-asn"
            placeholder="ASN (e.g. 7713)"
            value={form.asn}
            onChange={(e) => setForm((f) => ({ ...f, asn: e.target.value }))}
            className="w-44 font-mono"
            type="number"
            min={1}
          />
        </Field>
        <Field label="Name" htmlFor="wl-name">
          <Input
            id="wl-name"
            placeholder="Name (optional)"
            value={form.name}
            onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))}
            className="w-56"
          />
        </Field>
        <Button type="submit" disabled={action.busy}>
          <Plus aria-hidden="true" />
          Add ASN
        </Button>
        <label className="flex h-8 items-center gap-2 px-1 text-muted-foreground pointer-coarse:min-h-11">
          <input
            type="checkbox"
            checked={allowNonID}
            onChange={(e) => setAllowNonID(e.target.checked)}
            className="size-4 accent-[var(--signal)]"
          />
          Allow non-Indonesian ASN
        </label>
        <Button type="button" variant="outline" className="ml-auto" onClick={handleImport} disabled={action.busy}>
          <Download aria-hidden="true" />
          Import Indonesian ASNs
        </Button>
      </FormRow>
      <ErrorNote>{action.error}</ErrorNote>
      {message && <p className="text-muted-foreground">{message}</p>}

      <div className="flex flex-wrap items-end justify-between gap-2">
        <Field label="Filter" htmlFor="wl-filter">
          <Input
            id="wl-filter"
            type="search"
            placeholder="ASN, name or source"
            value={query}
            onChange={(e) => {
              setQuery(e.target.value);
              setPage(0);
            }}
            className="w-64"
          />
        </Field>
        {pageCount > 1 && (
          <nav aria-label="Pagination" className="flex items-center gap-2">
            <span className="text-muted-foreground">
              Page {page + 1} of {pageCount}
            </span>
            <Button variant="outline" size="sm" disabled={page === 0} onClick={() => setPage((p) => Math.max(0, p - 1))}>
              Previous
            </Button>
            <Button
              variant="outline"
              size="sm"
              disabled={page >= pageCount - 1}
              onClick={() => setPage((p) => Math.min(pageCount - 1, p + 1))}
            >
              Next
            </Button>
          </nav>
        )}
      </div>

      <div className="rounded-lg border bg-card">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>ASN</TableHead>
              <TableHead>Name</TableHead>
              <TableHead>Country</TableHead>
              <TableHead>Source</TableHead>
              <TableHead>Status</TableHead>
              <TableHead className="w-20 text-right">Actions</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {visible.map((item) => (
              <TableRow key={item.id}>
                <TableCell className="font-mono">AS{item.asn}</TableCell>
                <TableCell className="max-w-sm truncate">{item.name || "-"}</TableCell>
                <TableCell>{item.country || "-"}</TableCell>
                <TableCell className="text-muted-foreground">{item.source || "manual"}</TableCell>
                <TableCell>
                  <StatusBadge on={item.enabled} />
                </TableCell>
                <TableCell>
                  <div className="flex justify-end gap-1">
                    <IconButton
                      icon={Power}
                      label={`${item.enabled ? "Disable" : "Enable"} AS ${item.asn}`}
                      aria-pressed={item.enabled}
                      className={item.enabled ? "text-signal" : "text-muted-foreground"}
                      disabled={action.busy}
                      onClick={() => action.run(() => api.toggleWhitelistASN(item.id, !item.enabled))}
                    />
                    <Confirm
                      title={`Remove AS ${item.asn} from the whitelist?`}
                      description="Its routers can no longer open new inbound sessions. Re-importing restores RIR entries."
                      confirmLabel="Remove ASN"
                      onConfirm={() => action.run(() => api.deleteWhitelistASN(item.id))}
                    >
                      <IconButton icon={Trash2} label={`Remove AS ${item.asn}`} />
                    </Confirm>
                  </div>
                </TableCell>
              </TableRow>
            ))}
            <TableState
              colSpan={6}
              loading={loading}
              error={error}
              empty={visible.length === 0}
              emptyText={
                query
                  ? "No ASNs match the filter."
                  : "No whitelisted ASNs. Import the Indonesian list or add one manually."
              }
              onRetry={reload}
            />
          </TableBody>
        </Table>
      </div>
    </div>
  );
}
