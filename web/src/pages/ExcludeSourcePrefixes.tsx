import { useState, useEffect } from "react";
import { api, type ExcludeSourcePrefix } from "@/lib/api";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Button } from "@/components/ui/button";
import { ChevronLeft, ChevronRight } from "lucide-react";
import { ErrorNote, PageHeader, TableState } from "@/components/app";
import { formatDate } from "@/lib/format";

export default function ExcludeSourcePrefixes() {
  const [prefixes, setPrefixes] = useState<ExcludeSourcePrefix[]>([]);
  const [total, setTotal] = useState(0);
  const [offset, setOffset] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const limit = 100;

  useEffect(() => {
    fetchPrefixes();
  }, [offset]);

  const fetchPrefixes = async () => {
    setLoading(true);
    setError(null);
    try {
      const response = await api.getExcludeSourcePrefixes(offset, limit);
      setPrefixes(response.items || []);
      setTotal(response.total || 0);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to fetch prefixes");
    } finally {
      setLoading(false);
    }
  };

  const handlePrevious = () => {
    if (offset >= limit) {
      setOffset(offset - limit);
    }
  };

  const handleNext = () => {
    if (offset + limit < total) {
      setOffset(offset + limit);
    }
  };

  return (
    <div className="space-y-4">
      <PageHeader
        title="Remote Excludes"
        description="Exclusion prefixes synchronized from remote exclude sources. Manage the sources under Excludes."
      />
      <ErrorNote>{error && prefixes.length > 0 ? error : ""}</ErrorNote>

      <div className="space-y-4">
        <div className="rounded-lg border bg-card">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Source Name</TableHead>
                <TableHead>CIDR</TableHead>
                <TableHead>Created At</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              <TableState
                colSpan={3}
                loading={loading}
                error={error ?? ""}
                empty={prefixes.length === 0}
                emptyText="No remote exclusion prefixes."
                onRetry={fetchPrefixes}
              />
              {prefixes.map((prefix, idx) => (
                  <TableRow key={`${prefix.source_id}-${prefix.cidr}-${idx}`}>
                    <TableCell className="font-medium">
                      {prefix.source_name}
                    </TableCell>
                    <TableCell className="font-mono">{prefix.cidr}</TableCell>
                    <TableCell>{formatDate(prefix.created_at)}</TableCell>
                  </TableRow>
                ))}
            </TableBody>
          </Table>
        </div>

        <div className="flex flex-wrap items-center justify-between gap-2">
          <div className="text-muted-foreground">
            {total > 0 ? (
              <>
                Showing {offset + 1} to {Math.min(offset + limit, total)} of {total}
              </>
            ) : (
              "Showing 0 results"
            )}
          </div>
          <div className="flex items-center gap-2">
            <Button
              variant="outline"
              size="sm"
              onClick={handlePrevious}
              disabled={offset === 0 || loading}
            >
              <ChevronLeft aria-hidden="true" />
              Previous
            </Button>
            <Button
              variant="outline"
              size="sm"
              onClick={handleNext}
              disabled={offset + limit >= total || loading}
            >
              Next
              <ChevronRight aria-hidden="true" />
            </Button>
          </div>
        </div>
      </div>
    </div>
  );
}
