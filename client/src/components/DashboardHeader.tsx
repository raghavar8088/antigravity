"use client";

import React, { useState } from 'react';

export default function DashboardHeader({ online, balance, dailyPnL, onResetSuccess }: { online: boolean, balance: number, dailyPnL: number, onResetSuccess?: () => void }) {
  const [isResetting, setIsResetting] = useState(false);

  const handleReset = async () => {
    if (!confirm('Reset the paper trading account to its initial state?')) return;
    setIsResetting(true);
    try {
      const res = await fetch('http://localhost:8080/api/admin/reset', {
        method: 'POST',
      });
      if (!res.ok) throw new Error('Reset failed');
      alert('Paper trading account has been reset.');
      onResetSuccess?.();
    } catch (error) {
      console.error(error);
      alert('Unable to reset trading account. Check engine connection.');
    } finally {
      setIsResetting(false);
    }
  };

  return (
    <header className="glass-panel p-6 flex flex-col md:flex-row justify-between items-start md:items-center gap-4">
      <div>
        <h1 className="text-3xl font-extrabold tracking-tight">
          ANTI<span className="text-gradient">GRAVITY</span>
        </h1>
        <p className="text-sm text-gray-400 mt-1 flex items-center gap-2">
          {online ? (
            <><span className="w-2 h-2 bg-green-500 rounded-full animate-pulse shadow-[0_0_8px_#10b981]"></span> ENGINE ONLINE</>
          ) : (
            <><span className="w-2 h-2 bg-red-500 rounded-full shadow-[0_0_8px_#ef4444]"></span> ENGINE OFFLINE</>
          )}
          <span className="px-2 py-0.5 rounded-full bg-blue-500/10 text-blue-400 border border-blue-500/20 text-xs font-mono ml-2">BTC/USDT ONLY</span>
        </p>
      </div>

      <div className="flex gap-8 items-center">
        <div className="text-right">
          <p className="text-sm text-gray-400 font-semibold uppercase tracking-wider">Total Balance</p>
          <p className="text-2xl font-bold font-mono text-white">${balance.toLocaleString(undefined, {minimumFractionDigits: 2})}</p>
        </div>
        
        <div className="text-right">
          <p className="text-sm text-gray-400 font-semibold uppercase tracking-wider">Daily PnL</p>
          <p className={`text-2xl font-bold font-mono ${dailyPnL >= 0 ? "text-green-400" : "text-red-400"}`}>
            {dailyPnL >= 0 ? "+" : ""}${dailyPnL.toLocaleString(undefined, {minimumFractionDigits: 2})}
          </p>
        </div>

        <button className="px-6 py-3 rounded-xl bg-red-500/10 text-red-500 border border-red-500/30 hover:bg-red-500 hover:text-white transition-all font-bold uppercase tracking-widest text-sm outline-none focus:ring-4 focus:ring-red-500/50 group shadow-[0_0_15px_rgba(239,68,68,0.2)] ml-4">
          <span className="group-hover:hidden tracking-[0.2em]">ARMED</span>
          <span className="hidden group-hover:block tracking-[0.2em]">KILL SWITCH</span>
        </button>
        <button
          onClick={handleReset}
          disabled={isResetting}
          className="px-6 py-3 rounded-xl bg-blue-500/10 text-blue-500 border border-blue-500/30 hover:bg-blue-500 hover:text-white transition-all font-bold uppercase tracking-widest text-sm outline-none focus:ring-4 focus:ring-blue-500/50 shadow-[0_0_15px_rgba(59,130,246,0.2)]"
        >
          {isResetting ? 'RESETTING...' : 'RESET ACCOUNT'}
        </button>
      </div>
    </header>
  );
}
