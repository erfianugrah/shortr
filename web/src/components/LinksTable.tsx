import { useCallback, useEffect, useState } from "react";
import { createLink, deleteLink, listLinks } from "@/lib/api";
import type { Link } from "@/lib/schemas";

/*
 * Dense, server-data-driven table. design-utilitarian:
 *   - no card grid
 *   - no animation
 *   - inline create form above the table
 *   - delete is a one-click confirm
 */
export default function LinksTable() {
  const [links, setLinks] = useState<Link[]>([]);
  const [loading, setLoading] = useState(true);
  const [err, setErr] = useState<string>("");
  const [target, setTarget] = useState("");
  const [slug, setSlug] = useState("");

  const refresh = useCallback(async () => {
    setLoading(true);
    setErr("");
    try {
      const r = await listLinks();
      setLinks(r.links);
    } catch (e: unknown) {
      setErr((e as Error).message);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    refresh();
  }, [refresh]);

  async function onCreate(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setErr("");
    try {
      await createLink({
        target_url: target,
        slug: slug,
      });
      setTarget("");
      setSlug("");
      await refresh();
    } catch (e: unknown) {
      setErr((e as Error).message);
    }
  }

  async function onDelete(s: string) {
    if (!confirm(`delete /${s}?`)) return;
    try {
      await deleteLink(s);
      await refresh();
    } catch (e: unknown) {
      setErr((e as Error).message);
    }
  }

  return (
    <div className="space-y-3">
      <form onSubmit={onCreate} className="flex gap-2 items-center">
        <input
          required
          type="url"
          className="flex-1 border border-border rounded px-2 py-1 text-xs"
          placeholder="https://target.example.com/long/url"
          value={target}
          onChange={(e) => setTarget(e.target.value)}
        />
        <input
          type="text"
          className="w-32 border border-border rounded px-2 py-1 font-mono text-xs"
          placeholder="slug (auto)"
          value={slug}
          onChange={(e) => setSlug(e.target.value)}
        />
        <button
          type="submit"
          className="border border-border rounded px-3 py-1 text-xs hover:bg-muted"
        >
          create
        </button>
      </form>

      {err && <div className="text-xs text-destructive">error: {err}</div>}

      <table>
        <thead>
          <tr>
            <th>slug</th>
            <th>target</th>
            <th className="text-right">clicks</th>
            <th>expires</th>
            <th>created</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          {loading && (
            <tr>
              <td colSpan={6} className="text-muted-foreground">
                loading…
              </td>
            </tr>
          )}
          {!loading && links.length === 0 && (
            <tr>
              <td colSpan={6} className="text-muted-foreground">
                no links yet
              </td>
            </tr>
          )}
          {links.map((l) => (
            <tr key={l.slug}>
              <td className="mono">
                <a href={`/${l.slug}`} target="_blank" rel="noreferrer">
                  /{l.slug}
                </a>
              </td>
              <td className="truncate max-w-[40ch]">
                <a href={l.target_url} target="_blank" rel="noreferrer">
                  {l.target_url}
                </a>
              </td>
              <td className="text-right mono">{l.click_count}</td>
              <td className="mono">
                {l.expires_at ? new Date(l.expires_at).toISOString().slice(0, 10) : "–"}
              </td>
              <td className="mono">{new Date(l.created_at).toISOString().slice(0, 10)}</td>
              <td className="text-right">
                <button
                  type="button"
                  onClick={() => onDelete(l.slug)}
                  className="text-xs text-destructive hover:underline"
                >
                  delete
                </button>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
