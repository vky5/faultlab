import React, { useState, useEffect, useMemo } from "react";
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
  BarChart3,
  Maximize2,
  Minimize2,
  Database,
  Network,
  Share2,
  Info,
  ChevronRight,
  ChevronDown,
  Copy,
  Check,
  ArrowRight,
  Package
} from "lucide-react";
import { motion, AnimatePresence } from "framer-motion";
import { useClusterStore, MetricsResult, MetricsSnapshot, TimelineEvent } from "../store";

interface MetricsPanelProps {
  isExpanded?: boolean;
  onToggleExpand?: () => void;
}

export function MetricsPanel({ isExpanded = false, onToggleExpand }: MetricsPanelProps) {
  const { 
    metrics, 
    metricsHistory,
    selectedClusterId, 
    handleStartMetrics, 
    handleStopMetrics, 
    handleWatchKey, 
    fetchMetricsSnapshot,
    handleFetchMetricsHistory,
    showMetrics,
    setShowMetrics 
  } = useClusterStore();

  const [newKey, setNewKey] = useState("");
  const [intervalMs, setIntervalMs] = useState(1000);
  const [isRefreshing, setIsRefreshing] = useState(false);
  const [showHistory, setShowHistory] = useState(false);
  const [expandedRow, setExpandedRow] = useState<string | null>(null);

  useEffect(() => {
    if (!selectedClusterId) return;
    fetchMetricsSnapshot(selectedClusterId);
    handleFetchMetricsHistory(selectedClusterId);
  }, [selectedClusterId]);

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

  if (!showMetrics) return null;

  const isActive = metrics ? (metrics.isActive ?? (metrics.stoppedAt === "0001-01-01T00:00:00Z" || !metrics.stoppedAt)) : false;
  
  return (
    <>
      {/* Dimmed backdrop when expanded */}
      <AnimatePresence>
        {isExpanded && (
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            onClick={onToggleExpand}
            className="fixed inset-0 bg-slate-950/40 backdrop-blur-sm z-[55]"
          />
        )}
      </AnimatePresence>

      <motion.div
        layout
        initial={{ x: 420, opacity: 0 }}
        animate={{ 
          right: isExpanded ? "5vw" : 0,
          top: isExpanded ? "7.5vh" : 0,
          opacity: 1,
          width: isExpanded ? "90vw" : "420px",
          height: isExpanded ? "85vh" : "100%",
          borderRadius: isExpanded ? "24px" : "0px",
          x: 0,
          y: 0,
        }}
        exit={{ x: 420, opacity: 0 }}
        transition={{ type: "spring", damping: 30, stiffness: 300, mass: 0.8 }}
        className="fixed top-0 z-[60] bg-white dark:bg-slate-900 border-l border-slate-200 dark:border-slate-800 shadow-2xl flex flex-col overflow-hidden"
      >
        {/* Header */}
        <div className="px-6 py-4 border-b border-slate-100 dark:border-slate-800 flex items-center justify-between bg-white dark:bg-slate-900 shrink-0">
          <div className="flex items-center gap-3">
            <div className="w-8 h-8 rounded-lg bg-primary/10 flex items-center justify-center text-primary">
              <BarChart3 className="w-5 h-5" />
            </div>
            <div>
              <h2 className="text-sm font-black text-slate-900 dark:text-white uppercase tracking-wider">Metrics Center</h2>
              <div className="flex items-center gap-2">
                <div className={`w-1.5 h-1.5 rounded-full ${isActive ? "bg-emerald-500 animate-pulse" : "bg-slate-300"}`} />
                <span className="text-[10px] font-bold text-slate-400 uppercase tracking-tighter">
                  {isActive ? "Sampling active" : "Historical Summary"}
                </span>
              </div>
            </div>
          </div>
          <div className="flex items-center gap-1">
            <button 
              onClick={onToggleExpand}
              className="p-2 hover:bg-slate-100 dark:hover:bg-slate-800 rounded-lg transition-colors text-slate-400 hover:text-primary"
            >
              {isExpanded ? <Minimize2 className="w-4 h-4" /> : <Maximize2 className="w-4 h-4" />}
            </button>
            <button 
              onClick={() => setShowMetrics(false)}
              className="p-2 hover:bg-red-50 dark:hover:bg-red-900/20 rounded-lg transition-colors text-slate-400 hover:text-red-500"
            >
              <X className="w-4 h-4" />
            </button>
          </div>
        </div>

        <div className="flex-1 overflow-y-auto p-6 space-y-8 custom-scrollbar">
          {/* Global Summary Stats */}
          <div className="grid grid-cols-3 gap-3">
            <StatPill 
              label="RPCs" 
              value={metrics?.clusterStats?.totalRPCs || 0} 
              icon={<Network className="w-3 h-3" />}
            />
            <StatPill 
              label="Writes" 
              value={metrics?.clusterStats?.totalWrites || 0} 
              icon={<Database className="w-3 h-3" />}
            />
            <StatPill 
              label="Audit" 
              value={metrics?.trackedKeys?.length || 0} 
              icon={<Activity className="w-3 h-3" />}
            />
          </div>

          {!showHistory ? (
            <div className="space-y-8">
              {/* Controls */}
              <section className="bg-slate-50 dark:bg-slate-800/40 rounded-2xl p-4 border border-slate-100 dark:border-slate-800/60">
                <div className="flex items-center justify-between mb-4">
                  <div className="flex items-center gap-2">
                    <Zap className="w-3.5 h-3.5 text-amber-500" />
                    <span className="text-[10px] font-black uppercase tracking-widest text-slate-500">Live Controls</span>
                  </div>
                  <button onClick={manualRefresh} className={`p-1 text-slate-400 hover:text-primary ${isRefreshing ? "animate-spin" : ""}`}>
                    <RefreshCw className="w-3 h-3" />
                  </button>
                </div>
                <div className="flex gap-2">
                   {!isActive ? (
                    <button onClick={onStart} className="flex-1 py-2 bg-primary text-white rounded-lg text-[11px] font-bold uppercase transition-transform active:scale-95 shadow-lg shadow-primary/20">
                      Start Sampling
                    </button>
                  ) : (
                    <button onClick={onStop} className="flex-1 py-2 bg-red-500 text-white rounded-lg text-[11px] font-bold uppercase transition-transform active:scale-95 shadow-lg shadow-red-500/20">
                      Stop
                    </button>
                  )}
                  <div className="flex items-center gap-2 px-3 bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-lg">
                    <span className="text-[10px] font-bold text-slate-400">Rate:</span>
                    <input 
                      type="number" 
                      value={intervalMs} 
                      onChange={e => setIntervalMs(parseInt(e.target.value))}
                      disabled={isActive}
                      className="w-12 bg-transparent text-[10px] font-black font-mono text-center outline-none"
                    />
                  </div>
                </div>
              </section>

              {/* Key Management */}
              <section className="space-y-4">
                 <div className="flex items-center justify-between">
                    <h3 className="text-[10px] font-black uppercase tracking-widest text-slate-400">Key Audit</h3>
                    {metricsHistory.length > 0 && (
                      <button onClick={() => setShowHistory(true)} className="text-[10px] font-black text-primary uppercase">History →</button>
                    )}
                 </div>
                 <form onSubmit={onAddKey} className="relative">
                    <input 
                      type="text" 
                      placeholder="Add key to audit..."
                      value={newKey}
                      onChange={e => setNewKey(e.target.value)}
                      className="w-full pl-9 pr-4 py-2 bg-slate-50 dark:bg-slate-800/40 border border-slate-200 dark:border-slate-800/80 rounded-xl text-[11px] outline-none focus:ring-1 focus:ring-primary/40 transition-all font-medium"
                    />
                    <Plus className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-slate-400" />
                 </form>

                 {/* Discovery Suggestion */}
                 {metrics?.clusterStats?.recentKeys?.some(k => !metrics.trackedKeys.includes(k)) && (
                    <div className="flex items-center gap-2 p-2 bg-amber-50/50 dark:bg-amber-900/10 border border-amber-100/50 dark:border-amber-900/20 rounded-lg">
                      <Search className="w-3 h-3 text-amber-500" />
                      <span className="text-[9px] font-bold text-amber-600/80 uppercase">Detected activity:</span>
                      <div className="flex gap-1 overflow-x-auto no-scrollbar">
                        {metrics.clusterStats.recentKeys.filter(k => !metrics.trackedKeys.includes(k)).slice(0, 3).map(k => (
                          <button key={k} onClick={() => handleWatchKey(selectedClusterId!, k)} className="px-1.5 py-0.5 bg-white dark:bg-slate-800 border border-amber-200 dark:border-amber-800 rounded text-[9px] font-black text-amber-700 hover:bg-amber-100 transition-colors">
                            {k}
                          </button>
                        ))}
                      </div>
                    </div>
                 )}
              </section>

              {/* Real-time Dashboard */}
              <section className="space-y-4">
                 {metrics?.results && Object.keys(metrics.results).length > 0 ? (
                    <MetricsSummary snap={metrics} expandedRow={expandedRow} onToggleRow={setExpandedRow} />
                 ) : (
                    <div className="text-center py-12 border-2 border-dashed border-slate-100 dark:border-slate-800 rounded-[2rem]">
                       <div className="w-10 h-10 bg-slate-50 dark:bg-slate-800 rounded-full flex items-center justify-center mx-auto mb-3">
                          <Info className="w-5 h-5 text-slate-300" />
                       </div>
                       <p className="text-[11px] text-slate-500">Enter a key or start sampling to see consistency metrics.</p>
                    </div>
                 )}
              </section>
            </div>
          ) : (
            <div className="space-y-10">
               {/* History Ticker */}
               <div className="flex items-center justify-between">
                  <button onClick={() => setShowHistory(false)} className="text-[10px] font-black text-slate-400 uppercase tracking-widest">← Back to Live</button>
                  <h3 className="text-[10px] font-black uppercase tracking-widest text-slate-500 ml-auto">Archive</h3>
               </div>

               <div className="space-y-12">
                  {metricsHistory.map((snap, idx) => (
                    <div key={idx} className="space-y-6">
                       <div className="flex items-center gap-4 text-[10px] font-black text-slate-400 uppercase tracking-widest">
                          <span className="w-12 h-[1px] bg-slate-100 dark:bg-slate-800" />
                          Session: {new Date(snap.startedAt).toLocaleString()}
                          <span className="flex-1 h-[1px] bg-slate-100 dark:bg-slate-800" />
                          <span className={`${snap.isActive ? "text-emerald-500" : ""}`}>{snap.isActive ? "[Active]" : "[Stopped]"}</span>
                       </div>
                       
                       {/* Session Throughput Snapshot */}
                       <div className="grid grid-cols-3 gap-3">
                          <StatPill label="Throughput" value={snap.clusterStats?.totalRPCs || 0} icon={<Network className="w-3 h-3" />} />
                          <StatPill label="Mutations" value={snap.clusterStats?.totalWrites || 0} icon={<Database className="w-3 h-3" />} />
                          <StatPill label="Audit Depth" value={snap.trackedKeys?.length || 0} icon={<Activity className="w-3 h-3" />} />
                       </div>

                       <MetricsSummary snap={snap} />
                    </div>
                  ))}
               </div>
            </div>
          )}
        </div>
      </motion.div>
    </>
  );
}

