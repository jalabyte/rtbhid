import { useState } from "react";
import { api, type LookupResult } from "@/lib/api";
import * as ipaddr from "ipaddr.js";
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
import { Textarea } from "@/components/ui/textarea";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Loader2 } from "lucide-react";

export default function Lookup() {
  const [input, setInput] = useState("");
  const [results, setResults] = useState<LookupResult[] | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const validateIPOrCIDR = (input: string): boolean => {
    try {
      // Check if it's a CIDR notation
      if (input.includes("/")) {
        const [addr, prefix] = input.split("/");
        if (!addr || !prefix) return false;
        const prefixNum = parseInt(prefix, 10);
        if (isNaN(prefixNum)) return false;

        // Validate the IP part
        const parsed = ipaddr.parse(addr);
        const isV4 = parsed.kind() === "ipv4";
        const maxPrefix = isV4 ? 32 : 128;
        return prefixNum >= 0 && prefixNum <= maxPrefix;
      }
      // Check if it's a plain IP address
      ipaddr.parse(input);
      return true;
    } catch {
      return false;
    }
  };

  const handleSearch = async () => {
    setLoading(true);
    setError(null);
    setResults(null);

    try {
      const queries = input
        .split("\n")
        .map((line) => line.trim())
        .filter((line) => line.length > 0);

      if (queries.length === 0) {
        setError("Please enter at least one IP address or CIDR");
        setLoading(false);
        return;
      }

      if (queries.length > 10) {
        setError("Maximum 10 IP addresses or CIDRs allowed");
        setLoading(false);
        return;
      }

      // Validate each query
      const invalidQueries = queries.filter((q) => !validateIPOrCIDR(q));
      if (invalidQueries.length > 0) {
        setError(
          `Invalid IP address or CIDR: ${invalidQueries.join(", ")}`
        );
        setLoading(false);
        return;
      }

      const response = await api.lookup(queries);
      setResults(response.results);
      setInput("");
    } catch (err) {
      setError(err instanceof Error ? err.message : "An error occurred");
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="space-y-6">
      <h2 className="text-2xl font-bold">IP Lookup</h2>
      <p className="text-muted-foreground">
        Look up IP addresses or CIDRs to find matching prefixes and excludes
      </p>

      <div className="space-y-4">
        <Textarea
          placeholder="Enter IP addresses or CIDRs (one per line)..."
          value={input}
          onChange={(e) => setInput(e.target.value)}
          className="min-h-[120px]"
          disabled={loading}
        />
        <Button onClick={handleSearch} disabled={loading}>
          {loading ? (
            <>
              <Loader2 className="h-4 w-4 mr-1 animate-spin" />
              Searching...
            </>
          ) : (
            "Search"
          )}
        </Button>
      </div>

      {error && (
        <Alert variant="destructive">
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}

      {results && (
        <div className="space-y-4">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Query</TableHead>
                <TableHead>Type</TableHead>
                <TableHead>Name</TableHead>
                <TableHead>CIDR</TableHead>
                <TableHead>Note</TableHead>
                <TableHead>Date</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {results.every((r) => r.matches.length === 0) && results.length === 1 ? (
                <TableRow>
                  <TableCell colSpan={6} className="text-center text-muted-foreground">
                    No matches found
                  </TableCell>
                </TableRow>
              ) : (
                results.flatMap((result) =>
                  result.matches.length > 0
                    ? result.matches.map((match, idx) => (
                        <TableRow key={`${result.query}-${idx}`}>
                          <TableCell className="font-medium" data-query>
                            {result.query}
                          </TableCell>
                          <TableCell>
                            <Badge variant={match.type === "exclude" ? "secondary" : "default"}>
                              {match.type}
                            </Badge>
                          </TableCell>
                          <TableCell>{match.source_name}</TableCell>
                          <TableCell className="font-mono" data-cidr>
                            {match.cidr === result.query ? "matched" : match.cidr}
                          </TableCell>
                          <TableCell>{match.note || "—"}</TableCell>
                          <TableCell>{new Date(match.created_at).toLocaleString()}</TableCell>
                        </TableRow>
                      ))
                    : results.length > 1
                      ? [
                          <TableRow key={result.query}>
                            <TableCell className="font-medium">{result.query}</TableCell>
                            <TableCell>—</TableCell>
                            <TableCell>—</TableCell>
                            <TableCell>—</TableCell>
                            <TableCell>—</TableCell>
                          </TableRow>,
                        ]
                      : []
                )
              )}
            </TableBody>
          </Table>
        </div>
      )}
    </div>
  );
}
