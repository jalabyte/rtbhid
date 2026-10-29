import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import Whitelist from "./Whitelist";
import { api } from "@/lib/api";

vi.mock("@/lib/api", () => ({
  api: {
    getWhitelist: vi.fn(),
    addWhitelistASN: vi.fn(),
    deleteWhitelistASN: vi.fn(),
    toggleWhitelistASN: vi.fn(),
    importIndonesianASNs: vi.fn(),
    setAutoAccept: vi.fn(),
  },
}));

const mockGet = vi.mocked(api.getWhitelist);
const mockAdd = vi.mocked(api.addWhitelistASN);
const mockImport = vi.mocked(api.importIndonesianASNs);

const baseWhitelist = {
  items: [
    {
      id: "asn_1",
      asn: 7713,
      name: "PT Contoh",
      country: "ID",
      source: "rir",
      enabled: true,
      created_at: "2026-01-01T00:00:00Z",
    },
  ],
  count: 1,
  auto_accept: true,
  strict_id: true,
  id_registry_count: 4032,
};

describe("Whitelist Page", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockGet.mockResolvedValue(baseWhitelist);
    mockAdd.mockResolvedValue({
      id: "asn_2",
      asn: 65002,
      name: "Test",
      country: "NON-ID",
      source: "manual",
      enabled: true,
      created_at: "2026-01-01T00:00:00Z",
    });
    mockImport.mockResolvedValue({
      created: 4032,
      total: 4032,
      count: 4032,
      id_registry_count: 4032,
      warnings: [],
    });
  });

  it("renders items, registry count, and strict policy", async () => {
    render(<Whitelist />);

    expect(await screen.findByText("AS7713")).toBeInTheDocument();
    expect(screen.getByText("PT Contoh")).toBeInTheDocument();
    expect(screen.getByText("4032 ID ASNs cached")).toBeInTheDocument();
    expect(screen.getByText("strict Indonesian only")).toBeInTheDocument();
  });

  it("adds an ASN with the non-Indonesian override", async () => {
    const user = userEvent.setup();
    render(<Whitelist />);
    await screen.findByText("AS7713");

    await user.type(screen.getByPlaceholderText(/ASN \(e.g. 7713\)/), "65002");
    await user.type(screen.getByPlaceholderText(/Name \(optional\)/), "Test");
    await user.click(screen.getByLabelText(/Allow non-Indonesian ASN/));
    await user.click(screen.getByRole("button", { name: /Add ASN/ }));

    await waitFor(() => {
      expect(mockAdd).toHaveBeenCalledWith({
        asn: 65002,
        name: "Test",
        allow_non_id: true,
      });
    });
  });

  it("shows a validation error from the API", async () => {
    const user = userEvent.setup();
    mockAdd.mockRejectedValueOnce(
      new Error("AS65002 is not registered to Indonesia in RIR delegation data"),
    );
    render(<Whitelist />);
    await screen.findByText("AS7713");

    await user.type(screen.getByPlaceholderText(/ASN \(e.g. 7713\)/), "65002");
    await user.click(screen.getByRole("button", { name: /Add ASN/ }));

    expect(
      await screen.findByText(/not registered to Indonesia/),
    ).toBeInTheDocument();
  });

  it("reports partial RIR import warnings", async () => {
    const user = userEvent.setup();
    mockImport.mockResolvedValueOnce({
      created: 4032,
      total: 4032,
      count: 4032,
      id_registry_count: 4032,
      warnings: ["https://ftp.ripe.net: timeout"],
    });
    render(<Whitelist />);
    await screen.findByText("AS7713");

    await user.click(
      screen.getByRole("button", { name: /Import Indonesian ASNs/ }),
    );

    expect(await screen.findByText(/1 source\(s\) failed/)).toBeInTheDocument();
  });
});