import { useState } from "react";
import { api, type Agent, type AgentToken } from "@/lib/api";
import { useAction, useResource } from "@/hooks/use-api";
import {
  Confirm,
  ErrorNote,
  Field,
  IconButton,
  PageHeader,
  StatusBadge,
  TableState,
} from "@/components/app";
import { agentOnline, formatDate, relativeTime } from "@/lib/format";
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
import { Check, Copy, Plus, Trash2, X } from "lucide-react";

// The installer only accepts these characters in --name.
const NAME_RE = /^[A-Za-z0-9._-]+$/;

function InstallCommand({ command, name, onDone }: { command: string; name: string; onDone: () => void }) {
  const [copied, setCopied] = useState(false);
  const [copyError, setCopyError] = useState("");
  const copy = async () => {
    try {
      await navigator.clipboard.writeText(command);
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    } catch {
      setCopyError("Clipboard is not available; select the command and copy it.");
    }
  };
  return (
    <section aria-labelledby="install-h" className="space-y-2 rounded-lg border border-signal/40 bg-card p-3">
      <div className="flex flex-wrap items-center gap-2">
        <h2 id="install-h" className="mr-auto font-semibold">
          Install command for {name}
        </h2>
        <Button size="sm" variant="outline" onClick={copy}>
          {copied ? <Check aria-hidden="true" /> : <Copy aria-hidden="true" />}
          {copied ? "Copied" : "Copy"}
        </Button>
        <Button size="sm" variant="ghost" onClick={onDone}>
          Done
        </Button>
      </div>
      <p className="text-muted-foreground">
        Run it as root on the new node (Linux with systemd, amd64 or arm64). It installs rtbhd with the built-in BGP
        speaker and enrolls it here.
      </p>
      <pre className="rounded-md border bg-muted/40 p-3 font-mono text-sm break-all whitespace-pre-wrap select-all">{command}</pre>
      <p className="text-warn">
        This is the only time the token is shown. It enrolls one node; revoke it below if it leaks.
      </p>
      <p className="text-xs text-muted-foreground">
        Optional flags: <code className="font-mono">--asn N</code>, <code className="font-mono">--router-id IP</code>,{" "}
        <code className="font-mono">--nexthop-v4 IP</code>, <code className="font-mono">--nexthop-v6 IP</code>. Add{" "}
        <code className="font-mono">--help</code> to list them.
      </p>
      <ErrorNote>{copyError}</ErrorNote>
    </section>
  );
}

