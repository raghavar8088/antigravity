"use client";
import { useState, useEffect } from "react";

export default function useEngineState() {
  const [engineOnline, setEngineOnline] = useState(false);
  const [balance, setBalance] = useState(1000.00);

  useEffect(() => {
    // Attempting to ping the actual Phase 4 Golang Engine running locally
    fetch("http://localhost:8080/health")
      .then(res => {
        if (res.ok) setEngineOnline(true);
      })
      .catch(() => {
        setEngineOnline(false);
      });
  }, []);

  return {
    engineOnline,
    balance,
  };
}
