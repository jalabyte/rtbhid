import { useCallback, useEffect, useRef, useState } from "react";

export function errorMessage(err: unknown, fallback = "Request failed") {
  return err instanceof Error && err.message ? err.message : fallback;
}

// useResource loads data once (and every intervalMs when set), keeping the
// last good data visible if a refresh fails.
export function useResource<T>(fetcher: () => Promise<T>, intervalMs = 0) {
  const [data, setData] = useState<T | null>(null);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  const fetchRef = useRef(fetcher);
  useEffect(() => {
    fetchRef.current = fetcher;
  });

  const reload = useCallback(
    () =>
      fetchRef
        .current()
        .then((d) => {
          setData(d);
          setError("");
        })
        .catch((err) => setError(errorMessage(err, "Failed to load")))
        .finally(() => setLoading(false)),
    [],
  );

  useEffect(() => {
    reload();
    if (!intervalMs) return;
    const id = setInterval(reload, intervalMs);
    return () => clearInterval(id);
  }, [reload, intervalMs]);

  return { data, error, loading, reload };
}

// useAction runs a mutation, surfaces its error, and calls onDone (usually a
// reload) on success. Returns true when the action succeeded.
export function useAction(onDone?: () => unknown) {
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  const run = useCallback(
    async (fn: () => Promise<unknown>) => {
      setBusy(true);
      setError("");
      try {
        await fn();
        await onDone?.();
        return true;
      } catch (err) {
        setError(errorMessage(err));
        return false;
      } finally {
        setBusy(false);
      }
    },
    [onDone],
  );

  return { run, error, busy, setError };
}
