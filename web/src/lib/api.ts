const BASE = "/api/v1";

async function fetchJSON<T>(url: string): Promise<T> {
  const res = await fetch(`${BASE}${url}`);
  if (!res.ok) throw new Error(await res.text());
  return res.json();
}

async function postJSON<T>(url: string, data: unknown): Promise<T> {
  const res = await fetch(`${BASE}${url}`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(data),
  });
  if (!res.ok) throw new Error(await res.text());
  return res.json();
}

async function deleteJSON<T>(url: string): Promise<T> {
  const res = await fetch(`${BASE}${url}`, { method: "DELETE" });
  if (!res.ok) throw new Error(await res.text());
  return res.json();
}

export interface Agent {
  id: string;
  name: string;
  hostname: string;
  bird_version: string;
  listen_addr: string;
  status: string;
  last_seen: string | null;
  created_at: string;
  updated_at: string;
}

export interface Prefix {
  id: number;
  cidr: string;
  enabled: boolean;
  note: string;
  created_at: string;
}

export interface Source {
  id: string;
  name: string;
  url: string;
  interval: number;
  enabled: boolean;
  last_sync: string | null;
  last_count: number;
  created_at: string;
}

export interface Exclude {
  id: number;
  cidr: string;
  note: string;
  created_at: string;
}

export interface Peer {
  id: string;
  name: string;
  neighbor_ip: string;
  remote_as: number;
  local_as: number;
  multihop: number;
  enabled: boolean;
  std_communities: string[];
  large_communities: string[];
  created_at: string;
}

export interface Event {
  id: number;
  type: string;
  message: string;
  agent_id: string;
  created_at: string;
}

export interface Status {
  agents: number;
  prefixes: number;
  sources: number;
  excludes: number;
  peers: number;
  whitelist?: number;
  auto_accept?: boolean;
  effective_prefixes: number;
  local_asn: number;
}

export interface WhitelistASN {
  id: string;
  asn: number;
  name: string;
  country: string;
  source: string;
  enabled: boolean;
  created_at: string;
}

export interface WhitelistResponse {
  items: WhitelistASN[];
  count: number;
  auto_accept: boolean;
  strict_id: boolean;
  id_registry_count: number;
}

export interface BGPSession {
  agent_id: string;
  agent_name?: string;
  peer: string;
  name: string;
  remote_as: number;
  local_as: number;
  state: string;
  passive: boolean;
  uptime_sec: number;
  prefixes_in: number;
  last_error: string;
  updated_at: string;
}

export interface BGPOverview {
  agents: number;
  sessions: number;
  established: number;
  prefixes_in: number;
  engines: Record<string, string>;
  items: BGPSession[];
}

export interface LookupMatch {
  type: string;
  source_id: string;
  source_name: string;
  cidr: string;
  note: string;
  created_at: string;
  asn?: string;
  as_name?: string;
  as_domain?: string;
}

export interface LookupResult {
  query: string;
  matches: LookupMatch[];
}

export interface LookupResponse {
  results: LookupResult[];
}

export interface PrefixBuilderResponse {
  asn: string;
  count: number;
  prefixes: string[];
}

export interface BulkAddResponse {
  created: number;
  skipped: number;
  invalid: number;
}

export interface ExcludeSourcePrefix {
  source_id: string;
  source_name: string;
  cidr: string;
  created_at: string;
}

