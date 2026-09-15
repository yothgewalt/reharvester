

type Listener = () => void;

let listener: Listener | null = null;

export function onBackendFailure(fn: Listener): () => void {
  listener = fn;
  return () => {
    if (listener === fn) listener = null;
  };
}

export function reportBackendFailure(): void {
  listener?.();
}

/** Ask health.ts to re-ping /health now, e.g. after a settings change that
 *  could flip the LLM badge. Does not itself mark the backend as failed. */
export function requestHealthPing(): void {
  listener?.();
}
