import { useState } from "react";
import { api } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import { Plus, RefreshCw, ShieldOff } from "lucide-react";

const IRR_SOURCES = [
  "RADB",
  "RIPE",
  "ARIN",
  "APNIC",
  "AFRINIC",
  "LACNIC",
  "NTTCOM",
  "ALTDB",
  "BELL",
];

export default function PrefixBuilder() {
  const [asn, setAsn] = useState("");
  const [ipv4, setIpv4] = useState(true);
  const [ipv6, setIpv6] = useState(true);
  const [selectedSources, setSelectedSources] = useState<string[]>(IRR_SOURCES);
  const [note, setNote] = useState("");
  const [prefixes, setPrefixes] = useState<string[]>([]);
  const [loading, setLoading] = useState(false);
  const [adding, setAdding] = useState(false);
  const [error, setError] = useState("");
  const [status, setStatus] = useState("");

  const formatError = (err: unknown, fallback: string) =>
    err instanceof Error ? err.message : fallback;

  const handleBuild = async () => {
    setError("");
    setStatus("");

    const trimmed = asn.trim();
    if (!trimmed) {
      setPrefixes([]);
      setError("ASN is required");
      return;
    }
    if (!ipv4 && !ipv6) {
      setPrefixes([]);
      setError("Select IPv4 and/or IPv6");
      return;
    }
    if (selectedSources.length === 0) {
      setPrefixes([]);
      setError("Select at least one IRR source");
      return;
    }

    setLoading(true);
    try {
      const result = await api.buildPrefixesFromASN({
        asn: trimmed,
        ipv4,
        ipv6,
        sources: selectedSources,
      });
      setPrefixes(result.prefixes);
    } catch (err) {
      setPrefixes([]);
      setError(formatError(err, "Failed to build prefixes"));
    } finally {
      setLoading(false);
    }
  };

  const handleBulkAdd = async (target: "prefixes" | "excludes") => {
    if (prefixes.length === 0) return;
    setAdding(true);
    setError("");
    setStatus("");
    try {
      const payload = { items: prefixes, note: note.trim() };
      const result =
        target === "prefixes"
          ? await api.bulkAddPrefixes(payload)
          : await api.bulkAddExcludes(payload);
      setStatus(
        `Added to ${target}: created=${result.created}, skipped=${result.skipped}, invalid=${result.invalid}`,
      );
    } catch (err) {
      setError(formatError(err, "Failed to add prefixes"));
    } finally {
      setAdding(false);
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-3">
        <h2 className="text-2xl font-bold">Prefix Builder</h2>
        <Badge variant="secondary">{prefixes.length} prefixes</Badge>
      </div>

      <div className="space-y-4">
        <div className="flex flex-wrap gap-3 items-end">
          <div className="space-y-2">
            <Label htmlFor="asn">ASN</Label>
            <Input
              id="asn"
              placeholder="AS13335 or 13335"
              value={asn}
              onChange={(e) => setAsn(e.target.value)}
              className="max-w-xs"
            />
          </div>

          <div className="flex items-center gap-4 pt-6">
            <label className="flex items-center gap-2 text-sm">
              <input
                type="checkbox"
                checked={ipv4}
                onChange={(e) => setIpv4(e.target.checked)}
                className="h-4 w-4"
              />
              IPv4
            </label>
            <label className="flex items-center gap-2 text-sm">
              <input
                type="checkbox"
                checked={ipv6}
                onChange={(e) => setIpv6(e.target.checked)}
                className="h-4 w-4"
              />
              IPv6
            </label>
          </div>

          <Button onClick={handleBuild} disabled={loading}>
            <RefreshCw className="h-4 w-4 mr-2" />
            {loading ? "Building..." : "Build"}
          </Button>
        </div>

        <div className="space-y-2">
          <Label>IRR Sources</Label>
          <div className="flex flex-wrap gap-3">
            {IRR_SOURCES.map((source) => (
              <label key={source} className="flex items-center gap-2 text-sm">
                <input
                  type="checkbox"
                  checked={selectedSources.includes(source)}
                  onChange={(e) => {
                    if (e.target.checked) {
                      setSelectedSources((prev) => [...prev, source]);
                    } else {
                      setSelectedSources((prev) =>
                        prev.filter((item) => item !== source),
                      );
                    }
                  }}
                  className="h-4 w-4"
                />
                {source}
              </label>
            ))}
          </div>
        </div>

        <div className="space-y-2">
          <Label htmlFor="note">Note (optional)</Label>
          <Input
            id="note"
            placeholder="e.g. from ASN builder"
            value={note}
            onChange={(e) => setNote(e.target.value)}
            className="max-w-lg"
          />
        </div>

        <div className="flex flex-wrap gap-2">
          <Button
            onClick={() => handleBulkAdd("prefixes")}
            disabled={adding || prefixes.length === 0}
          >
            <Plus className="h-4 w-4 mr-2" />
            Add to Prefixes
          </Button>
          <Button
            variant="secondary"
            onClick={() => handleBulkAdd("excludes")}
            disabled={adding || prefixes.length === 0}
          >
            <ShieldOff className="h-4 w-4 mr-2" />
            Add to Excludes
          </Button>
        </div>

        {error && (
          <div className="text-sm text-destructive" role="alert">
            {error}
          </div>
        )}
        {status && (
          <div className="text-sm text-muted-foreground">{status}</div>
        )}
      </div>

      <div className="space-y-2">
        <Label>Prefixes</Label>
        <Textarea
          value={prefixes.join("\n")}
          readOnly
          className="min-h-[240px] font-mono"
          placeholder="Prefixes will appear here after building."
        />
      </div>
    </div>
  );
}