export default function Agents() {
  const { data, error, loading, reload } = useResource(api.getAgents, 15000);
  const tokens = useResource(api.getAgentTokens, 15000);
  const action = useAction(() => {
    reload();
    tokens.reload();
  });
  const [adding, setAdding] = useState(false);
  const [name, setName] = useState("");
  const [created, setCreated] = useState<{ name: string; command: string } | null>(null);

  const agents: Agent[] = data?.items ?? [];
  const tokenList: AgentToken[] = tokens.data?.items ?? [];
  const agentName = (id: string) => agents.find((a) => a.id === id)?.name ?? id.slice(0, 8);
  const enrollOff = tokens.data && !tokens.data.portal_url;
  const noBinary = tokens.data && tokens.data.portal_url && !tokens.data.binaries.amd64 && !tokens.data.binaries.arm64;
  const nameOk = NAME_RE.test(name.trim());

  const create = async () => {
    const n = name.trim();
    if (!NAME_RE.test(n)) return action.setError("Use letters, digits, '.', '_' or '-' for the name.");
    let command = "";
    const ok = await action.run(async () => {
      const r = await api.createAgentToken(n);
      command = `curl -fsSL ${tokens.data?.install_url} | sudo bash -s -- --token ${r.token} --name ${n}`;
    });
    if (ok) {
      setCreated({ name: n, command });
      setName("");
      setAdding(false);
    }
  };

  return (
    <div className="space-y-6">
      <PageHeader title="Agents" description="Nodes running rtbhd that announce the blackhole feed to your routers.">
        <Button
          size="sm"
          variant={adding ? "outline" : "default"}
          aria-expanded={adding}
          aria-controls="agent-add"
          disabled={!!enrollOff}
          onClick={() => setAdding((v) => !v)}
        >
          {adding ? <X aria-hidden="true" /> : <Plus aria-hidden="true" />}
          {adding ? "Cancel" : "Add agent"}
        </Button>
      </PageHeader>

      {enrollOff && (
        <p className="text-warn">
          Remote enrollment is off. Set <code className="font-mono">RTBH_AGENT_PORTAL_URL</code> on the portal to the
          address agents dial (for example <code className="font-mono">agent.example.net:443</code>).
        </p>
      )}
      {noBinary && (
        <p className="text-warn">
          No agent binary is published, so the installer cannot download rtbhd. Put{" "}
          <code className="font-mono">rtbhd-linux-amd64</code> in <code className="font-mono">RTBH_AGENT_BINARY_DIR</code>.
        </p>
      )}

      {adding && (
        <form
          id="agent-add"
          className="flex flex-wrap items-end gap-3 rounded-lg border bg-card p-3"
          onSubmit={(e) => {
            e.preventDefault();
            create();
          }}
        >
          <Field label="Agent name" htmlFor="agent-name" className="w-64 max-w-full">
            <Input
              id="agent-name"
              placeholder="e.g. jkt-edge-1"
              value={name}
              onChange={(e) => setName(e.target.value)}
              aria-invalid={name !== "" && !nameOk}
              autoFocus
            />
          </Field>
          <Button type="submit" disabled={action.busy || !nameOk}>
            Create install command
          </Button>
          <p className="basis-full text-xs text-muted-foreground">
            Creates a one-node enrollment token and a command to paste on the new host.
          </p>
        </form>
      )}

      {created && <InstallCommand {...created} onDone={() => setCreated(null)} />}
      <ErrorNote>{action.error}</ErrorNote>

      <div className="rounded-lg border bg-card">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Name</TableHead>
              <TableHead>Status</TableHead>
              <TableHead>Engine / BIRD</TableHead>
              <TableHead>Hostname</TableHead>
              <TableHead>Last seen</TableHead>
              <TableHead className="w-12 text-right">Actions</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {agents.map((a) => (
              <TableRow key={a.id}>
                <TableCell className="font-medium">{a.name}</TableCell>
                <TableCell>
                  <StatusBadge on={agentOnline(a.status)} onLabel="online" offLabel="offline" />
                </TableCell>
                <TableCell>{a.bird_version || "-"}</TableCell>
                <TableCell>{a.hostname || "-"}</TableCell>
                <TableCell>{formatDate(a.last_seen)}</TableCell>
                <TableCell className="text-right">
                  <Confirm
                    title={`Remove agent ${a.name}?`}
                    description="Only the portal record is removed. A running agent registers again on its next connect; revoke its token to keep it out."
                    confirmLabel="Remove agent"
                    onConfirm={() => action.run(() => api.deleteAgent(a.id))}
                  >
                    <IconButton icon={Trash2} label={`Remove agent ${a.name}`} />
                  </Confirm>
                </TableCell>
              </TableRow>
            ))}
            <TableState
              colSpan={6}
              loading={loading}
              error={error}
              empty={agents.length === 0}
              emptyText="No agents registered. Use Add agent to get an install command."
              onRetry={reload}
            />
          </TableBody>
        </Table>
      </div>

      {tokenList.length > 0 && (
        <section aria-labelledby="tokens-h" className="space-y-3">
          <div>
            <h2 id="tokens-h" className="font-semibold">
              Enrollment tokens
            </h2>
            <p className="text-muted-foreground">
              Each token admits one node. Revoking disconnects that node at once.
            </p>
          </div>
          <div className="rounded-lg border bg-card">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Name</TableHead>
                  <TableHead>Used by</TableHead>
                  <TableHead>Last used</TableHead>
                  <TableHead>Created</TableHead>
                  <TableHead className="w-12 text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {tokenList.map((t) => (
                  <TableRow key={t.id}>
                    <TableCell className="font-medium">{t.name}</TableCell>
                    <TableCell>
                      {t.agent_id ? agentName(t.agent_id) : <span className="text-muted-foreground">Not used yet</span>}
                    </TableCell>
                    <TableCell title={t.last_used_at ? formatDate(t.last_used_at) : undefined}>
                      {t.last_used_at ? relativeTime(t.last_used_at) : "-"}
                    </TableCell>
                    <TableCell title={formatDate(t.created_at)}>{relativeTime(t.created_at)}</TableCell>
                    <TableCell className="text-right">
                      <Confirm
                        title={`Revoke token ${t.name}?`}
                        description={
                          t.agent_id
                            ? "The node using it is disconnected now and cannot reconnect with this token."
                            : "The install command with this token stops working."
                        }
                        confirmLabel="Revoke token"
                        onConfirm={() => action.run(() => api.revokeAgentToken(t.id))}
                      >
                        <IconButton icon={Trash2} label={`Revoke token ${t.name}`} />
                      </Confirm>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        </section>
      )}
    </div>
  );
}
