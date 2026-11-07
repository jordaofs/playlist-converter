import { useCallback, useEffect, useRef, useState } from 'react';
import { api } from '../../lib/api';
import type { Conversion, Decision, Session } from '../../lib/types';
const active = new Set(['queued', 'reading', 'matching', 'publishing']);
export function useConversion() {
  const [session, setSession] = useState<Session | null>(null);
  const [job, setJob] = useState<Conversion | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState(() => {
    const result = new URLSearchParams(window.location.search).get('auth');
    return result === 'denied'
      ? 'Connection was cancelled. You can connect again when you’re ready.'
      : result === 'failed'
        ? 'We couldn’t connect your account. Please try again.'
        : '';
  });
  const [loading, setLoading] = useState(true);
  const inFlight = useRef(false);
  const creation = useRef<{ url: string; key: string } | null>(null);
  const jobId = job?.id;
  const jobStatus = job?.status;
  const pauseReason = job?.pauseReason;
  const loadSession = useCallback(async () => {
    const next = await api<Session>('/api/session');
    setSession(next);
    return next;
  }, []);
  useEffect(() => {
    let cancelled = false;
    void (async () => {
      try {
        const next = await api<Session>('/api/session');
        if (cancelled) return;
        setSession(next);
        const saved = localStorage.getItem('playlist-converter-conversion');
        const id =
          saved && next.conversionIds?.includes(saved)
            ? saved
            : next.conversionIds?.[0];
        if (id) {
          const conversion = await api<Conversion>(
            `/api/conversions/${encodeURIComponent(id)}`,
          );
          if (!cancelled) setJob(conversion);
        }
      } catch (e) {
        if (!cancelled)
          setError(
            e instanceof Error
              ? e.message
              : 'Unable to connect. Please reload.',
          );
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);
  useEffect(() => {
    if (!jobId || !jobStatus || !active.has(jobStatus) || pauseReason) return;
    let stopped = false;
    let timer: ReturnType<typeof setTimeout> | undefined;
    const controller = new AbortController();
    const poll = async () => {
      if (stopped) return;
      if (document.hidden) {
        timer = setTimeout(poll, 2500);
        return;
      }
      try {
        const next = await api<Conversion>(
          `/api/conversions/${encodeURIComponent(jobId)}`,
          { signal: controller.signal },
        );
        if (!stopped) setJob(next);
      } catch (e) {
        if (!stopped)
          setError(
            e instanceof Error ? e.message : 'Unable to refresh progress.',
          );
      }
      if (!stopped) timer = setTimeout(poll, 2000);
    };
    timer = setTimeout(poll, 1000);
    return () => {
      stopped = true;
      clearTimeout(timer);
      controller.abort();
    };
  }, [jobId, jobStatus, pauseReason]);
  const run = async (action: () => Promise<void>) => {
    if (inFlight.current) return;
    inFlight.current = true;
    setBusy(true);
    setError('');
    try {
      await action();
    } catch (e) {
      setError(
        e instanceof Error
          ? e.message
          : 'Something went wrong. Please try again.',
      );
      if (job) {
        try {
          setJob(
            await api<Conversion>(
              `/api/conversions/${encodeURIComponent(job.id)}`,
            ),
          );
        } catch {
          /* Preserve original actionable error and last known state. */
        }
      }
    } finally {
      inFlight.current = false;
      setBusy(false);
    }
  };
  const create = (sourceUrl: string) =>
    run(async () => {
      if (creation.current?.url !== sourceUrl)
        creation.current = { url: sourceUrl, key: crypto.randomUUID() };
      const next = await api<Conversion>('/api/conversions', {
        method: 'POST',
        body: { sourceUrl },
        csrf: session?.csrfToken,
        key: creation.current.key,
      });
      setJob(next);
      localStorage.setItem('playlist-converter-conversion', next.id);
    });
  const mutate = (suffix: string, body?: unknown, method = 'POST') =>
    run(async () => {
      if (!job) return;
      const next = await api<Conversion>(
        `/api/conversions/${encodeURIComponent(job.id)}/${suffix}`,
        {
          method,
          body,
          csrf: session?.csrfToken,
          key:
            suffix === 'publish'
              ? `publish-${job.id}-${job.revision}`
              : undefined,
        },
      );
      setJob(next);
    });
  const disconnect = (provider: string) =>
    run(async () => {
      await api(`/api/connections/${provider}`, {
        method: 'DELETE',
        csrf: session?.csrfToken,
      });
      await loadSession();
      if (job)
        setJob(
          await api<Conversion>(
            `/api/conversions/${encodeURIComponent(job.id)}`,
          ),
        );
    });
  const clearData = () =>
    run(async () => {
      await api('/api/session', { method: 'DELETE', csrf: session?.csrfToken });
      localStorage.removeItem('playlist-converter-conversion');
      setJob(null);
      await loadSession();
    });
  const startNew = () => {
    creation.current = null;
    setJob(null);
    setError('');
    localStorage.removeItem('playlist-converter-conversion');
  };
  return {
    session,
    job,
    busy,
    error,
    loading,
    create,
    disconnect,
    clearData,
    startNew,
    decide: (decision: Decision) =>
      mutate(
        'decisions',
        { revision: job?.revision, decisions: [decision] },
        'PATCH',
      ),
    publish: (name: string) =>
      mutate('publish', { revision: job?.revision, name }),
    resume: () => mutate('resume'),
    cancel: () => mutate('cancel'),
  };
}
