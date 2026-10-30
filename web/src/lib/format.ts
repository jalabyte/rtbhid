export function formatDate(iso: string | null | undefined) {
  if (!iso) return "-";
  const t = new Date(iso);
  return Number.isNaN(t.getTime()) ? "-" : t.toLocaleString();
}

export function relativeTime(iso: string | null | undefined) {
  if (!iso) return "-";
  const then = new Date(iso).getTime();
  if (Number.isNaN(then)) return "-";
  const diff = Math.floor((Date.now() - then) / 1000);
  if (diff < 5) return "just now";
  if (diff < 60) return `${diff}s ago`;
  if (diff < 3600) return `${Math.floor(diff / 60)}m ago`;
  if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`;
  return `${Math.floor(diff / 86400)}d ago`;
}

// Agent status is "online" at registration, then the heartbeat's BIRD status
// (e.g. "running"), and "offline" once the portal marks it stale.
export function agentOnline(status: string) {
  return status !== "" && status !== "offline";
}
