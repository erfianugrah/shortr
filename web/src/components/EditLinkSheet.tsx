import { useEffect, useState } from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Separator } from "@/components/ui/separator";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import { deleteLink, updateLink } from "@/lib/api";
import type { Link } from "@/lib/schemas";

interface Props {
  link: Link | null;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onSaved: (updated: Link) => void;
  onDeleted: (slug: string) => void;
}

function toDateInput(iso: string | null | undefined): string {
  if (!iso) return "";
  return new Date(iso).toISOString().slice(0, 10);
}

export default function EditLinkSheet({ link, open, onOpenChange, onSaved, onDeleted }: Props) {
  const [target, setTarget] = useState("");
  const [note, setNote] = useState("");
  const [expiresDate, setExpiresDate] = useState("");
  const [maxClicks, setMaxClicks] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [clearPassword, setClearPassword] = useState(false);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (link) {
      setTarget(link.target_url);
      setNote(link.note);
      setExpiresDate(toDateInput(link.expires_at));
      setMaxClicks(link.max_clicks ? String(link.max_clicks) : "");
      setNewPassword("");
      setClearPassword(false);
    }
  }, [link]);

  async function onSave() {
    if (!link) return;
    if (!target.trim()) {
      toast.error("Target URL required");
      return;
    }
    let max: number | null = null;
    if (maxClicks) {
      const parsed = Number.parseInt(maxClicks, 10);
      if (Number.isNaN(parsed) || parsed <= 0) {
        toast.error("max_clicks must be a positive integer");
        return;
      }
      max = parsed;
    }

    setSaving(true);
    try {
      const updated = await updateLink(link.slug, {
        target_url: target.trim(),
        note: note,
        expires_at: expiresDate ? new Date(`${expiresDate}T23:59:59Z`).toISOString() : null,
        clear_expiry: !expiresDate,
        max_clicks: max,
        clear_max_clicks: !maxClicks,
        password: clearPassword ? "" : newPassword || undefined,
      });
      toast.success(`Updated /${updated.slug}`);
      onSaved(updated);
      onOpenChange(false);
    } catch (err) {
      toast.error("Update failed", { description: (err as Error).message });
    } finally {
      setSaving(false);
    }
  }

  async function onDelete() {
    if (!link) return;
    if (!confirm(`Delete /${link.slug}? Clicks will be lost.`)) return;
    setSaving(true);
    try {
      await deleteLink(link.slug);
      toast.success(`Deleted /${link.slug}`);
      onDeleted(link.slug);
      onOpenChange(false);
    } catch (err) {
      toast.error("Delete failed", { description: (err as Error).message });
    } finally {
      setSaving(false);
    }
  }

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent side="right" className="w-full sm:max-w-md">
        <SheetHeader>
          <SheetTitle className="font-mono">/{link?.slug}</SheetTitle>
          <SheetDescription>
            Created {link ? new Date(link.created_at).toISOString().slice(0, 10) : "—"} ·{" "}
            {link?.click_count ?? 0} clicks
          </SheetDescription>
        </SheetHeader>

        <Separator className="my-3" />

        <div className="space-y-3 px-4 pb-4">
          <div className="space-y-1">
            <Label htmlFor="edit-target" className="text-xs">
              Target URL
            </Label>
            <Input
              id="edit-target"
              type="url"
              value={target}
              onChange={(e) => setTarget(e.target.value)}
            />
          </div>

          <div className="space-y-1">
            <Label htmlFor="edit-note" className="text-xs">
              Note
            </Label>
            <Input id="edit-note" value={note} onChange={(e) => setNote(e.target.value)} />
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-1">
              <Label htmlFor="edit-expires" className="text-xs">
                Expires
              </Label>
              <Input
                id="edit-expires"
                type="date"
                value={expiresDate}
                onChange={(e) => setExpiresDate(e.target.value)}
              />
            </div>
            <div className="space-y-1">
              <Label htmlFor="edit-max" className="text-xs">
                Max clicks
              </Label>
              <Input
                id="edit-max"
                type="number"
                min={1}
                placeholder="unlimited"
                value={maxClicks}
                onChange={(e) => setMaxClicks(e.target.value)}
              />
            </div>
          </div>

          <div className="space-y-1">
            <Label htmlFor="edit-password" className="text-xs">
              Password
            </Label>
            <Input
              id="edit-password"
              type="password"
              autoComplete="new-password"
              placeholder={link?.has_password ? "(set — leave blank to keep)" : "(none)"}
              value={newPassword}
              onChange={(e) => setNewPassword(e.target.value)}
              disabled={clearPassword}
            />
            {link?.has_password && (
              <label className="flex items-center gap-2 text-xs text-muted-foreground">
                <input
                  type="checkbox"
                  checked={clearPassword}
                  onChange={(e) => {
                    setClearPassword(e.target.checked);
                    if (e.target.checked) setNewPassword("");
                  }}
                />
                clear existing password
              </label>
            )}
          </div>
        </div>

        <SheetFooter className="flex-row justify-between gap-2">
          <Button variant="destructive" onClick={onDelete} disabled={saving} size="sm">
            delete
          </Button>
          <div className="flex gap-2">
            <Button variant="outline" onClick={() => onOpenChange(false)} size="sm">
              cancel
            </Button>
            <Button onClick={onSave} disabled={saving} size="sm">
              {saving ? "saving…" : "save"}
            </Button>
          </div>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  );
}
