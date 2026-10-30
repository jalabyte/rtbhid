import { api } from "@/lib/api";
import { PageHeader } from "@/components/app";
import { SourceManager } from "@/components/SourceManager";

const ops = {
  list: api.getSources,
  add: api.addSource,
  toggle: api.toggleSource,
  sync: api.syncSource,
  remove: api.deleteSource,
};

export default function Sources() {
  return (
    <div className="space-y-4">
      <PageHeader
        title="Sources"
        description="Remote blocklists fetched on an interval and merged into the feed."
      />
      <SourceManager
        ops={ops}
        idPrefix="src"
        noun="source"
        removeHint="Its prefixes are withdrawn from all routers on the next sync."
        viewHref={(s) => `/sources/${s.id}/prefixes`}
      />
    </div>
  );
}