function StatPill({ label, value, icon }: { label: string; value: number | string; icon: React.ReactNode }) {
  return (
    <div className="flex items-center gap-2 px-3 py-2 bg-slate-50 dark:bg-slate-800/40 border border-slate-100 dark:border-slate-800/80 rounded-xl">
      <div className="text-slate-400">{icon}</div>
      <div className="flex flex-col leading-none">
        <span className="text-[11px] font-black font-mono text-slate-800 dark:text-slate-200">{value}</span>
        <span className="text-[8px] font-bold uppercase tracking-tighter text-slate-400">{label}</span>
      </div>
    </div>
  );
}

function MetricsSummary({ snap, expandedRow, onToggleRow }: { snap: MetricsSnapshot; expandedRow?: string | null; onToggleRow?: (id: string | null) => void }) {
  const insights = useMemo(() => {
    const resultsAry = Object.entries(snap.results || {});
    if (resultsAry.length === 0) return null;

    const allConverged = resultsAry.every(([_, r]) => r.FinalConsistent);
    const divergentKeys = resultsAry.filter(([_, r]) => !r.FinalConsistent).map(([k]) => k);
    
    // Sort keys by highest AUC
    const maxAUCKey = [...resultsAry].sort((a, b) => b[1].AreaUnderDivergence - a[1].AreaUnderDivergence)[0];
    
    const peakDivergence = Math.max(...resultsAry.map(([_, r]) => r.PeakDivergence));
    
    // Aggregate stale node duration
    const nodeStaleness: Record<string, number> = {};
    resultsAry.forEach(([_, r]) => {
      Object.entries(r.StaleDurationPerNode || {}).forEach(([node, dur]) => {
        nodeStaleness[node] = (nodeStaleness[node] || 0) + dur;
      });
    });
    const topStaleNode = Object.entries(nodeStaleness).sort((a, b) => b[1] - a[1])[0];

    return {
      allConverged,
      divergentKeys,
      maxAUCKey: maxAUCKey ? { key: maxAUCKey[0], val: maxAUCKey[1].AreaUnderDivergence } : null,
      peakDivergence,
      topStaleNode: topStaleNode ? { node: topStaleNode[0], val: topStaleNode[1] } : null,
    };
  }, [snap.results]);

  const formatDuration = (ns?: number) => {
    if (ns === undefined || ns === null || ns < 0) return "-";
    const ms = ns / 1000000;
    if (ms < 1000) return `${ms.toFixed(1)}ms`;
    return `${(ms / 1000).toFixed(2)}s`;
  };

  const getTopStaleForKey = (result: MetricsResult) => {
    const entries = Object.entries(result.StaleDurationPerNode || {}).sort((a, b) => b[1] - a[1]);
    return entries.length > 0 ? `${entries[0][0]}=${formatDuration(entries[0][1])}` : "-";
  };

  return (
    <div className="space-y-6">
      <div className="overflow-x-auto">
        <table className="w-full text-left border-collapse">
          <thead>
            <tr className="border-b border-slate-100 dark:border-slate-800">
              <th className="py-2 text-[9px] font-black uppercase text-slate-400 tracking-widest">Key</th>
              <th className="py-2 text-[9px] font-black uppercase text-slate-400 tracking-widest text-center">Final</th>
              <th className="py-2 text-[9px] font-black uppercase text-slate-400 tracking-widest text-right px-2">Convergence</th>
              <th className="py-2 text-[9px] font-black uppercase text-slate-400 tracking-widest text-center">Peak</th>
              <th className="py-2 text-[9px] font-black uppercase text-slate-400 tracking-widest text-right px-2">AUC</th>
              <th className="py-2 text-[9px] font-black uppercase text-slate-400 tracking-widest text-right">Top Stale</th>
            </tr>
          </thead>
          <tbody className="font-mono text-[10px]">
            {Object.entries(snap.results || {})
              .filter(([key]) => key !== "__CLUSTER__") // Hide virtual lifecycle key
              .map(([key, result]) => {
               const isOpen = expandedRow === key;
               return (
                  <React.Fragment key={key}>
                    <tr 
                      onClick={() => onToggleRow?.(isOpen ? null : key)}
                      className={`hover:bg-slate-50 dark:hover:bg-slate-800/40 cursor-pointer group ${isOpen ? "bg-slate-50/50 dark:bg-slate-800/20" : ""}`}
                    >
                      <td className="py-3 font-black text-slate-900 dark:text-white flex items-center gap-1">
                        {onToggleRow && (
                          <div className="text-slate-300 group-hover:text-primary transition-colors">
                            {isOpen ? <ChevronDown className="w-3 h-3" /> : <ChevronRight className="w-3 h-3" />}
                          </div>
                        )}
                        <div className="flex items-center gap-1.5">
                          {key}
                          <button 
                            onClick={(e) => {
                              e.stopPropagation();
                              navigator.clipboard.writeText(JSON.stringify(result, null, 2));
                            }}
                            className="p-1 hover:bg-slate-200 dark:hover:bg-slate-700 rounded transition-colors text-slate-300 hover:text-primary opacity-0 group-hover:opacity-100"
                            title="Copy Result JSON"
                          >
                            <Copy className="w-2.5 h-2.5" />
                          </button>
                        </div>
                      </td>
                      <td className="py-3 text-center">
                        <span className={`font-black uppercase text-[8px] px-1.5 py-0.5 rounded ${result.FinalConsistent ? "bg-emerald-500/10 text-emerald-500" : "bg-red-500/10 text-red-500"}`}>
                          {result.FinalConsistent ? "True" : "False"}
                        </span>
                      </td>
                      <td className="py-3 text-right text-slate-500 font-bold px-2">{formatDuration(result.ConvergenceTime)}</td>
                      <td className="py-3 text-center font-black">{result.PeakDivergence}</td>
                      <td className="py-3 text-right text-slate-500 font-bold px-2">{formatDuration(result.AreaUnderDivergence)}</td>
                      <td className="py-3 text-right text-slate-400 font-bold">{getTopStaleForKey(result)}</td>
                    </tr>
                    {isOpen && (
                       <tr>
                         <td colSpan={6} className="pb-4 px-2">
                            <div className="bg-white dark:bg-slate-900 border border-slate-100 dark:border-slate-800 rounded-2xl p-4 shadow-inner">
                               <div className="flex items-center justify-between mb-4">
                                  <span className="text-[9px] font-black text-slate-400 uppercase tracking-widest">Convergence Curve (Distinct Values)</span>
                                  <div className="flex gap-4">
                                      <div className="text-[9px] font-bold text-slate-400"><span className="text-primary font-black uppercase mr-1">First Agreement:</span> {formatDuration(result.FirstAgreementTime)}</div>
                                  </div>
                               </div>
                               <div className="h-24 w-full bg-slate-50/50 dark:bg-slate-950/20 rounded-xl p-2 border border-slate-100 dark:border-slate-800/50">
                                  <svg viewBox="0 0 400 100" className="w-full h-full">
                                    <DivergenceSparkline results={result.ConvergenceCurve || result.DivergenceOverTime} color="#6366f1" />
                                  </svg>
                               </div>
                               <div className="space-y-2 mt-6 pt-6 border-t border-slate-100 dark:border-slate-800">
                                  <div className="flex items-center gap-2 mb-1">
                                     <Clock className="w-3 h-3 text-slate-400" />
                                     <span className="text-[9px] font-black text-slate-400 uppercase tracking-widest">Deterministic Replay (Timeline)</span>
                                  </div>
                                  <TimelineView timeline={result.Timeline} />
                               </div>
                            </div>
                         </td>
                       </tr>
                    )}
                  </React.Fragment>
               );
            })}
          </tbody>
        </table>
      </div>

      {insights && (
        <div className="space-y-6">
          {/* Global Event Feed */}
          <div className="p-5 bg-slate-50 dark:bg-slate-800/40 border border-slate-200 dark:border-slate-800 rounded-[2rem]">
             <div className="text-[9px] font-black uppercase tracking-[0.2em] text-slate-400 mb-4 flex items-center justify-between">
                <div className="flex items-center gap-2">
                   <Activity className="w-3 h-3 text-primary" />
                   Cluster-Wide Event Stream
                </div>
                <button 
                  onClick={() => {
                    const logs = Object.entries(snap.results || {})
                      .flatMap(([k, r]) => (r.Timeline || []).map(e => ({ ...e, key: k })))
                      .sort((a, b) => b.Time - a.Time)
                      .map(e => {
                        const time = (e.Time / 1000000).toFixed(1);
                        const action = getEventActionText(e.EventType, e.Source, e.Origin);
                        const round = e.Round > 0 ? ` [R${e.Round}]` : "";
                        const flow = e.Source && e.Source !== e.NodeID ? ` (${e.Source}→${e.NodeID})` : ` (${e.NodeID})`;
                        const value = e.Value ? ` "${e.Value}" (v${e.Version})` : "";
                        return `${time}ms | ${e.key} | ${action}${value} | Node${flow}${round}`;
                      })
                      .join("\n");
                    navigator.clipboard.writeText(logs);
                  }}
                  className="p-1.5 hover:bg-white dark:hover:bg-slate-700 rounded-md transition-colors text-slate-400 hover:text-primary flex items-center gap-1 group"
                >
                   <Copy className="w-3 h-3" />
                   <span className="opacity-0 group-hover:opacity-100 transition-opacity text-[9px]">Copy</span>
                </button>
             </div>

             {/* Bash-like Event Stream */}
             <div className="max-h-80 overflow-y-auto custom-scrollbar p-3 space-y-1 bg-white dark:bg-slate-900/50 rounded-xl border border-slate-200 dark:border-slate-800">
                {Object.entries(snap.results || {})
                  .filter(([k]) => k !== "__CLUSTER__")
                  .flatMap(([k, r]) => (r.Timeline || []).map(e => ({ ...e, key: k })))
                  .concat((snap.results?.["__CLUSTER__"]?.Timeline || []).map(e => ({ ...e, key: "SYSTEM" })))
                  .sort((a, b) => b.Time - a.Time)
                  .slice(0, 30)
                  .map((event, i) => {
                    const isSystem = event.key === "SYSTEM";
                    const isResolve = event.EventType === "RESOLVE";
                    const timeMs = (event.Time / 1000000).toFixed(0);

                    return (
                      <div key={i} className="font-mono text-[10px] flex items-baseline gap-2 flex-wrap border-b border-slate-100 dark:border-white/5 pb-1 opacity-90 hover:opacity-100 transition-opacity">
                         <span className="text-slate-400 font-bold min-w-[55px]">[{timeMs}ms]</span>
                         
                         {isSystem ? (
                           <span className="text-slate-500 font-black uppercase tracking-tighter">SYS:</span>
                         ) : (
                           <div className="flex items-center gap-1">
                              {event.Source && event.Source !== event.NodeID ? (
                                <>
                                  <span className="text-blue-500 font-bold">{event.Source}</span>
                                  <ArrowRight className="w-2.5 h-2.5 text-slate-300 mx-0.5" />
                                  <span className="text-emerald-500 font-bold">{event.NodeID}</span>
                                </>
                              ) : (
                                <span className="text-emerald-500 font-bold">{event.NodeID}</span>
                              )}
                           </div>
                         )}

                         <span className={`font-black uppercase tracking-tight ${isResolve ? "text-violet-500 underline decoration-violet-500/20" : "text-slate-700 dark:text-slate-300"}`}>
                            {getEventActionText(event.EventType, event.Source, event.Origin)}
                         </span>

                         {!isSystem && (
                           <>
                             <span className="text-slate-400 font-bold ml-auto">{event.key}</span>
                             <span className="text-primary font-black">'{event.Value}'</span>
                             <span className="text-slate-400 text-[9px] opacity-70">v{event.Version}</span>
                             {event.LWWTimestamp && (
                               <span className="text-[8px] text-slate-500 font-mono tracking-tighter opacity-50 bg-slate-100 dark:bg-white/5 px-1 rounded" title="LWW Timestamp">
                                 ts:{event.LWWTimestamp}
                               </span>
                             )}
                           </>
                         )}

                         {isSystem && (
                           <span className="text-slate-400 italic opacity-80">{event.Value}</span>
                         )}
                      </div>
                    );
                  })
                }
             </div>
          </div>

          <div className="p-5 bg-slate-900 dark:bg-black rounded-[2rem] text-white/90 shadow-2xl border border-white/5">
           <div className="text-[9px] font-black uppercase tracking-[0.3em] text-white/30 mb-4 flex items-center gap-2">
              <div className="w-4 h-[1px] bg-white/10" />
              State Analysis & Insights
           </div>
           <ul className="space-y-3 text-[10px] font-bold font-mono">
              <li className="flex items-start gap-3">
                 <div className={`w-2 h-2 rounded-full mt-1 ${insights.allConverged ? "bg-emerald-500 shadow-[0_0_8px_rgba(16,185,129,0.5)]" : "bg-red-500 shadow-[0_0_8px_rgba(239,68,68,0.5)]"}`} />
                 <span>
                    {insights.allConverged 
                      ? "all tracked keys are currently converged" 
                      : `keys still divergent: ${insights.divergentKeys.join(", ")}`}
                 </span>
              </li>
              {insights.maxAUCKey && (
                <li className="flex items-start gap-3">
                   <span className="text-primary mt-1">●</span>
                   <span className="text-white/60">highest inconsistency area: <span className="text-primary font-black uppercase tracking-tighter">{insights.maxAUCKey.key}</span> ({formatDuration(insights.maxAUCKey.val)})</span>
                </li>
              )}
              <li className="flex items-start gap-3">
                 <span className="text-primary mt-1">●</span>
                 <span className="text-white/60">highest observed peak divergence: <span className="text-white font-black">{insights.peakDivergence}</span> nodes</span>
              </li>
              {insights.topStaleNode && (
                <li className="flex items-start gap-3">
                   <span className="text-primary mt-1">●</span>
                   <span className="text-white/60">most stale node overall: <span className="text-amber-400 font-black">{insights.topStaleNode.node}</span>={formatDuration(insights.topStaleNode.val)}</span>
                </li>
              )}
           </ul>
         </div>
        </div>
      )}
    </div>
  );
}

