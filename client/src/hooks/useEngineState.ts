"use client";
import { useEffect, useState } from "react";

const FALLBACK_BALANCE = 100000.0;

export default function useEngineState() {
  const [engineOnline, setEngineOnline] = useState(false);

  useEffect(() => {
    const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

    fetch(`${API_URL}/health`)
      .then((res) => {
        setEngineOnline(res.ok);
      })
      .catch(() => {
        setEngineOnline(false);
      });
  }, []);

  return {
    engineOnline,
    balance: FALLBACK_BALANCE,
  };
}