export const api = {
  getStatus: () => fetchJSON<Status>("/status"),

  getMe: () =>
    fetchJSON<{ enabled: boolean; logged_in: boolean; username?: string }>(
      "/auth/me",
    ),
  login: (data: { username: string; password: string }) =>
    postJSON<{ ok: boolean }>("/auth/login", data),
  logout: () => postJSON<{ ok: boolean }>("/auth/logout", {}),

  getAgents: () => fetchJSON<{ items: Agent[] }>("/agents"),
  deleteAgent: (id: string) => deleteJSON(`/agents/${id}`),

  getPrefixes: () => fetchJSON<{ items: Prefix[] }>("/prefixes"),
  addPrefix: (data: { cidr: string; note: string }) =>
    postJSON<Prefix>("/prefixes", data),
  deletePrefix: (id: number) => deleteJSON(`/prefixes/${id}`),
  togglePrefix: (id: number, enabled: boolean) =>
    fetch(`${BASE}/prefixes/${id}/toggle`, {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ enabled }),
    }),

  getSources: () => fetchJSON<{ items: Source[] }>("/sources"),
  addSource: (data: { name: string; url: string; interval: number }) =>
    postJSON<Source>("/sources", data),
  deleteSource: (id: string) => deleteJSON(`/sources/${id}`),
  toggleSource: (id: string, enabled: boolean) =>
    fetch(`${BASE}/sources/${id}/toggle`, {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ enabled }),
    }),
  syncSource: (id: string) =>
    fetch(`${BASE}/sources/${id}/sync`, { method: "POST" }),

  getExcludes: () => fetchJSON<{ items: Exclude[] }>("/excludes"),
  addExclude: (data: { cidr: string; note: string }) =>
    postJSON<Exclude>("/excludes", data),
  deleteExclude: (id: number) => deleteJSON(`/excludes/${id}`),

  getExcludeSources: () => fetchJSON<{ items: Source[] }>("/exclude-sources"),
  addExcludeSource: (data: { name: string; url: string; interval: number }) =>
    postJSON<Source>("/exclude-sources", data),
  deleteExcludeSource: (id: string) => deleteJSON(`/exclude-sources/${id}`),
  toggleExcludeSource: (id: string, enabled: boolean) =>
    fetch(`${BASE}/exclude-sources/${id}/toggle`, {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ enabled }),
    }),
  syncExcludeSource: (id: string) =>
    fetch(`${BASE}/exclude-sources/${id}/sync`, { method: "POST" }),
  getExcludeSourcePrefixes: (offset = 0, limit = 100) =>
    fetchJSON<{
      items: ExcludeSourcePrefix[];
      total: number;
      offset: number;
      limit: number;
    }>(`/exclude-sources/prefixes?offset=${offset}&limit=${limit}`),

  getSourcePrefixesByID: (id: string, page = 1, limit = 100) =>
    fetchJSON<{
      items: string[];
      total: number;
      page: number;
      limit: number;
    }>(`/sources/${id}/prefixes?page=${page}&limit=${limit}`),

  getExcludeSourcePrefixesByID: (id: string, page = 1, limit = 100) =>
    fetchJSON<{
      items: string[];
      total: number;
      page: number;
      limit: number;
    }>(`/exclude-sources/${id}/prefixes?page=${page}&limit=${limit}`),

  getPeers: () => fetchJSON<{ items: Peer[] }>("/peers"),
  addPeer: (data: Omit<Peer, "id" | "created_at">) =>
    postJSON<Peer>("/peers", data),
  bulkAddPeers: (
    items: Array<{
      name: string;
      neighbor_ip: string;
      remote_as: number;
      multihop: number;
      enabled?: boolean;
      std_communities?: string[];
      large_communities?: string[];
    }>,
  ) => postJSON<{ created: number }>("/peers/bulk", { items }),
  togglePeer: (id: string, enabled: boolean) =>
    fetch(`${BASE}/peers/${id}/toggle`, {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ enabled }),
    }),
  deletePeer: (id: string) => deleteJSON(`/peers/${id}`),

  getWhitelist: () => fetchJSON<WhitelistResponse>("/whitelist"),
  addWhitelistASN: (data: {
    asn: number;
    name: string;
    allow_non_id?: boolean;
  }) => postJSON<WhitelistASN>("/whitelist", data),
  deleteWhitelistASN: (id: string) => deleteJSON(`/whitelist/${id}`),
  toggleWhitelistASN: (id: string, enabled: boolean) =>
    fetch(`${BASE}/whitelist/${id}/toggle`, {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ enabled }),
    }),
  importIndonesianASNs: () =>
    postJSON<{
      created: number;
      total: number;
      count: number;
      id_registry_count: number;
      warnings: string[];
    }>("/whitelist/import", {}),
  setAutoAccept: (enabled: boolean) =>
    fetch(`${BASE}/whitelist/auto-accept`, {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ enabled }),
    }),

  getEvents: (limit = 100) =>
    fetchJSON<{ items: Event[] }>(`/events?limit=${limit}`),

  getBGPOverview: () => fetchJSON<BGPOverview>("/bgp"),

  buildPrefixesFromASN: (data: {
    asn: string;
    ipv4?: boolean;
    ipv6?: boolean;
    sources?: string[];
  }) => postJSON<PrefixBuilderResponse>("/prefix-builder", data),
  bulkAddPrefixes: (data: { items: string[]; note: string }) =>
    postJSON<BulkAddResponse>("/prefixes/bulk", data),
  bulkAddExcludes: (data: { items: string[]; note: string }) =>
    postJSON<BulkAddResponse>("/excludes/bulk", data),

  lookup: (queries: string[]) =>
    fetch(`${BASE}/lookup`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ queries }),
    }).then(async (res) => {
      if (!res.ok) throw new Error(await res.text());
      return res.json() as Promise<LookupResponse>;
    }),
};
