import { useState } from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { createLink } from "@/lib/api";
import type { Link } from "@/lib/schemas";

interface Props {
  onCreated: (link: Link) => void;
}

/*
 * Inline create form. Sits above the table.
 *
 * Required: target_url
 * Optional: slug, note, expires_at (date input → midnight UTC), password,
 *           max_clicks
 */
export default function CreateLinkForm({ onCreated }: Props) {
  const [target, setTarget] = useState("");
  const [slug, setSlug] = useState("");
  const [showOptional, setShowOptional] = useState(false);
  const [note, setNote] = useState("");
  const [expiresDate, setExpiresDate] = useState(""); // YYYY-MM-DD
  const [password, setPassword] = useState("");
  const [maxClicks, setMaxClicks] = useState<string>("");
  const [submitting, setSubmitting] = useState(false);

  function reset() {
    setTarget("");
    setSlug("");
    setNote("");
    setExpiresDate("");
    setPassword("");
    setMaxClicks("");
    setShowOptional(false);
  }

  async function onSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (!target.trim()) {
      toast.error("Target URL is required");
      return;
    }
    setSubmitting(true);
    try {
      const expires_at = expiresDate ? new Date(`${expiresDate}T23:59:59Z`).toISOString() : null;
      let max: number | null = null;
      if (maxClicks) {
        const parsed = Number.parseInt(maxClicks, 10);
        if (Number.isNaN(parsed) || parsed <= 0) {
          toast.error("max_clicks must be a positive integer");
          return;
        }
        max = parsed;
      }
      const link = await createLink({
        target_url: target.trim(),
        slug: slug.trim(),
        note: note.trim(),
        expires_at,
        password: password,
        max_clicks: max,
      });
      toast.success(`Created /${link.slug}`);
      onCreated(link);
      reset();
    } catch (err) {
      toast.error("Create failed", { description: (err as Error).message });
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <form onSubmit={onSubmit} className="rounded border bg-card p-3 space-y-3">
      <div className="grid gap-3 sm:grid-cols-[1fr_140px_auto]">
        <div className="space-y-1">
          <Label htmlFor="target" className="text-xs">
            Target URL <span className="text-destructive">*</span>
          </Label>
          <Input
            id="target"
            required
            type="url"
            placeholder="https://target.example.com/long/path"
            value={target}
            onChange={(e) => setTarget(e.target.value)}
          />
        </div>
        <div className="space-y-1">
          <Label htmlFor="slug" className="text-xs">
            Slug
          </Label>
          <Input
            id="slug"
            className="font-mono"
            placeholder="auto"
            value={slug}
            onChange={(e) => setSlug(e.target.value)}
          />
        </div>
        <div className="flex items-end gap-2">
          <Button type="submit" disabled={submitting}>
            {submitting ? "creating…" : "create"}
          </Button>
          <Button
            type="button"
            variant="ghost"
            size="sm"
            onClick={() => setShowOptional((v) => !v)}
          >
            {showOptional ? "− options" : "+ options"}
          </Button>
        </div>
      </div>

      {showOptional && (
        <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4 border-t pt-3">
          <div className="space-y-1">
            <Label htmlFor="note" className="text-xs">
              Note
            </Label>
            <Input
              id="note"
              placeholder="(optional, internal)"
              value={note}
              onChange={(e) => setNote(e.target.value)}
            />
          </div>
          <div className="space-y-1">
            <Label htmlFor="expires" className="text-xs">
              Expires
            </Label>
            <Input
              id="expires"
              type="date"
              value={expiresDate}
              onChange={(e) => setExpiresDate(e.target.value)}
            />
          </div>
          <div className="space-y-1">
            <Label htmlFor="max" className="text-xs">
              Max clicks
            </Label>
            <Input
              id="max"
              type="number"
              min={1}
              placeholder="unlimited"
              value={maxClicks}
              onChange={(e) => setMaxClicks(e.target.value)}
            />
          </div>
          <div className="space-y-1">
            <Label htmlFor="password" className="text-xs">
              Password
            </Label>
            <Input
              id="password"
              type="password"
              autoComplete="new-password"
              placeholder="(optional)"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
            />
          </div>
        </div>
      )}
    </form>
  );
}
