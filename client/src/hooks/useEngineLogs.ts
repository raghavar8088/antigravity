import { useEffect, useState } from "react";

const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

export default function useEngineLogs(refreshKey = 0) {
  const [logs, setLogs] = useState<string[]>([]);

  useEffect(() => {
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
  }, [refreshKey]);

  return { logs };
}
