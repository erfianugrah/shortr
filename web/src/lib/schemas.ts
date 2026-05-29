import { z } from "zod";

/*
 * Shared zod schemas. These mirror the Go DTOs in internal/api/links.go.
 * When you change a field, update both. Add a runtime sanity check in dev
 * by hitting GET /api/me before round-tripping a real link.
 */

export const linkSchema = z.object({
  slug: z.string().min(1).max(64),
  target_url: z.url(),
  created_at: z.iso.datetime(),
  expires_at: z.iso.datetime().nullable(),
  has_password: z.boolean(),
  max_clicks: z.number().int().positive().nullable(),
  click_count: z.number().int().nonnegative(),
  note: z.string(),
  created_by: z.string(),
});
export type Link = z.infer<typeof linkSchema>;

export const listLinksResponseSchema = z.object({
  links: z.array(linkSchema),
});
export type ListLinksResponse = z.infer<typeof listLinksResponseSchema>;

export const createLinkRequestSchema = z.object({
  slug: z
    .string()
    .regex(/^[A-Za-z0-9_-]*$/, "letters, digits, _ and - only")
    .max(64)
    .optional()
    .default(""),
  target_url: z
    .url()
    .refine((u) => u.startsWith("http://") || u.startsWith("https://"), "must be http(s)://"),
  expires_at: z.iso.datetime().nullable().optional(),
  password: z.string().optional().default(""),
  max_clicks: z.number().int().positive().nullable().optional(),
  note: z.string().max(280).optional().default(""),
});
export type CreateLinkRequest = z.infer<typeof createLinkRequestSchema>;

export const updateLinkRequestSchema = z.object({
  target_url: z.url().optional(),
  expires_at: z.iso.datetime().nullable().optional(),
  clear_expiry: z.boolean().optional(),
  password: z.string().optional(),
  max_clicks: z.number().int().positive().nullable().optional(),
  clear_max_clicks: z.boolean().optional(),
  note: z.string().max(280).optional(),
});
export type UpdateLinkRequest = z.infer<typeof updateLinkRequestSchema>;

export const clickEventSchema = z.object({
  Slug: z.string(),
  TS: z.iso.datetime(),
  Country: z.string(),
  UserAgent: z.string(),
  Referrer: z.string(),
  IPHash: z.string(),
  FlyRegion: z.string(),
});
export type ClickEvent = z.infer<typeof clickEventSchema>;

export const listClicksResponseSchema = z.object({
  slug: z.string(),
  count: z.number(),
  events: z.array(clickEventSchema),
});
