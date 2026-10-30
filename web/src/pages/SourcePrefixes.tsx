import { useEffect, useState } from "react";
import { useParams, useNavigate } from "react-router";
import { api, type Source } from "@/lib/api";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Button } from "@/components/ui/button";
import { ChevronLeft, ChevronRight, ArrowLeft } from "lucide-react";
import { ErrorNote, PageHeader, StatusBadge } from "@/components/app";

interface PrefixesResponse {
  items: string[];
  total: number;
  page: number;
  limit: number;
}

export default function SourcePrefixes() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [source, setSource] = useState<Source | null>(null);
  const [data, setData] = useState<PrefixesResponse | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [page, setPage] = useState(1);
  const limit = 100;

  const loadSource = async () => {
    if (!id) return;
    try {
      const sources = await api.getSources();
      const found = sources.items.find((s) => s.id === id);
      if (found) {
        setSource(found);
      } else {
        setError("Source not found");
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load source");
    }
  };

  const loadPrefixes = async () => {
    if (!id) return;
    setLoading(true);
    setError(null);
    try {
      const response = await api.getSourcePrefixesByID(id, page, limit);
      setData(response);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load prefixes");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadSource();
  }, [id]);

  useEffect(() => {
    loadPrefixes();
  }, [id, page]);

  const totalPages = data ? Math.ceil(data.total / data.limit) : 0;
  const startItem = data ? (data.page - 1) * data.limit + 1 : 0;
  const endItem = data ? Math.min(startItem + data.items.length - 1, data.total) : 0;

  if (error) {
    return (
      <div className="space-y-4">
        <Button variant="ghost" onClick={() => navigate("/sources")}>
          <ArrowLeft className="h-4 w-4 mr-2" />
          Back to Sources
        </Button>
        <ErrorNote>{error}</ErrorNote>
      </div>
    );
  }

  return (
    <div className="space-y-4">
      <Button variant="ghost" size="sm" className="-ml-2" onClick={() => navigate("/sources")}>
        <ArrowLeft aria-hidden="true" />
        Sources
      </Button>
      <PageHeader
        title={source ? source.name || source.id : "Loading…"}
        description={
          source && (
            <span className="flex flex-wrap items-center gap-2">
              <span className="font-mono text-xs break-all">{source.url}</span>
              <StatusBadge on={source.enabled} onLabel="active" offLabel="inactive" />
            </span>
          )
        }
      />

      <div className="space-y-4">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <p className="text-muted-foreground">
            {data ? (
              <>
                Showing <span className="font-medium">{startItem}</span> to{" "}
                <span className="font-medium">{endItem}</span> of{" "}
                <span className="font-medium">{data.total}</span> prefixes
              </>
            ) : (
              "Loading…"
            )}
          </p>
          <div className="flex items-center gap-2">
            <Button
              variant="outline"
              size="sm"
              onClick={() => setPage((p) => Math.max(1, p - 1))}
              disabled={loading || page <= 1}
              aria-label="Previous page"
            >
              <ChevronLeft aria-hidden="true" />
            </Button>
            <span className="text-sm text-muted-foreground min-w-[100px] text-center">
              Page {page} of {totalPages || 1}
            </span>
            <Button
              variant="outline"
              size="sm"
              onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
              disabled={loading || page >= totalPages}
              aria-label="Next page"
            >
              <ChevronRight aria-hidden="true" />
            </Button>
          </div>
        </div>

        <div className="rounded-lg border bg-card">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="w-16">#</TableHead>
              <TableHead>CIDR</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {loading ? (
              <TableRow>
                <TableCell colSpan={2} className="text-center text-muted-foreground">
                  Loading…
                </TableCell>
              </TableRow>
            ) : data?.items.length === 0 ? (
              <TableRow>
                <TableCell colSpan={2} className="text-center text-muted-foreground">
                  No prefixes found
                </TableCell>
              </TableRow>
            ) : (
              data?.items.map((cidr, index) => (
                <TableRow key={cidr}>
                  <TableCell className="text-muted-foreground text-sm">
                    {startItem + index}
                  </TableCell>
                  <TableCell className="font-mono">{cidr}</TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
        </div>
      </div>
    </div>
  );
}
