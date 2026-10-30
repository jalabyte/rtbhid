import { useState } from "react";
import { api, type Prefix } from "@/lib/api";
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
import { formatDate } from "@/lib/format";
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
import { Plus, Power, Trash2 } from "lucide-react";

export default function Prefixes() {
  const { data, error, loading, reload } = useResource(api.getPrefixes);
  const action = useAction(reload);
  const [cidr, setCidr] = useState("");
  const [note, setNote] = useState("");
  const prefixes: Prefix[] = data?.items ?? [];

  const handleAdd = async () => {
    if (!cidr.trim()) return action.setError("CIDR is required");
    const ok = await action.run(() =>
      api.addPrefix({ cidr: cidr.trim(), note: note.trim() }),
    );
    if (ok) {
      setCidr("");
      setNote("");
    }
  };

  return (
    <div className="space-y-4">
      <PageHeader
        title="Prefixes"
        description="Manually blackholed prefixes. Excludes do not apply to these."
      />

      <FormRow onSubmit={handleAdd}>
        <Field label="CIDR or IP" htmlFor="pfx-cidr">
          <Input
            id="pfx-cidr"
            placeholder="192.0.2.0/24"
            value={cidr}
            onChange={(e) => setCidr(e.target.value)}
            className="w-56 font-mono"
          />
        </Field>
        <Field label="Note (optional)" htmlFor="pfx-note">
          <Input
            id="pfx-note"
            value={note}
            onChange={(e) => setNote(e.target.value)}
            className="w-64"
          />
        </Field>
        <Button type="submit" disabled={action.busy}>
          <Plus aria-hidden="true" />
          Add prefix
        </Button>
      </FormRow>
      <ErrorNote>{action.error}</ErrorNote>

      <div className="rounded-lg border bg-card">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>CIDR</TableHead>
              <TableHead>Status</TableHead>
              <TableHead>Note</TableHead>
              <TableHead>Created</TableHead>
              <TableHead className="w-20 text-right">Actions</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {prefixes.map((p) => (
              <TableRow key={p.id}>
                <TableCell className="font-mono">{p.cidr}</TableCell>
                <TableCell>
                  <StatusBadge on={p.enabled} />
                </TableCell>
                <TableCell className="max-w-xs truncate">{p.note || "-"}</TableCell>
                <TableCell>{formatDate(p.created_at)}</TableCell>
                <TableCell>
                  <div className="flex justify-end gap-1">
                    <IconButton
                      icon={Power}
                      label={`${p.enabled ? "Disable" : "Enable"} ${p.cidr}`}
                      aria-pressed={p.enabled}
                      className={p.enabled ? "text-signal" : "text-muted-foreground"}
                      disabled={action.busy}
                      onClick={() => action.run(() => api.togglePrefix(p.id, !p.enabled))}
                    />
                    <Confirm
                      title={`Delete ${p.cidr}?`}
                      description="It is withdrawn from all routers on the next sync."
                      confirmLabel="Delete prefix"
                      onConfirm={() => action.run(() => api.deletePrefix(p.id))}
                    >
                      <IconButton icon={Trash2} label={`Delete ${p.cidr}`} />
                    </Confirm>
                  </div>
                </TableCell>
              </TableRow>
            ))}
            <TableState
              colSpan={5}
              loading={loading}
              error={error}
              empty={prefixes.length === 0}
              emptyText="No manual prefixes."
              onRetry={reload}
            />
          </TableBody>
        </Table>
      </div>
    </div>
  );
}
