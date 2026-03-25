import { useEffect, useState } from "react";
import { api, type Source } from "@/lib/api";
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
import { Trash2, Plus, RefreshCw, Power } from "lucide-react";

export default function Sources() {
  const [sources, setSources] = useState<Source[]>([]);
  const [form, setForm] = useState({ name: "", url: "", interval: "300" });

  const load = () =>
    api
      .getSources()
      .then((r) => setSources(r?.items ?? []))
      .catch(() => setSources([]));

  useEffect(() => {
    load();
  }, []);

  const handleAdd = async () => {
    if (!form.url.trim()) return;
    await api.addSource({
      name: form.name.trim(),
      url: form.url.trim(),
      interval: parseInt(form.interval, 10) || 300,
    });
    setForm({ name: "", url: "", interval: "300" });
    load();
  };

  const handleDelete = async (id: string) => {
    await api.deleteSource(id);
    load();
  };

  const handleSync = async (id: string) => {
    await api.syncSource(id);
    load();
  };

  const handleToggle = async (id: string, enabled: boolean) => {
    await api.toggleSource(id, enabled);
    load();
  };

  return (
    <div className="space-y-6">
      <h2 className="text-2xl font-bold">Sources</h2>

      <div className="flex gap-2 flex-wrap">
        <Input
          placeholder="Name"
          value={form.name}
          onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))}
          className="max-w-[160px]"
        />
        <Input
          placeholder="URL"
          value={form.url}
          onChange={(e) => setForm((f) => ({ ...f, url: e.target.value }))}
          className="max-w-xs"
        />
        <Input
          placeholder="Interval (s)"
          value={form.interval}
          onChange={(e) => setForm((f) => ({ ...f, interval: e.target.value }))}
          className="max-w-[120px]"
        />
        <Button onClick={handleAdd}>
          <Plus className="h-4 w-4 mr-1" />
          Add
        </Button>
      </div>

      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Name</TableHead>
            <TableHead>URL</TableHead>
            <TableHead>Interval</TableHead>
            <TableHead>Status</TableHead>
            <TableHead>Last Sync</TableHead>
            <TableHead>Count</TableHead>
            <TableHead className="w-32"></TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {sources.map((s) => (
            <TableRow key={s.id}>
              <TableCell className="font-medium">{s.name || s.id}</TableCell>
              <TableCell className="max-w-[200px] truncate">{s.url}</TableCell>
              <TableCell>{s.interval}s</TableCell>
              <TableCell>
                <Badge variant={s.enabled ? "default" : "secondary"}>
                  {s.enabled ? "active" : "inactive"}
                </Badge>
              </TableCell>
              <TableCell>
                {s.last_sync ? new Date(s.last_sync).toLocaleString() : "—"}
              </TableCell>
              <TableCell>{s.last_count}</TableCell>
              <TableCell className="flex gap-1">
                <Button
                  variant="ghost"
                  size="icon"
                  onClick={() => handleToggle(s.id, !s.enabled)}
                  title={s.enabled ? "Deactivate" : "Activate"}
                >
                  <Power
                    className={`h-4 w-4 ${s.enabled ? "text-green-600" : "text-gray-400"}`}
                  />
                </Button>
                <Button
                  variant="ghost"
                  size="icon"
                  onClick={() => handleSync(s.id)}
                  title="Sync"
                >
                  <RefreshCw className="h-4 w-4" />
                </Button>
                <Button
                  variant="ghost"
                  size="icon"
                  onClick={() => handleDelete(s.id)}
                  title="Delete"
                >
                  <Trash2 className="h-4 w-4" />
                </Button>
              </TableCell>
            </TableRow>
          ))}
          {sources.length === 0 && (
            <TableRow>
              <TableCell
                colSpan={7}
                className="text-center text-muted-foreground"
              >
                No sources
              </TableCell>
            </TableRow>
          )}
        </TableBody>
      </Table>
    </div>
  );
}
