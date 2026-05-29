/*
 * Tiny typed fetch wrapper around the shortr API.
 * Bearer token comes from localStorage (set on the /login page).
 */

import {
  type CreateLinkInput,
  createLinkRequestSchema,
  linkSchema,
  listClicksResponseSchema,
  listLinksResponseSchema,
  type UpdateLinkRequest,
} from "./schemas";

const TOKEN_KEY = "shortr.adminToken";

export function setToken(t: string) {
  localStorage.setItem(TOKEN_KEY, t);
}
export function getToken(): string {
  return localStorage.getItem(TOKEN_KEY) ?? "";
}
export function clearToken() {
  localStorage.removeItem(TOKEN_KEY);
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers);
  headers.set("Authorization", `Bearer ${getToken()}`);
  if (init.body) headers.set("Content-Type", "application/json");

  const res = await fetch(path, { ...init, headers });
  if (!res.ok) {
    const text = await res.text().catch(() => "");
    let detail = text;
    try {
      const obj = JSON.parse(text);
      if (obj?.detail) detail = obj.detail;
      else if (obj?.title) detail = obj.title;
    } catch {
      /* keep raw text */
    }
    throw new Error(`${res.status} ${res.statusText}${detail ? `: ${detail}` : ""}`);
  }
  if (res.status === 204) return undefined as T;
  return (await res.json()) as T;
}

export async function listLinks(cursor?: string, limit = 50) {
  const q = new URLSearchParams();
  if (cursor) q.set("cursor", cursor);
  q.set("limit", String(limit));
  const raw = await request<unknown>(`/api/links?${q}`);
  return listLinksResponseSchema.parse(raw);
}

export async function createLink(input: CreateLinkInput) {
  // Apply defaults / strip unknowns before sending.
  const parsed = createLinkRequestSchema.parse(input);
  const raw = await request<unknown>("/api/links", {
    method: "POST",
    body: JSON.stringify(parsed),
  });
  return linkSchema.parse(raw);
}

export async function getLink(slug: string) {
  const raw = await request<unknown>(`/api/links/${encodeURIComponent(slug)}`);
  return linkSchema.parse(raw);
}

export async function updateLink(slug: string, input: UpdateLinkRequest) {
  const raw = await request<unknown>(`/api/links/${encodeURIComponent(slug)}`, {
    method: "PATCH",
    body: JSON.stringify(input),
  });
  return linkSchema.parse(raw);
}

export async function deleteLink(slug: string) {
  await request<void>(`/api/links/${encodeURIComponent(slug)}`, { method: "DELETE" });
}

export async function listClicks(
  slug: string,
  opts: { cursor?: string; limit?: number; days?: number } = {},
) {
  const q = new URLSearchParams();
  if (opts.cursor) q.set("cursor", opts.cursor);
  q.set("limit", String(opts.limit ?? 100));
  q.set("days", String(opts.days ?? 30));
  const raw = await request<unknown>(`/api/links/${encodeURIComponent(slug)}/clicks?${q}`);
  return listClicksResponseSchema.parse(raw);
}

export async function whoami() {
  return request<{ subject: string; method: string }>("/api/me");
}

/* Convenience: where the dashboard thinks the public base URL is.
 * Used to render copy-paste-able short URLs in the table. We don't have a
 * /api/config endpoint, so just use window.origin — fine since the dashboard
 * is served from the same origin as the redirect path. */
export function publicBaseURL(): string {
  if (typeof window === "undefined") return "";
  return window.location.origin;
}
