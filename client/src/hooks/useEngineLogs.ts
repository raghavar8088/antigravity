import { useEffect, useState } from "react";
import { resolveEngineApiUrl } from "@/lib/broker/engineApi";

const API_URL = resolveEngineApiUrl();

export default function useEngineLogs(refreshKey = 0, enabled = true) {
  const [logs, setLogs] = useState<string[]>([]);

  useEffect(() => {
    if (!enabled) return;
    let cancelled = false;

    const fetchLogs = async () => {
      try {
        const response = await fetch(`${API_URL}/api/logs`);
        if (!response.ok) {
          return;
        }

        const payload = await response.json() as { logs?: string[] };
        if (!cancelled && Array.isArray(payload.logs)) {
          setLogs(payload.logs);
        }
      } catch {
        // Ignore transient log fetch failures.
      }
    };

    fetchLogs();
    const interval = setInterval(fetchLogs, 4000);

    return () => {
      cancelled = true;
      clearInterval(interval);
    };
  }, [refreshKey, enabled]);

  return { logs };
}
