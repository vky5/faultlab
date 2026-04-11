import React, { useState, useEffect } from "react";
import { 
  Activity, 
  Search, 
  Shield, 
  ShieldAlert, 
  Clock, 
  TrendingUp, 
  Zap, 
  X, 
  Play, 
  Square,
  Plus,
  RefreshCw,
  BarChart3
} from "lucide-react";
import { motion, AnimatePresence } from "framer-motion";
import { useClusterStore, MetricsResult } from "../store";

export function MetricsPanel() {
  const { 
    metrics, 
    selectedClusterId, 
    handleStartMetrics, 
    handleStopMetrics, 
    handleWatchKey, 
    fetchMetricsSnapshot,
    showMetrics,
    setShowMetrics 
  } = useClusterStore();

  const [newKey, setNewKey] = useState("");
  const [intervalMs, setIntervalMs] = useState(1000);
  const [isRefreshing, setIsRefreshing] = useState(false);

  useEffect(() => {
    if (!selectedClusterId || !showMetrics) return;

    const interval = setInterval(() => {
      fetchMetricsSnapshot(selectedClusterId);
    }, 2000);

    return () => clearInterval(interval);
  }, [selectedClusterId, showMetrics]);

  const onStart = () => {
    if (selectedClusterId) handleStartMetrics(selectedClusterId, intervalMs);
  };

  const onStop = () => {
    if (selectedClusterId) handleStopMetrics(selectedClusterId);
  };

  const onAddKey = (e: React.FormEvent) => {
    e.preventDefault();
    if (selectedClusterId && newKey.trim()) {
      handleWatchKey(selectedClusterId, newKey.trim());
      setNewKey("");
    }
  };

  const manualRefresh = async () => {
    if (!selectedClusterId) return;
    setIsRefreshing(true);
    await fetchMetricsSnapshot(selectedClusterId);
    setTimeout(() => setIsRefreshing(false), 500);
  };

  const formatDuration = (ns?: number) => {
    if (ns === undefined || ns === null) return "-";
    const ms = ns / 1000000;
    if (ms < 1000) return `${ms.toFixed(1)}ms`;
    return `${(ms / 1000).toFixed(2)}s`;
  };

  if (!showMetrics) return null;

  const isActive = metrics?.stoppedAt === "0001-01-01T00:00:00Z" || !metrics?.stoppedAt;

  return (
    <motion.div
      initial={{ x: 400, opacity: 0 }}
      animate={{ x: 0, opacity: 1 }}
      exit={{ x: 400, opacity: 0 }}
      className="absolute top-0 right-0 w-[400px] h-full z-[60] bg-white/80 dark:bg-slate-900/80 backdrop-blur-2xl border-l border-slate-200 dark:border-slate-800 shadow-2xl flex flex-col"
    >
      {/* Header */}
      <div className="p-6 border-b border-slate-200 dark:border-slate-800 flex items-center justify-between bg-slate-50/50 dark:bg-slate-800/20">
        <div className="flex items-center gap-3">
          <div className="w-10 h-10 rounded-xl bg-primary/10 flex items-center justify-center text-primary">
            <BarChart3 className="w-6 h-6" />
          </div>
          <div>
            <h2 className="text-lg font-bold text-slate-900 dark:text-white leading-tight">System Metrics</h2>
            <div className="flex items-center gap-2">
              <div className={`w-2 h-2 rounded-full ${isActive ? "bg-emerald-500 animate-pulse" : "bg-slate-400"}`} />
              <span className="text-[10px] font-bold uppercase tracking-wider text-slate-500">
                {isActive ? "Live Sampling" : "Session Summary"}
              </span>
            </div>
          </div>
        </div>
        <button 
          onClick={() => setShowMetrics(false)}
          className="p-2 hover:bg-slate-200 dark:hover:bg-slate-800 rounded-full transition-colors text-slate-400"
        >
          <X className="w-5 h-5" />
        </button>
      </div>

      <div className="flex-1 overflow-y-auto p-6 space-y-8 custom-scrollbar">
        {/* Controls Section */}
        <section className="space-y-4">
          <div className="flex items-center justify-between font-bold text-[10px] uppercase tracking-widest text-slate-400 mb-2">
            <span>Session Controls</span>
            {isActive && (
              <button 
                onClick={manualRefresh}
                className={`flex items-center gap-1 hover:text-primary transition-colors ${isRefreshing ? "animate-spin" : ""}`}
              >
                <RefreshCw className="w-3 h-3" />
                Refresh
              </button>
            )}
          </div>
          
          <div className="grid grid-cols-2 gap-3">
            {!isActive ? (
              <button
                onClick={onStart}
                className="col-span-2 flex items-center justify-center gap-2 py-3 px-4 bg-primary text-white rounded-xl font-bold text-sm shadow-lg shadow-primary/20 hover:brightness-110 transition-all active:scale-95"
              >
                <Play className="w-4 h-4 fill-current" />
                Start Metrics Session
              </button>
            ) : (
              <button
                onClick={onStop}
                className="col-span-2 flex items-center justify-center gap-2 py-3 px-4 bg-red-500 text-white rounded-xl font-bold text-sm shadow-lg shadow-red-500/20 hover:brightness-110 transition-all active:scale-95"
              >
                <Square className="w-4 h-4 fill-current" />
                Stop Session
              </button>
            )}
          </div>

          <div className="p-4 bg-slate-50 dark:bg-slate-800/40 rounded-2xl border border-slate-100 dark:border-slate-800/50 space-y-4">
            <div className="flex items-center justify-between">
              <span className="text-xs font-medium text-slate-600 dark:text-slate-400">Sampling Interval</span>
              <div className="flex items-center gap-2">
                <input 
                  type="number" 
                  value={intervalMs}
                  onChange={(e) => setIntervalMs(parseInt(e.target.value))}
                  disabled={isActive}
                  className="w-20 px-2 py-1 bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-lg text-xs font-mono font-bold text-center outline-none focus:ring-2 focus:ring-primary/20 transition-all"
                />
                <span className="text-[10px] font-bold text-slate-400 uppercase">ms</span>
              </div>
            </div>
          </div>
        </section>

        {/* Watch Keys Section */}
        <section className="space-y-4">
          <h3 className="font-bold text-[10px] uppercase tracking-widest text-slate-400 mb-2">Tracked Keys</h3>
          
          <form onSubmit={onAddKey} className="flex gap-2">
            <div className="relative flex-1">
              <input 
                type="text"
                placeholder="key to track..."
                value={newKey}
                onChange={(e) => setNewKey(e.target.value)}
                className="w-full pl-9 pr-4 py-2 bg-slate-50 dark:bg-slate-800/40 border border-slate-200 dark:border-slate-700 rounded-xl text-xs font-medium outline-none focus:ring-2 focus:ring-primary/20 transition-all"
              />
              <Plus className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-slate-400" />
            </div>
            <button 
              type="submit"
              className="px-4 py-2 bg-slate-900 dark:bg-slate-100 text-white dark:text-slate-900 rounded-xl text-[10px] font-bold uppercase transition-all hover:opacity-90 active:scale-95"
            >
              Add
            </button>
          </form>

          <div className="flex flex-wrap gap-2">
            {metrics?.trackedKeys?.map(key => (
              <div key={key} className="px-3 py-1.5 bg-primary/5 border border-primary/10 rounded-lg text-[11px] font-mono font-bold text-primary flex items-center gap-2">
                <Activity className="w-3 h-3" />
                {key}
              </div>
            )) || (
              <div className="text-[11px] text-slate-500 italic">No keys tracked yet.</div>
            )}
          </div>
        </section>

        {/* Results Section */}
        <section className="space-y-4">
          <h3 className="font-bold text-[10px] uppercase tracking-widest text-slate-400 mb-2">Convergence Analysis</h3>
          
          <div className="space-y-4">
            {metrics?.results && Object.keys(metrics.results).length > 0 ? (
              Object.entries(metrics.results).map(([key, result]) => (
                <MetricsCard key={key} name={key} result={result} />
              ))
            ) : (
              <div className="p-8 bg-slate-50 dark:bg-slate-800/40 rounded-2xl border border-dashed border-slate-200 dark:border-slate-800 flex flex-col items-center justify-center text-center">
                <Shield className="w-8 h-8 text-slate-300 mb-2" />
                <p className="text-[11px] text-slate-500">Awaiting data snapshots to compute convergence metrics.</p>
              </div>
            )}
          </div>
        </section>
      </div>
      
      {/* Footer Info */}
      <div className="p-4 bg-slate-50 dark:bg-slate-800/40 border-t border-slate-200 dark:border-slate-800 text-[10px] text-slate-400 flex items-center justify-between">
        <div className="flex items-center gap-2 uppercase font-bold tracking-tighter">
          <Clock className="w-3 h-3" />
          {metrics?.startedAt ? `Started: ${new Date(metrics.startedAt).toLocaleTimeString()}` : "No active session"}
        </div>
        <div className="font-mono opacity-50">METRICS-v1.2</div>
      </div>
    </motion.div>
  );
}

