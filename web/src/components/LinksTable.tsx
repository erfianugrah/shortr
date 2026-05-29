import {
  BarChart3Icon,
  CopyIcon,
  LockIcon,
  PencilIcon,
  RefreshCwIcon,
  TimerIcon,
  TrashIcon,
} from "lucide-react";
import { useCallback, useEffect, useState } from "react";
import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { deleteLink, listLinks, publicBaseURL } from "@/lib/api";
import type { Link } from "@/lib/schemas";
import ClicksDrawer from "./ClicksDrawer";
import CreateLinkForm from "./CreateLinkForm";
import EditLinkSheet from "./EditLinkSheet";

/*
 * Dense, server-data-driven table. design-utilitarian:
 *   - no card grid
 *   - no animation tax
 *   - per-row actions: copy short URL, view clicks, edit, delete
 *   - inline create form above
 *   - edit/clicks open right-side sheets, not modal dialogs (less obscuring)
 */
export default function LinksTable() {
  const [links, setLinks] = useState<Link[]>([]);
  const [loading, setLoading] = useState(true);

  const [editing, setEditing] = useState<Link | null>(null);
  const [editOpen, setEditOpen] = useState(false);

  const [clicksSlug, setClicksSlug] = useState<string | null>(null);
  const [clicksOpen, setClicksOpen] = useState(false);

  const refresh = useCallback(async () => {
    setLoading(true);
    try {
      const r = await listLinks();
      setLinks(r.links);
    } catch (err) {
      toast.error("Failed to load links", { description: (err as Error).message });
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    refresh();
  }, [refresh]);

  function shortURL(slug: string) {
    return `${publicBaseURL()}/${slug}`;
  }

  async function onCopy(slug: string) {
    try {
      await navigator.clipboard.writeText(shortURL(slug));
      toast.success(`copied ${shortURL(slug)}`);
    } catch {
      toast.error("clipboard write failed");
    }
  }

  async function onQuickDelete(slug: string) {
    if (!confirm(`delete /${slug}? clicks will be lost.`)) return;
    try {
      await deleteLink(slug);
      setLinks((prev) => prev.filter((l) => l.slug !== slug));
      toast.success(`deleted /${slug}`);
    } catch (err) {
      toast.error("delete failed", { description: (err as Error).message });
    }
  }

  function openEdit(link: Link) {
    setEditing(link);
    setEditOpen(true);
  }

  function openClicks(slug: string) {
    setClicksSlug(slug);
    setClicksOpen(true);
  }

  return (
    <div className="space-y-4">
      <CreateLinkForm onCreated={(l) => setLinks((prev) => [l, ...prev])} />

      <div className="flex items-center justify-between">
        <div className="text-xs text-muted-foreground">
          {loading ? "loading…" : `${links.length} link${links.length === 1 ? "" : "s"}`}
        </div>
        <Button variant="ghost" size="sm" onClick={() => refresh()} disabled={loading}>
          <RefreshCwIcon className="size-3" />
          refresh
        </Button>
      </div>

      <div className="rounded border overflow-x-auto">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="w-[110px]">slug</TableHead>
              <TableHead>target</TableHead>
              <TableHead className="w-[80px] text-right">clicks</TableHead>
              <TableHead className="w-[100px]">expires</TableHead>
              <TableHead className="w-[60px]">flags</TableHead>
              <TableHead className="w-[100px]">created</TableHead>
              <TableHead className="w-[140px] text-right">actions</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {!loading && links.length === 0 && (
              <TableRow>
                <TableCell colSpan={7} className="text-muted-foreground text-xs">
                  no links yet — create one above
                </TableCell>
              </TableRow>
            )}
            {links.map((l) => (
              <TableRow key={l.slug}>
                <TableCell className="mono">
                  <a
                    href={`/${l.slug}`}
                    target="_blank"
                    rel="noreferrer"
                    className="hover:underline"
                    title={shortURL(l.slug)}
                  >
                    /{l.slug}
                  </a>
                </TableCell>
                <TableCell className="truncate max-w-[40ch]">
                  <a
                    href={l.target_url}
                    target="_blank"
                    rel="noreferrer"
                    className="hover:underline"
                    title={l.target_url}
                  >
                    {l.target_url}
                  </a>
                  {l.note && <div className="text-xs text-muted-foreground truncate">{l.note}</div>}
                </TableCell>
                <TableCell className="text-right mono">
                  {l.click_count.toLocaleString()}
                  {l.max_clicks != null && (
                    <span className="text-muted-foreground"> / {l.max_clicks}</span>
                  )}
                </TableCell>
                <TableCell className="mono">
                  {l.expires_at ? new Date(l.expires_at).toISOString().slice(0, 10) : "–"}
                </TableCell>
                <TableCell>
                  <div className="flex gap-1">
                    {l.has_password && (
                      <Badge variant="outline" title="password protected" className="px-1">
                        <LockIcon className="size-3" />
                      </Badge>
                    )}
                    {l.max_clicks != null && (
                      <Badge variant="outline" title="click cap set" className="px-1">
                        <TimerIcon className="size-3" />
                      </Badge>
                    )}
                  </div>
                </TableCell>
                <TableCell className="mono text-muted-foreground">
                  {new Date(l.created_at).toISOString().slice(0, 10)}
                </TableCell>
                <TableCell>
                  <div className="flex justify-end gap-1">
                    <Button
                      size="icon"
                      variant="ghost"
                      title="copy short URL"
                      onClick={() => onCopy(l.slug)}
                    >
                      <CopyIcon className="size-3" />
                    </Button>
                    <Button
                      size="icon"
                      variant="ghost"
                      title="view clicks"
                      onClick={() => openClicks(l.slug)}
                    >
                      <BarChart3Icon className="size-3" />
                    </Button>
                    <Button size="icon" variant="ghost" title="edit" onClick={() => openEdit(l)}>
                      <PencilIcon className="size-3" />
                    </Button>
                    <Button
                      size="icon"
                      variant="ghost"
                      title="delete"
                      onClick={() => onQuickDelete(l.slug)}
                    >
                      <TrashIcon className="size-3 text-destructive" />
                    </Button>
                  </div>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>

      <EditLinkSheet
        link={editing}
        open={editOpen}
        onOpenChange={setEditOpen}
        onSaved={(updated) =>
          setLinks((prev) => prev.map((l) => (l.slug === updated.slug ? updated : l)))
        }
        onDeleted={(slug) => setLinks((prev) => prev.filter((l) => l.slug !== slug))}
      />

      <ClicksDrawer slug={clicksSlug} open={clicksOpen} onOpenChange={setClicksOpen} />
    </div>
  );
}
