import { useEffect, useState } from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Separator } from "@/components/ui/separator";
import { clearToken, getToken, setToken, whoami } from "@/lib/api";

/*
 * Bearer-token "login" — really just stuffs the token into localStorage and
 * verifies it works by hitting /api/me.
 *
 * design-utilitarian: no logo, no marketing, no skeleton loaders.
 */
export default function LoginForm() {
  const [token, setLocal] = useState("");
  const [me, setMe] = useState<{ subject: string; method: string } | null>(null);
  const [checking, setChecking] = useState(false);

  // biome-ignore lint/correctness/useExhaustiveDependencies: one-shot on mount
  useEffect(() => {
    const t = getToken();
    if (t) {
      setLocal(t);
      void verify(t);
    }
  }, []);

  async function verify(t: string) {
    setChecking(true);
    setToken(t);
    try {
      const m = await whoami();
      setMe(m);
    } catch (err) {
      setMe(null);
      toast.error("Token check failed", { description: (err as Error).message });
    } finally {
      setChecking(false);
    }
  }

  async function onSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (!token.trim()) {
      toast.error("Token is empty");
      return;
    }
    await verify(token.trim());
    if (me || getToken()) toast.success("Token saved");
  }

  function onClear() {
    clearToken();
    setLocal("");
    setMe(null);
    toast.info("Token cleared");
  }

  return (
    <div className="max-w-xl space-y-4">
      <div className="space-y-1">
        <h1 className="text-base font-semibold">Admin token</h1>
        <p className="text-xs text-muted-foreground">
          Paste the value of <span className="mono">flyctl secrets get ADMIN_TOKEN</span> (or your
          local <span className="mono">.env.shortr-erfi.local</span>) and click save. The token is
          stored in this browser's localStorage; nothing is sent to a third party.
        </p>
      </div>

      <Separator />

      <form onSubmit={onSubmit} className="space-y-3">
        <div className="space-y-1">
          <Label htmlFor="token">Token</Label>
          <Input
            id="token"
            type="password"
            autoComplete="off"
            spellCheck={false}
            className="font-mono text-xs"
            placeholder="paste ADMIN_TOKEN here"
            value={token}
            onChange={(e) => setLocal(e.target.value)}
          />
        </div>

        <div className="flex gap-2">
          <Button type="submit" size="sm" disabled={checking}>
            {checking ? "checking…" : "save"}
          </Button>
          <Button type="button" size="sm" variant="outline" onClick={onClear}>
            clear
          </Button>
        </div>
      </form>

      {me && (
        <div className="rounded border bg-muted/40 px-3 py-2 text-xs text-muted-foreground">
          Authenticated as <span className="mono">{me.subject}</span> via{" "}
          <span className="mono">{me.method}</span>.
        </div>
      )}
    </div>
  );
}