function MetricsCard({ name, result }: { name: string; result: MetricsResult }) {
  const formatDuration = (ns?: number) => {
    if (ns === undefined || ns === null || ns < 0) return "-";
    const ms = ns / 1000000;
    if (ms < 1000) return `${ms.toFixed(0)}ms`;
    return `${(ms / 1000).toFixed(2)}s`;
  };

  return (
    <div className="p-4 rounded-2xl bg-white dark:bg-slate-800 border border-slate-200 dark:border-slate-700 shadow-sm hover:shadow-md transition-shadow group">
      <div className="flex items-center justify-between mb-4">
        <div className="flex items-center gap-3">
          <div className={`w-8 h-8 rounded-lg flex items-center justify-center ${result.FinalConsistent ? "bg-emerald-500/10 text-emerald-600" : "bg-red-500/10 text-red-600"}`}>
            {result.FinalConsistent ? <Shield className="w-4 h-4" /> : <ShieldAlert className="w-4 h-4" />}
          </div>
          <div>
            <h4 className="text-xs font-bold font-mono text-slate-900 dark:text-white group-hover:text-primary transition-colors">{name}</h4>
            <span className={`text-[9px] font-bold uppercase tracking-wider ${result.FinalConsistent ? "text-emerald-500" : "text-red-500"}`}>
              {result.FinalConsistent ? "Consistent" : "Diverged"}
            </span>
          </div>
        </div>
        <div className="text-right">
          <div className="text-[8px] font-bold text-slate-400 uppercase tracking-tighter mb-0.5">Peak Div</div>
          <div className="text-sm font-black font-mono text-slate-700 dark:text-slate-300 leading-none">
            {result.PeakDivergence}
          </div>
        </div>
      </div>

      <div className="grid grid-cols-2 gap-4 pt-4 border-t border-slate-100 dark:border-slate-700/50">
        <div className="space-y-1">
          <div className="flex items-center gap-1.5 text-[9px] font-bold text-slate-400 uppercase">
            <Zap className="w-3 h-3 text-amber-500" />
            Convergence
          </div>
          <div className="text-xs font-bold font-mono text-slate-700 dark:text-slate-300">
            {formatDuration(result.ConvergenceTime)}
          </div>
        </div>
        <div className="space-y-1">
          <div className="flex items-center gap-1.5 text-[9px] font-bold text-slate-400 uppercase">
             <TrendingUp className="w-3 h-3 text-indigo-500" />
            Area Under
          </div>
          <div className="text-xs font-bold font-mono text-slate-700 dark:text-slate-300">
             {result.AreaUnderDivergence ? (result.AreaUnderDivergence / 1000000).toFixed(0) : "0"} <span className="text-[10px] opacity-40">node-ms</span>
          </div>
        </div>
      </div>

      {/* Mini visualization if result has stale durations */}
      {result.StaleDurationPerNode && Object.keys(result.StaleDurationPerNode).length > 0 && (
        <div className="mt-4 pt-3 border-t border-slate-50 dark:border-slate-800 flex gap-1 h-1.5 items-end">
          {Object.entries(result.StaleDurationPerNode).slice(0, 8).map(([node, dur], i) => {
            const opacity = 0.2 + (dur / result.AreaUnderDivergence) * 2;
            return (
              <div 
                key={node} 
                className="flex-1 rounded-full bg-primary" 
                style={{ opacity: Math.min(1, opacity) }}
                title={`${node}: ${formatDuration(dur)} stale`}
              />
            );
          })}
        </div>
      )}
    </div>
  );
}
