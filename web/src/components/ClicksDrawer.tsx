import { useEffect, useState } from "react";
import { toast } from "sonner";
import { Separator } from "@/components/ui/separator";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import { listClicks } from "@/lib/api";
import type { ClickEvent, DayBucket } from "@/lib/schemas";

interface Props {
  slug: string | null;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

// Tiny inline sparkline rendered with SVG. No chart library.
function Sparkline({ buckets }: { buckets: DayBucket[] }) {
  if (buckets.length === 0) {
    return <div className="text-xs text-muted-foreground">no clicks in the last 30 days</div>;
  }
  const max = Math.max(1, ...buckets.map((b) => b.hits));
  const w = 320;
  const h = 60;
  const bar = Math.max(2, Math.floor(w / Math.max(1, buckets.length)) - 1);
  return (
    <svg width={w} height={h} role="img" aria-label="clicks per day">
      {buckets.map((b, i) => {
        const bh = Math.round((b.hits / max) * (h - 2)) || 1;
        const x = i * (bar + 1);
        const y = h - bh;
        return (
          <g key={b.day}>
            <rect x={x} y={y} width={bar} height={bh} className="fill-foreground" opacity={0.8}>
              <title>{`${b.day}: ${b.hits} clicks`}</title>
            </rect>
          </g>
        );
      })}
    </svg>
  );
}

export default function ClicksDrawer({ slug, open, onOpenChange }: Props) {
  const [count, setCount] = useState<number | null>(null);
  const [events, setEvents] = useState<ClickEvent[]>([]);
  const [buckets, setBuckets] = useState<DayBucket[]>([]);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (!open || !slug) return;
    setLoading(true);
    listClicks(slug, { limit: 100, days: 30 })
      .then((r) => {
        setCount(r.count);
        setEvents(r.events);
        setBuckets(r.by_day);
      })
      .catch((err) => {
        toast.error("Failed to load clicks", { description: (err as Error).message });
        setCount(null);
        setEvents([]);
        setBuckets([]);
      })
      .finally(() => setLoading(false));
  }, [open, slug]);

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent side="right" className="w-full sm:max-w-xl">
        <SheetHeader>
          <SheetTitle className="font-mono">/{slug} · clicks</SheetTitle>
          <SheetDescription>
            {loading
              ? "loading…"
              : count == null
                ? "—"
                : `${count.toLocaleString()} total · last 30 days plotted`}
          </SheetDescription>
        </SheetHeader>

        <div className="px-4 space-y-4">
          <Sparkline buckets={buckets} />

          <Separator />

          <div className="space-y-1">
            <div className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
              Recent events
            </div>
            {events.length === 0 && !loading && (
              <div className="text-xs text-muted-foreground">no events recorded yet</div>
            )}
            <table>
              <thead>
                <tr>
                  <th className="text-left">when</th>
                  <th className="text-left">country</th>
                  <th className="text-left">region</th>
                  <th className="text-left">referrer</th>
                  <th className="text-left">UA</th>
                </tr>
              </thead>
              <tbody>
                {events.map((e, i) => (
                  // eslint-disable-next-line react/no-array-index-key
                  <tr key={`${e.TS}-${i}`}>
                    <td className="mono whitespace-nowrap">
                      {new Date(e.TS).toISOString().replace("T", " ").slice(0, 19)}
                    </td>
                    <td className="mono">{e.Country || "—"}</td>
                    <td className="mono">{e.FlyRegion || "—"}</td>
                    <td className="truncate max-w-[18ch]">{e.Referrer || "—"}</td>
                    <td className="truncate max-w-[18ch] text-muted-foreground">
                      {e.UserAgent.split(" ")[0] || "—"}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      </SheetContent>
    </Sheet>
  );
}
