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
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Loader2, ChevronLeft, ChevronRight } from "lucide-react";

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
    <div className="space-y-6">
      <div className="flex flex-col gap-2">
        <h2 className="text-2xl font-bold">Remote Exclusion Prefixes</h2>
        <p className="text-muted-foreground">
          View all exclusion prefixes synchronized from remote sources
        </p>
      </div>

      {error && (
        <Alert variant="destructive">
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}

      <div className="space-y-4">
        <div className="border rounded-md">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Source Name</TableHead>
                <TableHead>CIDR</TableHead>
                <TableHead>Created At</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {loading && prefixes.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={3} className="text-center py-8">
                    <Loader2 className="h-6 w-6 animate-spin mx-auto text-muted-foreground" />
                  </TableCell>
                </TableRow>
              ) : prefixes.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={3} className="text-center py-8 text-muted-foreground">
                    No prefixes found
                  </TableCell>
                </TableRow>
              ) : (
                prefixes.map((prefix, idx) => (
                  <TableRow key={`${prefix.source_id}-${prefix.cidr}-${idx}`}>
                    <TableCell className="font-medium">
                      {prefix.source_name}
                    </TableCell>
                    <TableCell className="font-mono">{prefix.cidr}</TableCell>
                    <TableCell>
                      {new Date(prefix.created_at).toLocaleString()}
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </div>

        <div className="flex items-center justify-between">
          <div className="text-sm text-muted-foreground">
            {total > 0 ? (
              <>
                Showing {offset + 1} - {Math.min(offset + limit, total)} of {total}
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
              <ChevronLeft className="h-4 w-4 mr-1" />
              Previous
            </Button>
            <Button
              variant="outline"
              size="sm"
              onClick={handleNext}
              disabled={offset + limit >= total || loading}
            >
              Next
              <ChevronRight className="h-4 w-4 ml-1" />
            </Button>
          </div>
        </div>
      </div>
    </div>
  );
}
