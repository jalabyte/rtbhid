# Draft: IP/Prefix Lookup Page Feature

## Requirements (confirmed)
- Feature: IP/Prefix lookup page for finding which source/exclusion an IP belongs to
- Must support: Single IP (e.g., 1.1.1.1/32), Network prefixes (e.g., 1.0.0.0/24, any subnet size)
- All tasks must be converted to Orkes Kanban Board tickets under project "RTBHID"
- **Results Display**: Full details (source name, exclusion name, matched prefix, description, date added)
- **IP Version Support**: Both IPv4 and IPv6
- **Backend**: API endpoint for server-side IP matching

## Research Findings

### Codebase Architecture
- **Frontend**: React 19 + TypeScript + Vite + Tailwind CSS + shadcn/ui
- **Backend**: Go with SQLite (portal mode) or JSON (standalone mode)
- **Routing**: React Router v7 in `/web/src/App.tsx`
- **Navigation**: Sidebar in `/web/src/components/layout/Sidebar.tsx`

### Data Models (Portal Mode)
```go
// Source - Remote URL that syncs prefixes
type Source struct {
    ID        string     // UUID
    Name      string
    URL       string     // Remote URL to fetch
    Interval  int        // Sync interval in seconds
    Enabled   bool
    LastSync  *time.Time
    LastCount int        // Number of prefixes fetched
    CreatedAt time.Time
}

// Exclude - Manual CIDR exclusion
type Exclude struct {
    ID        int64
    CIDR      string     // IP/CIDR to exclude
    Note      string
    CreatedAt time.Time
}
```

### Frontend Types (`/web/src/lib/api.ts`)
```typescript
interface Source {
  id: string;
  name: string;
  url: string;
  interval: number;
  enabled: boolean;
  last_sync: string | null;
  last_count: number;
  created_at: string;
}

interface Exclude {
  id: number;
  cidr: string;
  note: string;
  created_at: string;
}
```

### IP/CIDR Handling (Backend)
- Uses Go's `net/netip` package for robust CIDR normalization
- `normalizeCIDR()` function handles both CIDR notation and bare IPs
- Bare IPs are converted to /32 (IPv4) or /128 (IPv6)
- `isExcludedByCIDR()` checks if a prefix is contained within exclusion list

### Effective Prefixes Calculation
1. Collect enabled manual prefixes
2. Collect enabled source prefixes (from remote URL syncs)
3. Build exclusion set from:
   - Manual excludes (`excludes` table)
   - Exclude source prefixes (from remote URL syncs)
4. Merge: manual + source - excludes (using CIDR-aware matching)

### Page Pattern (Canonical)
Pages in `/web/src/pages/` follow this pattern:
1. Import from `@/lib/api` and `@/components/ui/*`
2. Use `useState` for items and form state
3. Use `useEffect` to load data on mount
4. API calls via `api.getXxx()` functions
5. Layout: `<div className="space-y-6">` with `<h2>` and form/tables

### API Pattern
- `GET /api/v1/xxx` — List items
- `POST /api/v1/xxx` — Create item
- `DELETE /api/v1/xxx/:id` — Delete item
- `PUT /api/v1/xxx/:id/toggle` — Enable/disable

### Key Files for Implementation
- `/web/src/pages/Lookup.tsx` — New page (create)
- `/web/src/App.tsx` — Add route
- `/web/src/components/layout/Sidebar.tsx` — Add nav item
- `/web/src/lib/api.ts` — Add lookup API function
- `/internal/portal/httpapi/server.go` — Add lookup endpoint
- `/internal/portal/state/store.go` — Add lookup query method

## IP/CIDR Library Recommendation

### Frontend (TypeScript)
- **Library**: `ipaddr.js` (1.9KB gzipped, zero deps, IPv4+IPv6 support)
- **Pattern**: `ipaddr.parse(ip).match(ipaddr.parseCIDR(cidr))`
```typescript
import ipaddr from 'ipaddr.js';

// Parse and match
const addr = ipaddr.parse('192.168.1.100');
const range = ipaddr.parseCIDR('192.168.1.0/24');
addr.match(range); // => true

// Safe matching (handles IPv4 vs IPv6)
function safeMatch(ip: string, cidr: string): boolean {
  const parsedIp = ipaddr.parse(ip);
  const [rangeAddr, bits] = ipaddr.parseCIDR(cidr);
  if (parsedIp.kind() !== rangeAddr.kind()) return false;
  return parsedIp.match([rangeAddr, bits]);
}
```

### Backend (Go)
- Already uses `net/netip` package
- Existing `normalizeCIDR()` and `isExcludedByCIDR()` functions
- Can reuse for lookup functionality

### Performance
- < 1000 prefixes: Linear scan is fine (~1ms)
- > 1000 prefixes: Consider first-octet indexing

## Confirmed Requirements

### User Flow
1. User enters IP/prefix (single or bulk, one per line)
2. System searches both Sources and Exclusions
3. Results show all matching prefixes (hierarchical), with most specific first
4. Each result shows: Source name, Exclusion name (if applicable), Matched prefix, Note/description, Date added

### Display Options
- Full details (source name, exclusion name, matched prefix, description, date added)
- Hierarchical results (all CIDR ranges that contain the search IP/prefix)
- Most specific match listed first

### Technical Decisions
- **Frontend Location**: `/web/src/pages/Lookup.tsx`
- **Backend Endpoint**: `POST /api/v1/lookup` (accepts IP/CIDR, returns matches)
- **Data Flow**: Frontend → API → Store → CIDR matching → Results
- **Frontend Library**: `ipaddr.js` for client-side validation
- **Backend Library**: Go's `net/netip` (already in use)
- **Test Framework**: Vitest (to be set up)
- **UI Layout**: Simple search box + results table below

### API Design
```typescript
// Request
POST /api/v1/lookup
{
  "queries": ["1.1.1.1", "192.168.0.0/24"]  // Single or bulk
}

// Response
{
  "results": [
    {
      "query": "1.1.1.1",
      "matches": [
        {
          "type": "source" | "exclude_source" | "manual_exclude",
          "source_id": "src_abc123",
          "source_name": "Spamhaus DROP",
          "cidr": "1.0.0.0/8",
          "note": "...",
          "created_at": "2024-01-15T..."
        }
      ]
    }
  ]
}
```

## Scope Boundaries
- IN: 
  - Lookup page with search functionality
  - Backend API endpoint
  - Navigation integration (add to sidebar)
  - Test infrastructure setup (vitest)
  - Unit tests for lookup functionality
- OUT: 
  - Authentication/authorization changes
  - Database schema changes
  - Changes to existing pages (Sources, Excludes, etc.)
  - gRPC changes (only affecting agents)