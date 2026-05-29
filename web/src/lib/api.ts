/*
 * Tiny typed fetch wrapper around the shortr API.
 * Bearer token comes from localStorage (set on the /login page).
 */

import type { z } from "zod";
import {
  createLinkRequestSchema,
  linkSchema,
  listClicksResponseSchema,
  listLinksResponseSchema,
  type UpdateLinkRequest,
} from "./schemas";

// Use the *input* type so callers don't have to fill in every defaulted field.
export type CreateLinkInput = z.input<typeof createLinkRequestSchema>;

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
    throw new Error(`${res.status} ${res.statusText}${text ? `: ${text}` : ""}`);
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
  // Apply schema defaults before sending so the server gets a complete shape.
  const parsed = createLinkRequestSchema.parse(input);
  const raw = await request<unknown>("/api/links", {
    method: "POST",
    body: JSON.stringify(parsed),
  });
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

export async function listClicks(slug: string, cursor?: string, limit = 100) {
  const q = new URLSearchParams();
  if (cursor) q.set("cursor", cursor);
  q.set("limit", String(limit));
  const raw = await request<unknown>(`/api/links/${encodeURIComponent(slug)}/clicks?${q}`);
  return listClicksResponseSchema.parse(raw);
}

export async function whoami() {
  return request<{ subject: string; method: string }>("/api/me");
}
