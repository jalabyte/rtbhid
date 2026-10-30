import { useState, type ChangeEvent } from "react";
import { api, type Peer } from "@/lib/api";
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
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { Plus, Power, Trash2, Upload } from "lucide-react";

const emptyForm = { name: "", neighbor_ip: "", remote_as: "", multihop: "0" };

export default function Peers() {
  const peers = useResource(api.getPeers);
  const status = useResource(api.getStatus);
  const action = useAction(peers.reload);
  const [form, setForm] = useState(emptyForm);
  const [bulkText, setBulkText] = useState("");
  const [showBulk, setShowBulk] = useState(false);
  const [bulkNote, setBulkNote] = useState("");
  const items: Peer[] = peers.data?.items ?? [];

  const set = (k: keyof typeof emptyForm) => (e: ChangeEvent<HTMLInputElement>) =>
    setForm((f) => ({ ...f, [k]: e.target.value }));

  const handleAdd = async () => {
    if (!form.name.trim() || !form.neighbor_ip.trim())
      return action.setError("Name and neighbor IP are required");
    const ok = await action.run(() =>
      api.addPeer({
        name: form.name.trim(),
        neighbor_ip: form.neighbor_ip.trim(),
        remote_as: parseInt(form.remote_as, 10) || 0,
        local_as: 0, // use the global ASN
        multihop: parseInt(form.multihop, 10) || 0,
        enabled: true,
        std_communities: [],
        large_communities: [],
      }),
    );
    if (ok) setForm(emptyForm);
  };

  const handleBulkAdd = async () => {
    const lines = bulkText.trim().split("\n").filter((l) => l.trim());
    const items: Parameters<typeof api.bulkAddPeers>[0] = [];
    const bad: number[] = [];
    lines.forEach((line, i) => {
      const parts = line.split(",").map((p) => p.trim());
      if (parts.length < 3 || !parts[1]) return bad.push(i + 1);
      items.push({
        name: parts[0],
        neighbor_ip: parts[1],
        remote_as: parseInt(parts[2], 10) || 0,
        multihop: parseInt(parts[3] || "0", 10) || 0,
        enabled: true,
        std_communities: [],
        large_communities: [],
      });
    });
    if (bad.length) return action.setError(`Line ${bad.join(", ")}: expected name,neighbor_ip,remote_as[,multihop]`);
    if (!items.length) return;
    let created = 0;
    const ok = await action.run(async () => {
      created = (await api.bulkAddPeers(items)).created;
    });
    if (ok) {
      setBulkNote(`Added ${created} of ${items.length} peers.`);
      setBulkText("");
      setShowBulk(false);
    }
  };

  return (
    <div className="space-y-4">
      <PageHeader
        title="Peers"
        description="Static BGP neighbors. Whitelisted ASNs can also connect without being listed here."
      >
        {status.data && (
          <span className="rounded-md border px-2 py-1 text-xs">
            Local ASN <span className="font-mono font-medium">AS{status.data.local_asn}</span>
          </span>
        )}
      </PageHeader>

      <FormRow onSubmit={handleAdd}>
        <Field label="Name" htmlFor="peer-name">
          <Input id="peer-name" value={form.name} onChange={set("name")} className="w-36" />
        </Field>
        <Field label="Neighbor IP" htmlFor="peer-ip">
          <Input
            id="peer-ip"
            placeholder="198.51.100.2"
            value={form.neighbor_ip}
            onChange={set("neighbor_ip")}
            className="w-44 font-mono"
          />
        </Field>
        <Field label="Remote AS" htmlFor="peer-as">
          <Input id="peer-as" type="number" min={1} value={form.remote_as} onChange={set("remote_as")} className="w-28" />
        </Field>
        <Field label="Multihop" htmlFor="peer-hop">
          <Input id="peer-hop" type="number" min={0} value={form.multihop} onChange={set("multihop")} className="w-20" />
        </Field>
        <Button type="submit" disabled={action.busy}>
          <Plus aria-hidden="true" />
          Add peer
        </Button>
        <Button
          type="button"
          variant="outline"
          aria-expanded={showBulk}
          aria-controls="bulk-peers"
          onClick={() => setShowBulk((v) => !v)}
        >
          <Upload aria-hidden="true" />
          Bulk add
        </Button>
      </FormRow>
      <ErrorNote>{action.error}</ErrorNote>
      {bulkNote && <p className="text-muted-foreground">{bulkNote}</p>}

      {showBulk && (
        <div id="bulk-peers" className="space-y-2 rounded-lg border bg-card p-4">
          <Label htmlFor="bulk-text">
            One peer per line: <code className="font-mono">name,neighbor_ip,remote_as[,multihop]</code>
          </Label>
          <Textarea
            id="bulk-text"
            className="min-h-[120px] font-mono"
            placeholder={"ix-a,198.51.100.2,64512,1\nix-b,203.0.113.3,64513,1"}
            value={bulkText}
            onChange={(e) => setBulkText(e.target.value)}
          />
          <div className="flex gap-2">
            <Button onClick={handleBulkAdd} disabled={!bulkText.trim() || action.busy}>
              Add peers
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
        </div>
      )}

      <div className="rounded-lg border bg-card">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Name</TableHead>
              <TableHead>Neighbor</TableHead>
              <TableHead>Remote AS</TableHead>
              <TableHead>Multihop</TableHead>
              <TableHead>Status</TableHead>
              <TableHead className="w-20 text-right">Actions</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {items.map((p) => (
              <TableRow key={p.id}>
                <TableCell className="font-medium">{p.name}</TableCell>
                <TableCell className="font-mono">{p.neighbor_ip}</TableCell>
                <TableCell className="font-mono">AS{p.remote_as}</TableCell>
                <TableCell>{p.multihop}</TableCell>
                <TableCell>
                  <StatusBadge on={p.enabled} />
                </TableCell>
                <TableCell>
                  <div className="flex justify-end gap-1">
                    <IconButton
                      icon={Power}
                      label={`${p.enabled ? "Disable" : "Enable"} peer ${p.name}`}
                      aria-pressed={p.enabled}
                      className={p.enabled ? "text-signal" : "text-muted-foreground"}
                      disabled={action.busy}
                      onClick={() => action.run(() => api.togglePeer(p.id, !p.enabled))}
                    />
                    <Confirm
                      title={`Delete peer ${p.name}?`}
                      description={`The BGP session to ${p.neighbor_ip} is closed and the router stops receiving blackhole routes.`}
                      confirmLabel="Delete peer"
                      onConfirm={() => action.run(() => api.deletePeer(p.id))}
                    >
                      <IconButton icon={Trash2} label={`Delete peer ${p.name}`} />
                    </Confirm>
                  </div>
                </TableCell>
              </TableRow>
            ))}
            <TableState
              colSpan={6}
              loading={peers.loading}
              error={peers.error}
              empty={items.length === 0}
              emptyText="No static peers."
              onRetry={peers.reload}
            />
          </TableBody>
        </Table>
      </div>
    </div>
  );
}
