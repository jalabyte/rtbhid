import { useEffect, useMemo, useState } from "react";
import { api, type WhitelistASN } from "@/lib/api";
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
import { Download, Plus, Power, Search, Trash2 } from "lucide-react";

const PAGE_SIZE = 100;

export default function Whitelist() {
  const [items, setItems] = useState<WhitelistASN[]>([]);
  const [autoAccept, setAutoAccept] = useState(true);
  const [form, setForm] = useState({ asn: "", name: "" });
  const [query, setQuery] = useState("");
  const [page, setPage] = useState(0);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState("");

  const load = () => {
    api
      .getWhitelist()
      .then((r) => {
        setItems(r?.items ?? []);
        setAutoAccept(r?.auto_accept ?? true);
      })
      .catch(() => setItems([]));
  };

  useEffect(() => {
    load();
  }, []);

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
    if (!asn) return;
    setBusy(true);
    setMessage("");
    try {
      await api.addWhitelistASN({ asn, name: form.name.trim() });
      setForm({ asn: "", name: "" });
      load();
    } catch (err) {
      setMessage(err instanceof Error ? err.message : "Failed to add ASN");
    } finally {
      setBusy(false);
    }
  };

  const handleImport = async () => {
    setBusy(true);
    setMessage("");
    try {
      const r = await api.importIndonesianASNs();
      setMessage(
        `Imported ${r.created} Indonesian ASNs from APNIC (${r.total} found).`,
      );
      load();
    } catch (err) {
      setMessage(err instanceof Error ? err.message : "Import failed");
    } finally {
      setBusy(false);
    }
  };

  const handleToggle = async (id: string, enabled: boolean) => {
    await api.toggleWhitelistASN(id, enabled);
    load();
  };

  const handleDelete = async (id: string) => {
    await api.deleteWhitelistASN(id);
    load();
  };

  const handleAutoAccept = async () => {
    await api.setAutoAccept(!autoAccept);
    load();
  };

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-4 flex-wrap">
        <h2 className="text-2xl font-bold">ASN Whitelist</h2>
        <Badge variant="outline" className="text-sm px-3 py-1">
          {items.length} ASNs
        </Badge>
        <Badge variant={autoAccept ? "default" : "secondary"}>
          Auto-accept {autoAccept ? "on" : "off"}
        </Badge>
        <Button variant="outline" size="sm" onClick={handleAutoAccept}>
          <Power className="h-4 w-4 mr-1" />
          Turn {autoAccept ? "off" : "on"}
        </Button>
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">Auto-accepted BGP peers</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <p className="text-sm text-muted-foreground">
            Agents running the embedded BGP server accept inbound peering
            sessions only from enabled ASNs in this list. Use "Import
            Indonesian ASNs" to load the current APNIC list (country ID), then
            add or remove individual ASNs manually.
          </p>

          <div className="flex gap-2 flex-wrap">
            <Input
              placeholder="ASN (e.g. 7713)"
              value={form.asn}
              onChange={(e) =>
                setForm((f) => ({ ...f, asn: e.target.value }))
              }
              className="max-w-[140px]"
              type="number"
            />
            <Input
              placeholder="Name (optional)"
              value={form.name}
              onChange={(e) =>
                setForm((f) => ({ ...f, name: e.target.value }))
              }
              className="max-w-[220px]"
            />
            <Button onClick={handleAdd} disabled={busy}>
              <Plus className="h-4 w-4 mr-1" />
              Add ASN
            </Button>
            <Button variant="outline" onClick={handleImport} disabled={busy}>
              <Download className="h-4 w-4 mr-1" />
              Import Indonesian ASNs
            </Button>
          </div>

          {message && (
            <p className="text-sm text-muted-foreground">{message}</p>
          )}

          <div className="flex items-center gap-2">
            <Search className="h-4 w-4 text-muted-foreground" />
            <Input
              placeholder="Filter by ASN or name"
              value={query}
              onChange={(e) => {
                setQuery(e.target.value);
                setPage(0);
              }}
              className="max-w-[280px]"
            />
          </div>

          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>ASN</TableHead>
                <TableHead>Name</TableHead>
                <TableHead>Country</TableHead>
                <TableHead>Source</TableHead>
                <TableHead>Status</TableHead>
                <TableHead className="text-right">Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {visible.map((item) => (
                <TableRow key={item.id}>
                  <TableCell className="font-mono">AS{item.asn}</TableCell>
                  <TableCell>{item.name || "-"}</TableCell>
                  <TableCell>{item.country || "-"}</TableCell>
                  <TableCell>
                    <Badge variant="outline">{item.source || "manual"}</Badge>
                  </TableCell>
                  <TableCell>
                    <Badge variant={item.enabled ? "default" : "secondary"}>
                      {item.enabled ? "enabled" : "disabled"}
                    </Badge>
                  </TableCell>
                  <TableCell className="text-right">
                    <div className="flex justify-end gap-1">
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => handleToggle(item.id, !item.enabled)}
                      >
                        <Power className="h-4 w-4" />
                      </Button>
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => handleDelete(item.id)}
                      >
                        <Trash2 className="h-4 w-4 text-destructive" />
                      </Button>
                    </div>
                  </TableCell>
                </TableRow>
              ))}
              {visible.length === 0 && (
                <TableRow>
                  <TableCell
                    colSpan={6}
                    className="text-center text-muted-foreground py-8"
                  >
                    No whitelisted ASNs. Import the Indonesian list or add one
                    manually.
                  </TableCell>
                </TableRow>
              )}
            </TableBody>
          </Table>

          {pageCount > 1 && (
            <div className="flex items-center justify-between text-sm">
              <span className="text-muted-foreground">
                Page {page + 1} of {pageCount}
              </span>
              <div className="flex gap-2">
                <Button
                  variant="outline"
                  size="sm"
                  disabled={page === 0}
                  onClick={() => setPage((p) => Math.max(0, p - 1))}
                >
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
              </div>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}