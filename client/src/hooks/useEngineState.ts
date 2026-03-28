"use client";
import { useState, useEffect } from "react";

export default function useEngineState() {
  const [engineOnline, setEngineOnline] = useState(false);
  const [balance, setBalance] = useState(1000.00);

  useEffect(() => {
    // Dynamically connect to either Local Engine or Cloud Render Engine 
    const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";
    
    fetch(`${API_URL}/health`)
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
