import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiError, API_URL, authFetch, login } from "./api";

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("login", () => {
  it("posts the credentials as JSON to the login endpoint", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ token: "jwt-abc" }), { status: 200 }),
    );
    vi.stubGlobal("fetch", fetchMock);

    const result = await login("ana@example.com", "hunter22");

    expect(result).toEqual({ token: "jwt-abc" });
    // The assert is on the INTERACTION, not just the return value: what URL,
    // method and body login actually sent fetch.
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(fetchMock).toHaveBeenCalledWith(`${API_URL}/api/auth/login`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ email: "ana@example.com", password: "hunter22" }),
    });
  });

  it("rejects with the server's error message when the login is refused", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValue(new Response(JSON.stringify({ error: "invalid credentials" }), { status: 401 }));
    vi.stubGlobal("fetch", fetchMock);

    await expect(login("ana@example.com", "wrong")).rejects.toMatchObject({
      status: 401,
      message: "invalid credentials",
    });
  });
});

describe("authFetch", () => {
  it("attaches the bearer token to every request it makes", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response("{}", { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);

    await authFetch("/api/reports", "jwt-abc");

    expect(fetchMock).toHaveBeenCalledWith(`${API_URL}/api/reports`, {
      headers: { Authorization: "Bearer jwt-abc" },
    });
  });

  it("throws a session-expired ApiError on a 401, without reading the body", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(null, { status: 401 }));
    vi.stubGlobal("fetch", fetchMock);

    await expect(authFetch("/api/reports", "stale-token")).rejects.toThrow(ApiError);
    await expect(authFetch("/api/reports", "stale-token")).rejects.toMatchObject({
      status: 401,
      message: "session expired",
    });
  });
});
