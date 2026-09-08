/**
 * Where the browser should reach the API.
 *
 * Set NEXT_PUBLIC_API_BASE_URL to override. Otherwise the origin is inferred:
 * a packaged install serves this UI from the Go binary, so the API is the page's
 * own origin and hardcoding a port would break every address but :8000. The
 * Next dev server on :3000 is the one case where they differ, and it falls back
 * to the API's default port.
 */
const DEV_SERVER_PORT = "3000";
const DEFAULT_API_ORIGIN = "http://localhost:8000";

function resolveApiBase(): string {
  const configured = process.env.NEXT_PUBLIC_API_BASE_URL;
  if (configured) return configured.replace(/\/$/, "");
  // No window during the static export's prerender; the value below is never
  // used then, because every call happens in the browser.
  if (typeof window === "undefined") return DEFAULT_API_ORIGIN;
  if (window.location.port === DEV_SERVER_PORT) return DEFAULT_API_ORIGIN;
  return window.location.origin;
}

export const API_BASE = resolveApiBase();
export const WS_BASE = API_BASE.replace(/^http/, "ws");
