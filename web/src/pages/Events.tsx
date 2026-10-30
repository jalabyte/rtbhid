import { api, type Event } from "@/lib/api";
import { useResource } from "@/hooks/use-api";
import { PageHeader, TableState } from "@/components/app";
import { formatDate } from "@/lib/format";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { cn } from "@/lib/utils";

export default function Events() {
  const { data, error, loading, reload } = useResource(() => api.getEvents(), 15000);
  const events: Event[] = data?.items ?? [];

  return (
    <div className="space-y-4">
      <PageHeader title="Events" description="Latest 100 events, newest first." />
      <div className="rounded-lg border bg-card">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="w-44">Time</TableHead>
              <TableHead className="w-28">Type</TableHead>
              <TableHead>Message</TableHead>
              <TableHead className="w-40">Agent</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {events.map((e) => (
              <TableRow key={e.id}>
                <TableCell className="text-muted-foreground">{formatDate(e.created_at)}</TableCell>
                <TableCell className={cn("font-medium", e.type === "error" && "text-destructive")}>
                  {e.type}
                </TableCell>
                <TableCell className="whitespace-normal">{e.message}</TableCell>
                <TableCell className="font-mono text-xs">{e.agent_id || "-"}</TableCell>
              </TableRow>
            ))}
            <TableState
              colSpan={4}
              loading={loading}
              error={error}
              empty={events.length === 0}
              emptyText="No events yet."
              onRetry={reload}
            />
          </TableBody>
        </Table>
      </div>
    </div>
  );
}
