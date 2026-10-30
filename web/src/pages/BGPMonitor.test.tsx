import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import BGPMonitor from "./BGPMonitor";
import { api } from "@/lib/api";

vi.mock("@/lib/api", () => ({
  api: {
    getBGPOverview: vi.fn(),
  },
}));

const mockGet = vi.mocked(api.getBGPOverview);

function overview(updatedAt: string) {
  return {
    agents: 1,
    sessions: 1,
    established: 1,
    prefixes_in: 5,
    engines: { "agent-1": "custom" },
    items: [
      {
        agent_id: "agent-1",
        agent_name: "router-01",
        peer: "192.0.2.1",
        name: "peer-a",
        remote_as: 7713,
        local_as: 65000,
        state: "Established",
        passive: false,
        uptime_sec: 3661,
        prefixes_in: 5,
        last_error: "",
        updated_at: updatedAt,
      },
    ],
  };
}

describe("BGP Monitor Page", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockGet.mockResolvedValue(overview(new Date().toISOString()));
  });

  it("renders summary, agents, and sessions", async () => {
    render(<BGPMonitor />);

    expect((await screen.findAllByText("router-01")).length).toBeGreaterThanOrEqual(1);
    const row = screen.getByText("192.0.2.1").closest("tr");
    expect(row).not.toBeNull();
    expect(within(row as HTMLElement).getByText("Established")).toBeInTheDocument();
    expect(screen.getByText("AS7713")).toBeInTheDocument();
    expect(screen.getByText("1h 1m")).toBeInTheDocument();
    expect(screen.getAllByText("custom").length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText("1/1 up")).toBeInTheDocument();
  });

  it("pauses live refresh", async () => {
    const user = userEvent.setup();
    render(<BGPMonitor />);
    await screen.findAllByText("router-01");

    await user.click(screen.getByRole("button", { name: /Pause/ }));

    expect(screen.getByText("paused")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Resume/ })).toBeInTheDocument();
  });

  it("marks old session reports as stale", async () => {
    mockGet.mockResolvedValue(
      overview(new Date(Date.now() - 120_000).toISOString()),
    );
    render(<BGPMonitor />);

    expect(await screen.findByText("2m ago")).toBeInTheDocument();
  });
});