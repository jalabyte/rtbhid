import { useEffect, useState } from "react";
import { api, type Peer, type Status } from "@/lib/api";
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
import { Trash2, Plus, Upload } from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";

export default function Peers() {
  const [peers, setPeers] = useState<Peer[]>([]);
  const [status, setStatus] = useState<Status | null>(null);
  const [form, setForm] = useState({
    name: "",
    neighbor_ip: "",
    remote_as: "",
    multihop: "0",
  });
  const [bulkText, setBulkText] = useState("");
  const [showBulk, setShowBulk] = useState(false);

  const load = () => {
    api
      .getPeers()
      .then((r) => setPeers(r?.items ?? []))
      .catch(() => setPeers([]));
    api
      .getStatus()
      .then(setStatus)
      .catch(() => setStatus(null));
  };

  useEffect(() => {
    load();
  }, []);

  const handleAdd = async () => {
    if (!form.name.trim() || !form.neighbor_ip.trim()) return;
    await api.addPeer({
      name: form.name.trim(),
      neighbor_ip: form.neighbor_ip.trim(),
      remote_as: parseInt(form.remote_as, 10) || 0,
      local_as: 0, // Use global ASN
      multihop: parseInt(form.multihop, 10) || 0,
      enabled: true,
      std_communities: [],
      large_communities: [],
    });
    setForm({ name: "", neighbor_ip: "", remote_as: "", multihop: "0" });
    load();
  };

  const handleBulkAdd = async () => {
    if (!bulkText.trim()) return;
    const lines = bulkText.trim().split("\n");
    const items = lines
      .map((line) => {
        const parts = line.split(",").map((p) => p.trim());
        if (parts.length >= 3) {
          return {
            name: parts[0],
            neighbor_ip: parts[1],
            remote_as: parseInt(parts[2], 10) || 0,
            multihop: parseInt(parts[3] || "0", 10) || 0,
            enabled: true,
            std_communities: [] as string[],
            large_communities: [] as string[],
          };
        }
        return null;
      })
      .filter((item): item is NonNullable<typeof item> => item !== null);

    if (items.length > 0) {
      await api.bulkAddPeers(items);
    }
    setBulkText("");
    setShowBulk(false);
    load();
  };

  const handleDelete = async (id: string) => {
    await api.deletePeer(id);
    load();
  };

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-4">
        <h2 className="text-2xl font-bold">Peers</h2>
        {status && (
          <Badge variant="outline" className="text-sm px-3 py-1">
            Local ASN:{" "}
            <span className="font-bold ml-1">{status.local_asn}</span>
          </Badge>
        )}
      </div>

      <div className="space-y-4">
        <div className="flex gap-2 flex-wrap">
          <Input
            placeholder="Name"
            value={form.name}
            onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))}
            className="max-w-[140px]"
          />
          <Input
            placeholder="Neighbor IP"
            value={form.neighbor_ip}
            onChange={(e) =>
              setForm((f) => ({ ...f, neighbor_ip: e.target.value }))
            }
            className="max-w-[160px]"
          />
          <Input
            placeholder="Remote AS"
            value={form.remote_as}
            onChange={(e) =>
              setForm((f) => ({ ...f, remote_as: e.target.value }))
            }
            className="max-w-[110px]"
            type="number"
          />
          <Input
            placeholder="Multihop"
            value={form.multihop}
            onChange={(e) =>
              setForm((f) => ({ ...f, multihop: e.target.value }))
            }
            className="max-w-[100px]"
            type="number"
          />
          <Button onClick={handleAdd}>
            <Plus className="h-4 w-4 mr-1" />
            Add
          </Button>
          <Button variant="outline" onClick={() => setShowBulk(!showBulk)}>
            <Upload className="h-4 w-4 mr-1" />
            Bulk Add
          </Button>
        </div>

        {showBulk && (
          <Card>
            <CardHeader>
              <CardTitle className="text-sm">Bulk Add Peers</CardTitle>
              <p className="text-xs text-muted-foreground">
                Format per line:{" "}
                <code>name,neighbor_ip,remote_as[,multihop]</code>
              </p>
            </CardHeader>
            <CardContent className="space-y-2">
              <textarea
                className="w-full min-h-[120px] p-2 border rounded-md font-mono text-sm"
                placeholder="ix-a,198.51.100.2,64512,1&#10;ix-b,203.0.113.3,64513,1"
                value={bulkText}
                onChange={(e) => setBulkText(e.target.value)}
              />
              <div className="flex gap-2">
                <Button onClick={handleBulkAdd} disabled={!bulkText.trim()}>
                  <Upload className="h-4 w-4 mr-1" />
                  Add Peers
                </Button>
                <Button
                  variant="outline"
                  onClick={() => {
                    setShowBulk(false);
                    setBulkText("");
                  }}
                >
                  Cancel
                </Button>
              </div>
            </CardContent>
          </Card>
        )}
      </div>

      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Name</TableHead>
            <TableHead>Neighbor</TableHead>
            <TableHead>Remote AS</TableHead>
            <TableHead>Multihop</TableHead>
            <TableHead>Status</TableHead>
            <TableHead className="w-16"></TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {peers.map((p) => (
            <TableRow key={p.id}>
              <TableCell className="font-medium">{p.name}</TableCell>
              <TableCell className="font-mono">{p.neighbor_ip}</TableCell>
              <TableCell>{p.remote_as}</TableCell>
              <TableCell>{p.multihop}</TableCell>
              <TableCell>
                <Badge variant={p.enabled ? "default" : "secondary"}>
                  {p.enabled ? "enabled" : "disabled"}
                </Badge>
              </TableCell>
              <TableCell>
                <Button
                  variant="ghost"
                  size="icon"
                  onClick={() => handleDelete(p.id)}
                >
                  <Trash2 className="h-4 w-4" />
                </Button>
              </TableCell>
            </TableRow>
          ))}
          {peers.length === 0 && (
            <TableRow>
              <TableCell
                colSpan={6}
                className="text-center text-muted-foreground"
              >
                No peers
              </TableCell>
            </TableRow>
          )}
        </TableBody>
      </Table>
    </div>
  );
}
