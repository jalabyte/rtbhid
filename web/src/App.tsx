import { BrowserRouter, Routes, Route } from "react-router";
import { Sidebar } from "./components/layout/Sidebar";
import Dashboard from "./pages/Dashboard";
import Agents from "./pages/Agents";
import Prefixes from "./pages/Prefixes";
import PrefixBuilder from "./pages/PrefixBuilder";
import Sources from "./pages/Sources";
import Excludes from "./pages/Excludes";
import Peers from "./pages/Peers";
import Events from "./pages/Events";
import Docs from "./pages/Docs";
import Lookup from "./pages/Lookup";

export default function App() {
  return (
    <BrowserRouter>
      <div className="flex h-screen">
        <Sidebar />
        <main className="flex-1 overflow-auto p-6">
          <Routes>
            <Route path="/" element={<Dashboard />} />
            <Route path="/agents" element={<Agents />} />
            <Route path="/prefixes" element={<Prefixes />} />
            <Route path="/prefix-builder" element={<PrefixBuilder />} />
            <Route path="/sources" element={<Sources />} />
            <Route path="/excludes" element={<Excludes />} />
            <Route path="/peers" element={<Peers />} />
            <Route path="/events" element={<Events />} />
            <Route path="/lookup" element={<Lookup />} />
            <Route path="/docs" element={<Docs />} />
          </Routes>
        </main>
      </div>
    </BrowserRouter>
  );
}