function DivergenceSparkline({ results, color }: { results: any[]; color: string }) {
  if (!results || results.length < 2) return null;

  const maxNodes = 10; 
  const maxTime = results[results.length - 1].Time;

  const points = results.map(r => {
    const x = (r.Time / maxTime) * 400;
    const y = 90 - (r.Divergence / maxNodes) * 80;
    return `${x},${y}`;
  });

  const pathData = `M 0,90 ${points.map(p => `L ${p}`).join(" ")} L 400,90 Z`;
  const lineData = `M 0,90 ${points.map(p => `L ${p}`).join(" ")}`;

  return (
    <>
      <motion.path
        initial={{ pathLength: 0, opacity: 0 }}
        animate={{ pathLength: 1, opacity: 1 }}
        transition={{ duration: 1, ease: "easeOut" }}
        d={pathData}
        fill={color}
        fillOpacity="0.05"
      />
      <motion.path
        initial={{ pathLength: 0 }}
        animate={{ pathLength: 1 }}
        transition={{ duration: 1, ease: "easeOut" }}
        d={lineData}
        fill="none"
        stroke={color}
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </>
  );
}

function NodeBadge({ id }: { id: string }) {
  // Simple heuristic for group coloring based on h1 hypothesis
  const isGroup2 = ["node3", "node4", "node5"].includes(id); 
  const groupLabel = isGroup2 ? "GROUP 2" : "GROUP 1";
  const groupColor = isGroup2 
    ? "bg-amber-500/10 text-amber-500 border-amber-500/20" 
    : "bg-blue-500/10 text-blue-500 border-blue-500/20";

  return (
    <div className="flex items-center gap-1.5 group">
      <span className={`px-1.5 py-0.5 rounded border text-[7px] font-black uppercase tracking-tighter ${groupColor}`}>
        {groupLabel}
      </span>
      <span className="px-2 py-0.5 bg-slate-900/5 dark:bg-white/5 rounded border border-slate-200 dark:border-white/10 font-mono font-bold text-slate-700 dark:text-slate-200 min-w-[50px] text-center text-[10px]">
        {id}
      </span>
    </div>
  );
}

