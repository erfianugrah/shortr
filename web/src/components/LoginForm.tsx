import { useEffect, useState } from "react";
import { clearToken, getToken, setToken, whoami } from "@/lib/api";

/*
 * Bearer-token "login" — really just stuffs the token into localStorage and
 * verifies it works by hitting /api/me.
 *
 * design-utilitarian: no logo, no copy, no skeleton loaders. One input, one
 * button, one status line.
 */
export default function LoginForm() {
  const [token, setLocal] = useState("");
  const [status, setStatus] = useState<string>("");
  const [me, setMe] = useState<{ subject: string; method: string } | null>(null);

  useEffect(() => {
    const t = getToken();
    if (t) {
      setLocal(t);
      whoami()
        .then(setMe)
        .catch((e: Error) => setStatus(e.message));
    }
  }, []);

  async function onSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setStatus("verifying…");
    setToken(token);
    try {
      const m = await whoami();
      setMe(m);
      setStatus("ok");
    } catch (err: unknown) {
      const e = err as Error;
      setMe(null);
      setStatus(e.message);
    }
  }

  function onClear() {
    clearToken();
    setLocal("");
    setMe(null);
    setStatus("");
  }

  return (
    <div className="space-y-3 max-w-md">
      <form onSubmit={onSubmit} className="flex gap-2">
        <input
          type="password"
          autoComplete="off"
          className="flex-1 border border-border rounded px-2 py-1 font-mono text-xs"
          placeholder="ADMIN_TOKEN"
          value={token}
          onChange={(e) => setLocal(e.target.value)}
        />
        <button
          type="submit"
          className="border border-border rounded px-3 py-1 text-xs hover:bg-muted"
        >
          save
        </button>
        <button
          type="button"
          onClick={onClear}
          className="border border-border rounded px-3 py-1 text-xs hover:bg-muted"
        >
          clear
        </button>
      </form>
      {status && <div className="text-xs text-muted-foreground">status: {status}</div>}
      {me && (
        <div className="text-xs text-muted-foreground">
          authenticated as <span className="mono">{me.subject}</span> via{" "}
          <span className="mono">{me.method}</span>
        </div>
      )}
    </div>
  );
}
