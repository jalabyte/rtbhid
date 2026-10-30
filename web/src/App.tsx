import { useEffect, useState } from "react";
import { BrowserRouter, Routes, Route } from "react-router";
import { api } from "./lib/api";
import { NullwellMark } from "./components/app";
import { Sidebar } from "./components/layout/Sidebar";
import Dashboard from "./pages/Dashboard";
import Agents from "./pages/Agents";
import Prefixes from "./pages/Prefixes";
import PrefixBuilder from "./pages/PrefixBuilder";
import Sources from "./pages/Sources";
import SourcePrefixes from "./pages/SourcePrefixes";
import Excludes from "./pages/Excludes";
import ExcludeSourcePrefixes from "./pages/ExcludeSourcePrefixes";
import Peers from "./pages/Peers";
import Whitelist from "./pages/Whitelist";
import BGPMonitor from "./pages/BGPMonitor";
import Events from "./pages/Events";
import Docs from "./pages/Docs";
import Lookup from "./pages/Lookup";

type Session = { enabled: boolean; loggedIn: boolean; username: string };

// What a visitor who isn't signed in may see: the public docs, nothing else.
function PublicDocs() {
  return (
    <div className="flex h-dvh flex-col">
      <header className="flex h-14 shrink-0 items-center justify-between border-b bg-card px-4">
        <div className="flex items-center gap-2.5">
          <NullwellMark className="size-7" />
          <span className="font-semibold">Nullwell</span>
        </div>
        <a href="/login" className="rounded-md border px-3 py-1.5 font-medium hover:bg-accent">
          Sign in
        </a>
      </header>
      <main className="min-w-0 flex-1 overflow-auto p-4 md:p-5">
        <Docs />
      </main>
    </div>
  );
}

export default function App() {
  const [session, setSession] = useState<Session | null>(null);

  useEffect(() => {
    api
      .getMe()
      .then((r) => setSession({ enabled: r.enabled, loggedIn: r.logged_in, username: r.username ?? "" }))
      // Unknown state: render the app; every API call is still authorized server-side.
      .catch(() => setSession({ enabled: false, loggedIn: false, username: "" }));
  }, []);

  if (!session) return null;
  if (session.enabled && !session.loggedIn) {
    if (window.location.pathname === "/docs") return <PublicDocs />;
    window.location.replace("/login");
    return null;
  }

  return (
    <BrowserRouter>
      <div className="flex h-dvh flex-col md:flex-row">
        <Sidebar authEnabled={session.enabled} username={session.username} />
        <main className="min-w-0 flex-1 overflow-auto p-4 md:p-5">
          <Routes>
            <Route path="/" element={<Dashboard />} />
            <Route path="/agents" element={<Agents />} />
            <Route path="/prefixes" element={<Prefixes />} />
            <Route path="/prefix-builder" element={<PrefixBuilder />} />
            <Route path="/sources" element={<Sources />} />
            <Route path="/sources/:id/prefixes" element={<SourcePrefixes />} />
            <Route path="/excludes" element={<Excludes />} />
            <Route path="/exclude-prefixes" element={<ExcludeSourcePrefixes />} />
            <Route path="/peers" element={<Peers />} />
            <Route path="/whitelist" element={<Whitelist />} />
            <Route path="/bgp" element={<BGPMonitor />} />
            <Route path="/events" element={<Events />} />
            <Route path="/lookup" element={<Lookup />} />
            <Route path="/docs" element={<Docs />} />
          </Routes>
        </main>
      </div>
    </BrowserRouter>
  );
}