function getEventActionText(type: string, source: string, origin: string) {
  switch (type) {
    case "WRITE": return `ACCEPTED WRITE`;
    case "GOSSIP_RECEIVE": return `GOSSIP UPDATE`;
    case "RESOLVE": return `CONFLICT RESOLVED (LWW)`;
    case "NODE_JOIN": return "JOINED CLUSTER";
    case "PARTITION": return "PARTITIONED";
    case "HEAL": return "PARTITION HEALED";
    case "CRASH": return "CRASHED";
    case "RECOVER": return "RECOVERED";
    default: return "ACTION";
  }
}

function getEventTypeColor(type: string) {
  switch (type) {
    case "WRITE":
      return "bg-blue-500/10 text-blue-600 dark:text-blue-400 border-blue-500/20";
    case "GOSSIP_RECEIVE":
      return "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/20";
    case "RESOLVE":
      return "bg-violet-500/10 text-violet-600 dark:text-violet-400 border-violet-500/20";
    case "PARTITION":
    case "CRASH":
      return "bg-red-500/10 text-red-600 dark:text-red-400 border-red-500/20";
    case "HEAL":
    case "RECOVER":
    case "NODE_JOIN":
      return "bg-amber-500/10 text-amber-600 dark:text-amber-400 border-amber-500/20";
    default:
      return "bg-slate-500/10 text-slate-600 dark:text-slate-400 border-slate-500/20";
  }
}

