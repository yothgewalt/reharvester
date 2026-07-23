

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