function TimelineView({ timeline }: { timeline: TimelineEvent[] }) {
  if (!timeline || timeline.length === 0) {
    return (
      <div className="py-8 text-center bg-slate-50/50 dark:bg-slate-950/20 rounded-xl border border-dashed border-slate-200 dark:border-slate-800">
        <p className="text-[10px] font-bold text-slate-400 uppercase tracking-widest">No timeline events recorded</p>
      </div>
    );
  }

  const getEventStyle = (type: string) => {
    switch (type) {
      case "WRITE": return "bg-blue-500/10 text-blue-500 border-blue-500/20";
      case "GOSSIP_RECEIVE": return "bg-emerald-500/10 text-emerald-500 border-emerald-500/20";
      case "RESOLVE": return "bg-violet-500/10 text-violet-500 border-violet-500/20 shadow-[0_0_10px_rgba(139,92,246,0.1)]";
      case "PARTITION": 
      case "CRASH": return "bg-red-500/10 text-red-500 border-red-500/20";
      case "HEAL":
      case "RECOVER":
      case "NODE_JOIN": return "bg-amber-500/10 text-amber-500 border-amber-500/20";
      default: return "bg-slate-500/10 text-slate-500 border-slate-500/20";
    }
  };

  const formatTime = (ns: number) => {
    const ms = ns / 1000000;
    return `${ms.toFixed(1)}ms`;
  };

  return (
    <div className="overflow-hidden border border-slate-100 dark:border-slate-800 rounded-xl bg-white/50 dark:bg-slate-900/50 backdrop-blur-sm">
      <div className="max-h-80 overflow-y-auto custom-scrollbar">
        <table className="w-full text-left border-collapse">
          <thead className="sticky top-0 bg-slate-100/80 dark:bg-slate-800/80 backdrop-blur-md z-10">
            <tr>
              <th className="py-2.5 px-3 text-[8px] font-black uppercase text-slate-500 tracking-tighter">Time</th>
              <th className="py-2.5 px-3 text-[8px] font-black uppercase text-slate-500 tracking-tighter">Event Narrative & Causal Flow</th>
              <th className="py-2.5 px-3 text-[8px] font-black uppercase text-slate-500 tracking-tighter text-right flex items-center justify-end gap-2 pr-3">
                <span>Logical Round</span>
                <button 
                  onClick={() => {
                    const logs = timeline
                      .map(e => `[${(e.Time / 1000000).toFixed(1)}ms] ${e.NodeID} ${e.EventType} ${getEventActionText(e.EventType, e.Source, e.Origin)} | Value: ${e.Value} (v${e.Version}) | Flow: ${e.Source} -> ${e.Origin}`)
                      .join("\n");
                    navigator.clipboard.writeText(logs);
                  }}
                  className="p-1 hover:bg-white dark:hover:bg-slate-700 rounded transition-colors text-slate-400 hover:text-primary"
                  title="Copy Timeline"
                >
                  <Copy className="w-2.5 h-2.5" />
                </button>
              </th>
            </tr>
          </thead>
          <tbody className="font-mono text-[9px]">
            {timeline.map((event, idx) => (
              <tr key={idx} className="border-b border-slate-100 dark:border-slate-800/50 hover:bg-white dark:hover:bg-slate-800 transition-colors">
                <td className="py-3 px-3 font-bold text-slate-400 whitespace-nowrap align-top">{formatTime(event.Time)}</td>
                <td className="py-3 px-3">
                  <div className="flex flex-col gap-2">
                    <div className="flex items-center gap-3">
                       <NodeBadge id={event.NodeID} />
                       <span className={`px-1.5 py-0.5 rounded-full border text-[7px] font-black ${getEventStyle(event.EventType)}`}>
                        {event.EventType}
                      </span>
                    </div>
                    
                    <div className="pl-1 space-y-1">
                      <div className="text-slate-600 dark:text-slate-300 font-bold leading-tight">
                        {getEventActionText(event.EventType, event.Source, event.Origin)}
                      </div>
                      <div className="flex items-center gap-2 flex-wrap">
                        <span className="text-primary font-black px-1.5 py-0.5 bg-primary/5 rounded border border-primary/10 tracking-tight">
                          '{event.Value}'
                        </span>
                        <span className="text-[8px] text-slate-400 font-bold">Version {event.Version}</span>
                        
                        <div className="flex items-center gap-1.5 ml-2 border-l border-slate-200 dark:border-slate-700 pl-2">
                           <span className="text-[8px] font-black text-slate-400 uppercase tracking-tighter">Flow:</span>
                           <span className="text-[9px] font-bold text-slate-400">{event.Source}</span>
                           <ChevronRight className="w-2.5 h-2.5 text-slate-300" />
                           <span className="text-[9px] font-black text-slate-500">{event.Origin}</span>
                        </div>
                      </div>
                    </div>
                  </div>
                </td>
                <td className="py-3 px-3 text-right align-top">
                  {event.Round > 0 ? (
                    <span className="px-2 py-1 bg-slate-100 dark:bg-slate-800 text-[8px] font-black text-slate-500 rounded-lg border border-slate-200 dark:border-slate-700">
                      R{event.Round}
                    </span>
                  ) : (
                    <span className="text-slate-300 text-[8px] font-bold uppercase tracking-widest italic">Init</span>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
