import React, { useState, useEffect, useMemo, useRef } from "react";
import { 
  Activity, 
  Search, 
  Shield, 
  ShieldAlert, 
  Clock, 
  Database,
  Network,
  X, 
  Play, 
  Pause,
  SkipBack,
  SkipForward,
  Maximize2,
  Minimize2,
  ChevronRight,
  ChevronDown,
  Check,
  ArrowRight,
  AlertTriangle,
  Zap,
  FileText
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
    selectedClusterId, 
    handleStartMetrics, 
    handleStopMetrics, 
    handleWatchKey, 
    fetchMetricsSnapshot,
    showMetrics,
    setShowMetrics,
    playbackTimeMs,
    setPlaybackTimeMs,
    isPlaying,
    setIsPlaying
  } = useClusterStore();

  const [newKey, setNewKey] = useState("");
  const [intervalMs, setIntervalMs] = useState(1000);
  const [selectedKey, setSelectedKey] = useState<string | null>(null);

  useEffect(() => {
    if (!selectedClusterId || !showMetrics) return;
    const interval = setInterval(() => fetchMetricsSnapshot(selectedClusterId), 2000);
    return () => clearInterval(interval);
  }, [selectedClusterId, showMetrics]);

  const snap = metrics;
  const isActive = snap ? (snap.isActive ?? (snap.stoppedAt === "0001-01-01T00:00:00Z" || !snap.stoppedAt)) : false;

  // Aggregate global events
  const globalEvents = useMemo(() => {
    if (!snap?.results) return [];
    return Object.entries(snap.results)
      .filter(([k]) => k !== "__CLUSTER__")
      .flatMap(([k, r]) => (r.Timeline || []).map(e => ({ ...e, key: k })))
      .concat((snap.results?.["__CLUSTER__"]?.Timeline || []).map(e => ({ ...e, key: "SYSTEM" })))
      .filter(e => e.EventType !== "GOSSIP_RECEIVE") // Filter out noise
      .sort((a, b) => a.Time - b.Time);
  }, [snap]);

  const maxTimeMs = useMemo(() => {
    if (globalEvents.length === 0) return 0;
    return globalEvents[globalEvents.length - 1].Time / 1000000;
  }, [globalEvents]);

  if (!showMetrics) return null;

  return (
    <>
      <AnimatePresence>
        {isExpanded && (
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            onClick={onToggleExpand}
            className="fixed inset-0 bg-black/80 backdrop-blur-md z-[55]"
          />
        )}
      </AnimatePresence>

      <motion.div
        layout
        initial={{ x: 420, opacity: 0 }}
        animate={{ 
          right: isExpanded ? "5vw" : 0,
          top: isExpanded ? "5vh" : 0,
          opacity: 1,
          width: isExpanded ? "90vw" : "420px",
          height: isExpanded ? "90vh" : "100%",
          borderRadius: isExpanded ? "24px" : "0px",
          x: 0,
          y: 0,
        }}
        exit={{ x: 420, opacity: 0 }}
        transition={{ type: "spring", damping: 30, stiffness: 300, mass: 0.8 }}
        className={`fixed top-0 z-[60] bg-slate-950 border-l border-white/10 shadow-2xl flex flex-col overflow-hidden text-slate-300 ${isExpanded ? "border shadow-[0_0_50px_rgba(0,0,0,0.5)] print:hidden" : "print:hidden"}`}
      >
        {/* HEADER */}
        <div className="px-6 py-4 border-b border-white/5 flex items-center justify-between shrink-0 bg-slate-900/50">
          <div className="flex items-center gap-3">
            <div className="w-8 h-8 rounded-lg bg-indigo-500/20 flex items-center justify-center text-indigo-400 border border-indigo-500/30">
              <Activity className="w-5 h-5" />
            </div>
            <div>
              <h2 className="text-sm font-black text-white uppercase tracking-widest">FaultLab Simulator</h2>
              <div className="flex items-center gap-2">
                <div className={`w-1.5 h-1.5 rounded-full ${isActive ? "bg-emerald-500 animate-pulse shadow-[0_0_8px_rgba(16,185,129,0.8)]" : "bg-slate-500"}`} />
                <span className="text-[10px] font-bold text-slate-400 uppercase tracking-tighter">
                  {isActive ? "Telemetry Active" : "Telemetry Inactive"}
                </span>
              </div>
            </div>
          </div>
          <div className="flex items-center gap-2">
            {!isActive ? (
              <button onClick={() => selectedClusterId && handleStartMetrics(selectedClusterId, intervalMs)} className="px-3 py-1.5 bg-indigo-600 hover:bg-indigo-500 text-white rounded text-[10px] font-bold uppercase transition-colors">
                Start Audit
              </button>
            ) : (
              <button onClick={() => selectedClusterId && handleStopMetrics(selectedClusterId)} className="px-3 py-1.5 bg-red-500/20 text-red-400 hover:bg-red-500/30 border border-red-500/30 rounded text-[10px] font-bold uppercase transition-colors">
                Stop Audit
              </button>
            )}
            
            {isExpanded && (
              <>
                <div className="w-px h-6 bg-white/10 mx-1" />
                <button 
                  onClick={() => generatePDFReport(snap, globalEvents)}
                  className="px-3 py-1.5 bg-slate-800 text-slate-300 hover:bg-slate-700 border border-white/10 rounded text-[10px] font-bold uppercase transition-colors flex items-center gap-1.5"
                >
                  <FileText className="w-3 h-3" />
                  Export PDF
                </button>
              </>
            )}

            <div className="w-px h-6 bg-white/10 mx-1" />
            <button onClick={onToggleExpand} className="p-2 hover:bg-white/10 rounded-lg transition-colors text-slate-400 hover:text-white">
              {isExpanded ? <Minimize2 className="w-4 h-4" /> : <Maximize2 className="w-4 h-4" />}
            </button>
            <button onClick={() => setShowMetrics(false)} className="p-2 hover:bg-red-500/20 rounded-lg transition-colors text-slate-400 hover:text-red-500">
              <X className="w-4 h-4" />
            </button>
          </div>
        </div>

        {/* COMPACT MODE vs EXPANDED MODE */}
        {!isExpanded ? (
          <div className="flex-1 overflow-y-auto p-6 space-y-6 custom-scrollbar">
            {/* Minimal Dashboard for unexpanded */}
            <div className="p-4 bg-slate-900 border border-white/5 rounded-2xl">
              <h3 className="text-[10px] font-black uppercase tracking-widest text-slate-500 mb-3">Key Audit</h3>
              <form onSubmit={(e) => { e.preventDefault(); if (selectedClusterId && newKey) { handleWatchKey(selectedClusterId, newKey); setNewKey(""); } }} className="relative mb-4">
                <input 
                  type="text" 
                  placeholder="Monitor a key..."
                  value={newKey}
                  onChange={e => setNewKey(e.target.value)}
                  className="w-full pl-9 pr-4 py-2 bg-slate-950 border border-white/10 rounded-xl text-[11px] outline-none focus:border-indigo-500/50 text-white placeholder-slate-600 transition-colors"
                />
                <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-slate-500" />
              </form>
              
              <div className="space-y-2">
                {Object.entries(snap?.results || {}).filter(([k]) => k !== "__CLUSTER__").map(([k, r]) => (
                  <div key={k} className="flex justify-between items-center p-2 bg-slate-950/50 border border-white/5 rounded-lg text-[11px]">
                     <span className="font-bold text-white">{k}</span>
                     <span className={`px-1.5 py-0.5 rounded text-[9px] font-black uppercase ${r.FinalConsistent ? "bg-emerald-500/20 text-emerald-400" : "bg-red-500/20 text-red-400"}`}>
                       {r.FinalConsistent ? "Converged" : "Divergent"}
                     </span>
                  </div>
                ))}
                {Object.keys(snap?.results || {}).length === 0 && (
                   <div className="text-center text-[10px] text-slate-500 py-4">No keys monitored.</div>
                )}
              </div>
            </div>

            <div className="p-4 bg-indigo-950/20 border border-indigo-500/20 rounded-2xl text-center">
               <Maximize2 className="w-6 h-6 text-indigo-400/50 mx-auto mb-2" />
               <p className="text-[11px] text-indigo-300/70 font-medium">Expand this panel to access the full Distributed Systems Execution Replay, Replica Matrix, and Timeline Scrubbing.</p>
               <button onClick={onToggleExpand} className="mt-3 px-4 py-1.5 bg-indigo-500/20 text-indigo-400 hover:bg-indigo-500/30 rounded font-bold uppercase tracking-widest text-[10px] transition-colors">
                  Open Laboratory
               </button>
            </div>
          </div>
        ) : (
          /* EXPANDED "FAULTLAB LABORATORY" VIEW */
          <div className="flex-1 flex flex-col overflow-hidden bg-slate-950">
            {/* LAB HEADER METRICS */}
            <div className="grid grid-cols-5 gap-px bg-white/5 border-b border-white/5 shrink-0">
               <MetricBox label="Monitored Keys" value={(snap?.trackedKeys || []).length} />
               <MetricBox label="Total RPCs" value={snap?.clusterStats?.totalRPCs || 0} />
               <MetricBox label="Global Divergence" value={calculateGlobalDivergence(snap?.results)} />
               <MetricBox label="Avg Convergence" value={calculateAvgConvergence(snap?.results)} />
               <MetricBox label="Partitions/Heals" value={globalEvents.filter(e => e.EventType === "PARTITION" || e.EventType === "HEAL").length} />
            </div>

            {/* MAIN LAB CONTENT */}
            <div className="flex-1 flex overflow-hidden">
               {/* LEFT SIDEBAR: Key Selector */}
               <div className="w-64 bg-slate-900/30 border-r border-white/5 flex flex-col">
                 <div className="p-3 border-b border-white/5">
                    <h3 className="text-[10px] font-black uppercase tracking-widest text-slate-500 mb-2">Monitored State</h3>
                    <form onSubmit={(e) => { e.preventDefault(); if (selectedClusterId && newKey) { handleWatchKey(selectedClusterId, newKey); setNewKey(""); } }} className="relative">
                      <input 
                        type="text" 
                        placeholder="Add key..."
                        value={newKey}
                        onChange={e => setNewKey(e.target.value)}
                        className="w-full pl-7 pr-3 py-1.5 bg-slate-950 border border-white/10 rounded-lg text-[11px] outline-none text-white transition-colors"
                      />
                      <Search className="absolute left-2.5 top-1/2 -translate-y-1/2 w-3 h-3 text-slate-500" />
                    </form>
                 </div>
                 <div className="flex-1 overflow-y-auto custom-scrollbar p-2 space-y-1">
                    <button 
                      onClick={() => setSelectedKey(null)}
                      className={`w-full text-left px-3 py-2 rounded-lg text-[11px] font-bold transition-colors ${!selectedKey ? "bg-indigo-500/20 text-indigo-400" : "hover:bg-white/5 text-slate-400"}`}
                    >
                      <Activity className="w-3 h-3 inline-block mr-2" />
                      Global View
                    </button>
                    {Object.entries(snap?.results || {}).filter(([k]) => k !== "__CLUSTER__").map(([k, r]) => (
                      <button
                        key={k}
                        onClick={() => setSelectedKey(k)}
                        className={`w-full text-left px-3 py-2 rounded-lg text-[11px] font-bold transition-colors flex items-center justify-between group ${selectedKey === k ? "bg-white/10 text-white" : "hover:bg-white/5 text-slate-400"}`}
                      >
                         <div className="flex items-center gap-2 truncate">
                           <Database className="w-3 h-3 shrink-0" />
                           <span className="truncate">{k}</span>
                         </div>
                         <div className={`w-2 h-2 rounded-full shrink-0 ${r.FinalConsistent ? "bg-emerald-500" : "bg-red-500"}`} />
                      </button>
                    ))}
                 </div>
               </div>

               {/* RIGHT CONTENT AREA */}
               <div className="flex-1 flex flex-col overflow-hidden relative">
                 {/* DYNAMIC CONTENT */}
                 <div className="flex-1 overflow-y-auto custom-scrollbar p-6">
                    {selectedKey ? (
                      <KeyInspectorView 
                        snap={snap} 
                        keyName={selectedKey} 
                        result={snap?.results[selectedKey]} 
                        playbackTimeMs={maxTimeMs}
                      />
                    ) : (
                      <GlobalLabView 
                        events={globalEvents} 
                        playbackTimeMs={maxTimeMs} 
                      />
                    )}
                 </div>
               </div>
            </div>
          </div>
        )}
      </motion.div>
    </>
  );
}

// ----------------------------------------------------------------------
// HELPER COMPONENTS
// ----------------------------------------------------------------------

function MetricBox({ label, value }: { label: string, value: string | number }) {
  return (
    <div className="bg-slate-950 p-4 text-center">
      <div className="text-[20px] font-light text-white font-mono tracking-tight">{value}</div>
      <div className="text-[9px] font-black text-slate-500 uppercase tracking-widest mt-1">{label}</div>
    </div>
  );
}

function calculateGlobalDivergence(results?: Record<string, MetricsResult>) {
  if (!results) return "0.00";
  const vals = Object.values(results);
  if (vals.length === 0) return "0.00";
  const sum = vals.reduce((acc, r) => acc + (r.FinalConsistent ? 0 : 1), 0);
  return (sum / vals.length).toFixed(2);
}

function calculateAvgConvergence(results?: Record<string, MetricsResult>) {
  if (!results) return "-";
  const vals = Object.values(results).filter(r => r.ConvergenceTime && r.ConvergenceTime > 0);
  if (vals.length === 0) return "-";
  const sum = vals.reduce((acc, r) => acc + r.ConvergenceTime!, 0);
  return `${(sum / vals.length / 1000000).toFixed(1)}ms`;
}

// ----------------------------------------------------------------------
// KEY INSPECTOR VIEW (Replica Matrix + Conflict Resolution)
// ----------------------------------------------------------------------
function KeyInspectorView({ snap, keyName, result, playbackTimeMs }: { snap: any, keyName: string, result: MetricsResult, playbackTimeMs: number }) {
  if (!result) return <div className="text-slate-500 text-center py-20">Waiting for data...</div>;

  const timeline = result.Timeline || [];
  
  // Build Replica Matrix data
  // We want to know the state of every node at `playbackTimeMs`
  const nodeStates: Record<string, { value: string, version: number, lastUpdate: number }> = {};
  
  // All unique nodes that have ever had an event for this key
  const allNodes = Array.from(new Set(timeline.map(e => e.NodeID))).sort();
  allNodes.forEach(n => nodeStates[n] = { value: "-", version: 0, lastUpdate: 0 });

  let latestLWW: TimelineEvent | null = null;
  let activeConflicts: TimelineEvent[] = [];

  // Replay events up to playbackTimeMs
  for (const ev of timeline) {
    const tMs = ev.Time / 1000000;
    if (tMs > playbackTimeMs) break;

    // Process event
    if (ev.EventType === "WRITE" || ev.EventType === "GOSSIP_RECEIVE" || ev.EventType === "RESOLVE") {
      nodeStates[ev.NodeID] = { value: ev.Value, version: ev.Version, lastUpdate: tMs };
    }

    if (ev.EventType === "WRITE") {
      activeConflicts.push(ev);
    }
    if (ev.EventType === "RESOLVE") {
      latestLWW = ev;
      // If we resolved, maybe clear old active conflicts that match or just keep them for history
    }
  }

  // Determine if split brain exists currently
  const uniqueValues = new Set(Object.values(nodeStates).map(s => s.value).filter(v => v !== "-"));
  const isSplitBrain = uniqueValues.size > 1;

  return (
    <div className="space-y-8 animate-in fade-in duration-300">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-light text-white tracking-tight font-mono">{keyName}</h2>
          <div className="flex items-center gap-3 mt-2">
            <span className={`px-2 py-0.5 rounded text-[10px] font-black uppercase tracking-widest ${isSplitBrain ? "bg-red-500/20 text-red-400 border border-red-500/30" : "bg-emerald-500/20 text-emerald-400 border border-emerald-500/30"}`}>
              {isSplitBrain ? "Divergent (Split Brain)" : "Converged"}
            </span>
            <span className="text-[11px] text-slate-500 font-mono">Unique Replicas: {uniqueValues.size}</span>
          </div>
        </div>
        <div className="text-right">
          <div className="text-[32px] font-light text-indigo-400 font-mono leading-none">{playbackTimeMs.toFixed(0)}<span className="text-lg text-indigo-400/50">ms</span></div>
          <div className="text-[10px] font-black uppercase tracking-widest text-slate-500">Current Simulation Time</div>
        </div>
      </div>

      <div className="grid grid-cols-2 gap-6">
        {/* REPLICA STATE MATRIX */}
        <div className="bg-slate-900 border border-white/5 rounded-2xl p-5 shadow-xl">
           <h3 className="text-[11px] font-black uppercase tracking-widest text-slate-400 mb-4 flex items-center gap-2">
             <Database className="w-4 h-4 text-indigo-400" /> Replica State Matrix
           </h3>
           <div className="space-y-3">
              {allNodes.map(node => {
                const state = nodeStates[node];
                const isDivergent = isSplitBrain && Array.from(uniqueValues)[0] !== state.value;
                return (
                  <div key={node} className={`p-3 rounded-xl border flex items-center justify-between transition-colors ${state.value === "-" ? "bg-slate-950 border-white/5 opacity-50" : (isDivergent ? "bg-red-950/20 border-red-900/50" : "bg-emerald-950/10 border-emerald-900/30")}`}>
                     <div className="flex items-center gap-3">
                        <div className="w-8 h-8 rounded-lg bg-slate-950 flex items-center justify-center border border-white/10 font-mono text-[10px] text-slate-300">
                          {node.replace("node", "N")}
                        </div>
                        <div>
                          <div className="text-[12px] font-bold text-white font-mono">
                            {state.value === "-" ? "null" : `"${state.value}"`}
                          </div>
                          <div className="text-[9px] text-slate-500 uppercase tracking-widest mt-0.5">
                            Version: {state.version} {state.lastUpdate > 0 ? `• Updated @ ${state.lastUpdate.toFixed(1)}ms` : ""}
                          </div>
                        </div>
                     </div>
                     {isDivergent && <AlertTriangle className="w-4 h-4 text-red-500/50" />}
                     {!isDivergent && state.value !== "-" && <Check className="w-4 h-4 text-emerald-500/50" />}
                  </div>
                );
              })}
           </div>
        </div>

        {/* CONFLICT RESOLUTION INSPECTOR */}
        <div className="space-y-6">
           <div className="bg-slate-900 border border-white/5 rounded-2xl p-5 shadow-xl h-full flex flex-col">
             <h3 className="text-[11px] font-black uppercase tracking-widest text-slate-400 mb-4 flex items-center gap-2">
               <Zap className="w-4 h-4 text-violet-400" /> Conflict Resolution Inspector
             </h3>
             
             {latestLWW ? (
               <div className="flex-1 flex flex-col justify-center space-y-6">
                  <div className="text-center">
                    <div className="inline-block px-3 py-1 bg-violet-500/20 border border-violet-500/30 text-violet-300 rounded-full text-[10px] font-black uppercase tracking-widest mb-3">
                      LWW Policy Triggered
                    </div>
                    <div className="text-[14px] text-white">Winner: <span className="font-mono text-emerald-400 font-bold">"{latestLWW.Value}"</span></div>
                    <div className="text-[11px] text-slate-500 mt-2">Resolved at {(latestLWW.Time/1000000).toFixed(1)}ms across replicas.</div>
                  </div>

                  <div className="p-4 bg-slate-950 border border-white/5 rounded-xl text-[11px] font-mono space-y-2">
                     <div className="text-slate-400">Winning Timestamp: <span className="text-white">{latestLWW.LWWTimestamp}</span></div>
                     <div className="text-slate-400">Resolution Origin: <span className="text-white">{latestLWW.Source}</span></div>
                     <div className="text-slate-400">Reason: <span className="text-emerald-400">Later logical timestamp strictly dominates.</span></div>
                  </div>
               </div>
             ) : (
               <div className="flex-1 flex items-center justify-center text-center">
                  <div>
                    <Shield className="w-8 h-8 text-slate-700 mx-auto mb-3" />
                    <div className="text-[11px] text-slate-500 uppercase tracking-widest font-bold">No Conflicts Resolved Yet</div>
                    <div className="text-[10px] text-slate-600 mt-1 max-w-[200px] mx-auto">Writes must occur during a partition and subsequently heal to trigger LWW.</div>
                  </div>
               </div>
             )}
           </div>
        </div>
      </div>
    </div>
  );
}

// ----------------------------------------------------------------------
// GLOBAL LAB VIEW (Audit Stream mapped against time)
// ----------------------------------------------------------------------
function GlobalLabView({ events }: { events: any[] }) {
  // Filter events (all events are shown)
  const visibleEvents = events.slice().reverse(); // Newest first
  const allKnownNodes = Array.from(new Set(events.map(e => e.NodeID).filter(n => n && n !== "SYSTEM"))).sort((a: any, b: any) => {
    return (parseInt(a.replace(/[^\d]/g, '')) || 0) - (parseInt(b.replace(/[^\d]/g, '')) || 0);
  }) as string[];
  const [zoomedDiagramData, setZoomedDiagramData] = useState<{eventsUpToNow: any[], activeEvent: any} | null>(null);

  return (
    <div className="max-w-5xl mx-auto animate-in fade-in duration-300">
      <AnimatePresence>
        {zoomedDiagramData && (
          <div className="fixed inset-0 z-[100] flex items-center justify-center p-8 bg-black/90 backdrop-blur-md" onClick={() => setZoomedDiagramData(null)}>
            <motion.div 
              initial={{scale:0.95, opacity:0}} 
              animate={{scale:1, opacity:1}} 
              exit={{scale:0.95, opacity:0}} 
              onClick={e => e.stopPropagation()} 
              className="bg-slate-900 border border-indigo-500/30 rounded-[2rem] p-10 max-w-5xl w-full flex flex-col items-center shadow-[0_0_100px_rgba(99,102,241,0.2)] relative"
            >
              <button onClick={() => setZoomedDiagramData(null)} className="absolute top-6 right-6 p-2 text-slate-500 hover:text-white hover:bg-white/10 rounded-full transition-colors">
                <X className="w-6 h-6" />
              </button>
              
              <div className="w-full mb-8 text-center">
                <h3 className="text-2xl font-light text-white tracking-widest uppercase mb-2">Topology State Inspection</h3>
                <div className="text-slate-400 font-mono text-sm">
                  {getEventActionText(zoomedDiagramData.activeEvent.EventType, zoomedDiagramData.activeEvent.Source, zoomedDiagramData.activeEvent.Origin)} @ {(zoomedDiagramData.activeEvent.Time / 1000000).toFixed(1)}ms
                </div>
              </div>

              <div className="w-full bg-slate-950 rounded-3xl border border-white/5 p-8 flex justify-center shadow-inner">
                <LargeClusterDiagram eventsUpToNow={zoomedDiagramData.eventsUpToNow} activeEvent={zoomedDiagramData.activeEvent} allKnownNodes={allKnownNodes} />
              </div>
            </motion.div>
          </div>
        )}
      </AnimatePresence>
       <div className="flex items-center justify-between mb-8">
         <h2 className="text-xl font-light text-white tracking-tight flex items-center gap-3">
           <Network className="w-6 h-6 text-indigo-400" /> Execution Narrative
         </h2>
         <div className="px-3 py-1.5 bg-slate-900 border border-white/10 rounded-lg text-[11px] font-mono text-slate-400">
           Showing <span className="text-white font-bold">{visibleEvents.length}</span> critical transitions
         </div>
       </div>

       <div className="relative border-l-2 border-white/10 ml-4 pl-8 space-y-8 pb-20">
          {visibleEvents.map((ev, i) => {
            const timeMs = (ev.Time / 1000000).toFixed(1);
            let color = "text-slate-400 bg-slate-900 border-white/10";
            let icon = <Activity className="w-3.5 h-3.5" />;
            let title = getEventActionText(ev.EventType, ev.Source, ev.Origin);
            let highlight = false;

            if (ev.EventType === "PARTITION" || ev.EventType === "CRASH") {
              color = "text-red-400 bg-red-950/50 border-red-900";
              icon = <ShieldAlert className="w-3.5 h-3.5" />;
              highlight = true;
            } else if (ev.EventType === "HEAL" || ev.EventType === "RECOVER") {
              color = "text-emerald-400 bg-emerald-950/50 border-emerald-900";
              icon = <Shield className="w-3.5 h-3.5" />;
              highlight = true;
            } else if (ev.EventType === "RESOLVE") {
              color = "text-violet-400 bg-violet-950/50 border-violet-900";
              icon = <Check className="w-3.5 h-3.5" />;
              highlight = true;
            } else if (ev.EventType === "WRITE") {
              color = "text-blue-400 bg-blue-950/50 border-blue-900";
              icon = <Database className="w-3.5 h-3.5" />;
            }

            // Calculate state up to this EXACT event
            // Note: visibleEvents is reversed, so we need to filter from the original events array
            const eventsUpToThis = events.filter(e => e.Time <= ev.Time);

            return (
              <div key={i} className="relative">
                 {/* Timeline dot */}
                 <div className={`absolute -left-[41px] top-6 w-5 h-5 rounded-full border-2 ${color} flex items-center justify-center bg-slate-950 shadow-[0_0_10px_rgba(0,0,0,0.5)] z-10`}>
                   <div className="w-1.5 h-1.5 rounded-full bg-current" />
                 </div>

                 {/* Card */}
                 <div className={`p-5 rounded-2xl border transition-all ${color} flex gap-6 items-center ${highlight ? "shadow-xl shadow-current/5" : ""}`}>
                   <div className="flex-1">
                     <div className="flex items-center justify-between mb-4">
                       <div className="flex items-center gap-2">
                         {icon}
                         <span className="text-[12px] font-black uppercase tracking-widest">{title}</span>
                         {ev.key !== "SYSTEM" && (
                           <span className="px-2 py-0.5 bg-slate-950 rounded text-[10px] font-mono text-slate-300 border border-white/5">{ev.key}</span>
                         )}
                       </div>
                       <span className="font-mono text-[12px] font-bold opacity-70 bg-black/20 px-2 py-1 rounded">{timeMs}ms</span>
                     </div>

                     <div className="text-[13px] text-slate-300">
                       {ev.key === "SYSTEM" ? (
                         <span className="font-medium text-[14px] text-white">{ev.Value}</span>
                       ) : (
                         <div className="flex flex-col gap-2">
                           {ev.EventType === "WRITE" && (
                             <div>Client wrote value <span className="font-mono font-bold text-white bg-white/10 px-1 rounded">"{ev.Value}"</span> to <span className="font-mono text-indigo-300">{ev.NodeID}</span>.</div>
                           )}
                           {ev.EventType === "RESOLVE" && (
                             <div>Replicas synchronized. State converged to <span className="font-mono font-bold text-white bg-white/10 px-1 rounded">"{ev.Value}"</span> on <span className="font-mono text-indigo-300">{ev.NodeID}</span>.</div>
                           )}
                           {ev.Source && ev.Source !== ev.NodeID && (
                             <div className="flex items-center gap-2 text-[11px] text-slate-400 font-mono mt-2 bg-black/20 p-2 rounded w-fit border border-white/5">
                                <span className="uppercase tracking-widest text-[9px] font-black text-slate-500">Flow:</span>
                                <span>{ev.Source}</span>
                                <ArrowRight className="w-3 h-3" />
                                <span className="text-white">{ev.NodeID}</span>
                             </div>
                           )}
                         </div>
                       )}
                     </div>
                     
                     <div className="mt-5 pt-4 border-t border-white/10">
                        <div className="text-[9px] font-black uppercase tracking-widest text-slate-500 mb-2">Cluster Key State</div>
                        <ClusterStateSummary eventsUpToNow={eventsUpToThis} />
                     </div>
                   </div>

                   <div 
                     className="shrink-0 bg-slate-950 p-2 rounded-xl border border-white/5 shadow-inner cursor-pointer hover:ring-2 hover:ring-indigo-500/50 hover:bg-slate-900 transition-all group/diagram"
                     onClick={() => setZoomedDiagramData({eventsUpToNow: eventsUpToThis, activeEvent: ev})}
                   >
                      <div className="relative">
                        <MiniClusterDiagram eventsUpToNow={eventsUpToThis} activeEvent={ev} allKnownNodes={allKnownNodes} />
                        <div className="absolute inset-0 flex items-center justify-center opacity-0 group-hover/diagram:opacity-100 transition-opacity bg-black/40 rounded-lg">
                          <Maximize2 className="w-6 h-6 text-white" />
                        </div>
                      </div>
                   </div>
                 </div>
              </div>
            );
          })}
          {visibleEvents.length === 0 && (
             <div className="text-slate-500 text-[11px] font-bold uppercase tracking-widest pt-4">No events in this timeframe</div>
          )}
       </div>
    </div>
  );
}

function ClusterStateSummary({ eventsUpToNow }: { eventsUpToNow: any[] }) {
  const nodeValues: Record<string, string> = {};
  for (const e of eventsUpToNow) {
    if ((e.EventType === "WRITE" || e.EventType === "RESOLVE" || e.EventType === "GOSSIP_RECEIVE") && e.NodeID && e.key !== "SYSTEM") {
      nodeValues[e.NodeID] = e.Value;
    }
  }

  const valueGroups: Record<string, string[]> = {};
  for (const [node, val] of Object.entries(nodeValues)) {
    if (!val) continue;
    if (!valueGroups[val]) valueGroups[val] = [];
    valueGroups[val].push(node.replace("node", "N"));
  }

  const entries = Object.entries(valueGroups);
  if (entries.length === 0) return <span className="text-slate-500 text-[10px] font-mono">All replicas empty</span>;

  return (
    <div className="flex flex-col gap-1.5">
      {entries.map(([val, nodes]) => (
        <div key={val} className="text-[11px] font-mono flex items-center gap-2">
          <span className="text-white font-bold bg-white/10 px-1.5 rounded">{nodes.sort().join(", ")}</span> 
          <span className="text-slate-500">hold</span> 
          <span className="text-indigo-400 font-bold border border-indigo-500/30 px-1.5 rounded bg-indigo-500/10">"{val}"</span>
        </div>
      ))}
    </div>
  );
}

function MiniClusterDiagram({ eventsUpToNow, activeEvent, allKnownNodes }: { eventsUpToNow: any[], activeEvent: any, allKnownNodes: string[] }) {
  let isPartitioned = false;
  const nodeValues: Record<string, string> = {};
  
  for (const e of eventsUpToNow) {
    if (e.EventType === "PARTITION") isPartitioned = true;
    if (e.EventType === "HEAL") isPartitioned = false;
    if ((e.EventType === "WRITE" || e.EventType === "RESOLVE" || e.EventType === "GOSSIP_RECEIVE") && e.NodeID && e.key !== "SYSTEM") {
      nodeValues[e.NodeID] = e.Value;
    }
  }

  const uniqueVals = Array.from(new Set(Object.values(nodeValues).filter(v => v)));
  const getColorForValue = (val: string) => {
    const colors = ["#3b82f6", "#8b5cf6", "#f59e0b", "#ec4899", "#10b981"];
    const idx = uniqueVals.indexOf(val);
    return colors[idx % colors.length] || "#64748b";
  };

  if (!allKnownNodes || allKnownNodes.length === 0) return null;

  const centerX = 100;
  const centerY = 60;
  const radius = 40;
  
  const nodes = allKnownNodes.map((id, i) => {
    const angle = (i / allKnownNodes.length) * 2 * Math.PI - Math.PI / 2; // start top
    return { id, x: centerX + radius * Math.cos(angle), y: centerY + radius * Math.sin(angle) };
  });

  return (
    <svg width="200" height="120" className="opacity-90 hover:opacity-100 transition-opacity bg-slate-900 border border-white/5 rounded-xl shrink-0 overflow-visible">
      {/* Edges */}
      {!isPartitioned ? (
        nodes.map((n1, i) => 
          nodes.slice(i+1).map((n2, j) => (
             <line key={`edge-${i}-${j}`} x1={n1.x} y1={n1.y} x2={n2.x} y2={n2.y} stroke="#334155" strokeWidth="1" opacity="0.3" />
          ))
        )
      ) : (
        <>
          <line x1={centerX} y1={centerY - radius - 15} x2={centerX} y2={centerY + radius + 15} stroke="#ef4444" strokeWidth="3" strokeDasharray="4 4" className="animate-pulse" />
          {nodes.map((n1, i) => 
            nodes.slice(i+1).map((n2, j) => {
              if ((n1.x <= centerX && n2.x <= centerX) || (n1.x > centerX && n2.x > centerX)) {
                return <line key={`edge-${i}-${j}`} x1={n1.x} y1={n1.y} x2={n2.x} y2={n2.y} stroke="#334155" strokeWidth="1" opacity="0.3" />;
              }
              return null;
            })
          )}
        </>
      )}

      {/* Nodes */}
      {nodes.map(n => {
        const val = nodeValues[n.id];
        const color = val ? getColorForValue(val) : "#1e293b";
        const isTarget = activeEvent?.NodeID === n.id;
        
        return (
          <g key={n.id} transform={`translate(${n.x}, ${n.y})`}>
            {isTarget && (
              <circle r="16" fill="none" stroke={color} strokeWidth="2" className="animate-ping opacity-60" />
            )}
            <circle r="12" fill={color} stroke={isTarget ? "#fff" : "#0f172a"} strokeWidth="2" />
            <text y="1" textAnchor="middle" alignmentBaseline="middle" fill="#fff" fontSize="8" fontWeight="bold">
              {n.id.replace("node", "N")}
            </text>
            {val && (
              <text y="-16" textAnchor="middle" fill={color} fontSize="9" fontWeight="bold">
                {val}
              </text>
            )}
          </g>
        );
      })}
    </svg>
  );
}

function LargeClusterDiagram({ eventsUpToNow, activeEvent, allKnownNodes }: { eventsUpToNow: any[], activeEvent: any, allKnownNodes: string[] }) {
  let isPartitioned = false;
  const nodeValues: Record<string, string> = {};
  
  for (const e of eventsUpToNow) {
    if (e.EventType === "PARTITION") isPartitioned = true;
    if (e.EventType === "HEAL") isPartitioned = false;
    if ((e.EventType === "WRITE" || e.EventType === "RESOLVE" || e.EventType === "GOSSIP_RECEIVE") && e.NodeID && e.key !== "SYSTEM") {
      nodeValues[e.NodeID] = e.Value;
    }
  }

  const uniqueVals = Array.from(new Set(Object.values(nodeValues).filter(v => v)));
  const getColorForValue = (val: string) => {
    const colors = ["#3b82f6", "#8b5cf6", "#f59e0b", "#ec4899", "#10b981"];
    const idx = uniqueVals.indexOf(val);
    return colors[idx % colors.length] || "#64748b";
  };

  if (!allKnownNodes || allKnownNodes.length === 0) return null;

  const width = 800;
  const height = 500;
  const centerX = width / 2;
  const centerY = height / 2;
  const radius = 180;
  
  const nodes = allKnownNodes.map((id, i) => {
    const angle = (i / allKnownNodes.length) * 2 * Math.PI - Math.PI / 2; // start top
    return { id, x: centerX + radius * Math.cos(angle), y: centerY + radius * Math.sin(angle) };
  });

  return (
    <svg width={width} height={height} className="overflow-visible">
      <defs>
        <filter id="glow">
          <feGaussianBlur stdDeviation="4" result="coloredBlur"/>
          <feMerge>
            <feMergeNode in="coloredBlur"/>
            <feMergeNode in="SourceGraphic"/>
          </feMerge>
        </filter>
      </defs>

      {/* Edges */}
      {!isPartitioned ? (
        nodes.map((n1, i) => 
          nodes.slice(i+1).map((n2, j) => (
             <line key={`edge-${i}-${j}`} x1={n1.x} y1={n1.y} x2={n2.x} y2={n2.y} stroke="#334155" strokeWidth="2" opacity="0.3" strokeDasharray="6 6" />
          ))
        )
      ) : (
        <>
          <line x1={centerX} y1={centerY - radius - 40} x2={centerX} y2={centerY + radius + 40} stroke="#ef4444" strokeWidth="6" strokeDasharray="12 12" className="animate-pulse" filter="url(#glow)" />
          {nodes.map((n1, i) => 
            nodes.slice(i+1).map((n2, j) => {
              // Left vs Right check
              if ((n1.x <= centerX && n2.x <= centerX) || (n1.x > centerX && n2.x > centerX)) {
                return <line key={`edge-${i}-${j}`} x1={n1.x} y1={n1.y} x2={n2.x} y2={n2.y} stroke="#334155" strokeWidth="2" opacity="0.3" strokeDasharray="6 6" />;
              }
              return null;
            })
          )}
        </>
      )}

      {/* Nodes */}
      {nodes.map(n => {
        const val = nodeValues[n.id];
        const color = val ? getColorForValue(val) : "#1e293b";
        const isTarget = activeEvent?.NodeID === n.id;
        const angle = Math.atan2(n.y - centerY, n.x - centerX);
        const tx = Math.cos(angle) * 60;
        const ty = Math.sin(angle) * 60;
        let anchor = "middle";
        if (Math.cos(angle) > 0.1) anchor = "start";
        else if (Math.cos(angle) < -0.1) anchor = "end";
        
        return (
          <g key={n.id} transform={`translate(${n.x}, ${n.y})`}>
            {isTarget && (
              <circle r="45" fill="none" stroke={color} strokeWidth="4" className="animate-ping opacity-60" />
            )}
            <circle r="36" fill={color} stroke={isTarget ? "#fff" : "#0f172a"} strokeWidth="6" filter="url(#glow)" />
            <text y="3" textAnchor="middle" alignmentBaseline="middle" fill="#fff" fontSize="20" fontWeight="900" fontFamily="monospace">
              {n.id.replace("node", "N")}
            </text>
            {val && (
              <text x={tx} y={ty} textAnchor={anchor} alignmentBaseline="middle" fill={color} fontSize="18" fontWeight="bold" fontFamily="monospace" filter="url(#glow)">
                {val}
              </text>
            )}
          </g>
        );
      })}
    </svg>
  );
}

// ----------------------------------------------------------------------
// UTILS
// ----------------------------------------------------------------------
function getEventActionText(type: string, source: string, origin: string) {
  switch (type) {
    case "WRITE": return "ACCEPTED WRITE";
    case "GOSSIP_RECEIVE": return "GOSSIP UPDATE";
    case "RESOLVE": return "CONFLICT RESOLVED (LWW)";
    case "NODE_JOIN": return "JOINED CLUSTER";
    case "PARTITION": return "PARTITIONED";
    case "HEAL": return "PARTITION HEALED";
    case "CRASH": return "CRASHED";
    case "RECOVER": return "RECOVERED";
    default: return "ACTION";
  }
}

function generatePDFReport(snap: any, globalEvents: any[]) {
  if (!snap) return;

  const totalEvents = globalEvents.length;
  const timeRangeMs = totalEvents > 0 ? (globalEvents[0].Time / 1000000) : 0;
  const timeRange = timeRangeMs.toFixed(1);
  const divScore = calculateGlobalDivergence(snap.results);
  const avgConv = calculateAvgConvergence(snap.results);

  // 1. Partition Window
  let partitionStart = 0;
  let partitionEnd = timeRangeMs;
  let hasPartition = false;
  for (const ev of [...globalEvents].reverse()) {
    if (ev.EventType === "PARTITION" && !hasPartition) {
       partitionStart = ev.Time / 1000000;
       hasPartition = true;
    }
    if (ev.EventType === "HEAL" && hasPartition) {
       partitionEnd = ev.Time / 1000000;
    }
  }
  const partitionDuration = hasPartition ? (partitionEnd - partitionStart).toFixed(1) : 0;

  // 2. Heatmap Data
  const heatmapRows = [];
  const allKnownNodes = Array.from(new Set(globalEvents.map(e => e.NodeID).filter(n => n && n !== "SYSTEM"))).sort((a: any, b: any) => {
    return (parseInt(a.replace(/[^\d]/g, '')) || 0) - (parseInt(b.replace(/[^\d]/g, '')) || 0);
  }) as string[];
  const nodeValues: Record<string, string> = {};
  allKnownNodes.forEach(n => nodeValues[n] = "");

  let lastTimeMs = -1;
  const chronologicalEvents = [...globalEvents].reverse();
  
  for (const ev of chronologicalEvents) {
    const timeMs = ev.Time / 1000000;
    if ((ev.EventType === "WRITE" || ev.EventType === "RESOLVE" || ev.EventType === "GOSSIP_RECEIVE") && ev.NodeID && ev.key !== "SYSTEM") {
      nodeValues[ev.NodeID] = ev.Value;
      if (Math.abs(timeMs - lastTimeMs) > 1.0) { // Sample every 1ms at most for heatmap
        heatmapRows.push({ time: timeMs, state: { ...nodeValues } });
        lastTimeMs = timeMs;
      }
    }
  }

  let heatmapHTML = `
    <table class="heatmap-table">
      <thead>
        <tr>
          <th>Time</th>
          ${allKnownNodes.map(n => `<th>${n.replace("node", "N")}</th>`).join("")}
        </tr>
      </thead>
      <tbody>
  `;
  for (const row of heatmapRows) {
    const vals = Object.values(row.state).filter(v => v);
    let majorityVal = "";
    if (vals.length > 0) {
      const counts: Record<string, number> = {};
      vals.forEach(v => counts[v] = (counts[v] || 0) + 1);
      majorityVal = Object.keys(counts).reduce((a, b) => counts[a] > counts[b] ? a : b);
    }
    const isConverged = vals.length === allKnownNodes.length && vals.every(v => v === majorityVal);
    const isSplit = new Set(vals).size > 1;

    heatmapHTML += `<tr><td>${row.time.toFixed(1)}ms</td>`;
    for (const n of allKnownNodes) {
       const v = row.state[n];
       let cellClass = "empty";
       if (v) {
         if (isConverged) cellClass = "converged";
         else if (isSplit && v === majorityVal) cellClass = "stale";
         else if (isSplit && v !== majorityVal) cellClass = "divergent";
         else cellClass = "stale";
       }
       heatmapHTML += `<td class="${cellClass}">${v || "-"}</td>`;
    }
    heatmapHTML += `</tr>`;
  }
  heatmapHTML += `</tbody></table>`;

  // 3. Convergence Waterfall
  let waterfallHTML = "<div style='color: #64748b; font-size: 13px; font-style: italic;'>No partition healed during this window.</div>";
  if (hasPartition) {
    const convergeTimes: Record<string, number> = {};
    for (const ev of chronologicalEvents) {
      const timeMs = ev.Time / 1000000;
      if (timeMs >= partitionEnd && ev.EventType === "RESOLVE" && ev.NodeID) {
        if (!convergeTimes[ev.NodeID]) {
           convergeTimes[ev.NodeID] = timeMs - partitionEnd;
        }
      }
    }
    
    if (Object.keys(convergeTimes).length > 0) {
      waterfallHTML = `<div class="waterfall">`;
      const maxConv = Math.max(...Object.values(convergeTimes));
      for (const [nodeId, cTime] of Object.entries(convergeTimes).sort((a,b) => a[1] - b[1])) {
        const pct = maxConv > 0 ? (cTime / maxConv) * 100 : 100;
        waterfallHTML += `
          <div class="waterfall-row">
            <span class="w-label">${nodeId.replace('node','N')}</span>
            <div class="w-track"><div class="w-bar" style="width: ${pct}%;"></div></div>
            <span class="w-time">${cTime.toFixed(1)}ms</span>
          </div>`;
      }
      waterfallHTML += `</div>`;
    }
  }

  const reportHTML = `
    <!DOCTYPE html>
    <html lang="en">
    <head>
      <meta charset="UTF-8">
      <title>Post-Mortem Investigation - ${snap.clusterId}</title>
      <style>
        body { font-family: 'Inter', -apple-system, sans-serif; padding: 40px; color: #1e293b; line-height: 1.6; max-width: 900px; margin: 0 auto; background: #f8fafc; }
        .header { border-bottom: 2px solid #cbd5e1; padding-bottom: 20px; margin-bottom: 30px; }
        h1 { font-size: 28px; font-weight: 900; letter-spacing: -0.5px; margin: 0 0 8px 0; color: #0f172a; text-transform: uppercase; }
        .meta { color: #64748b; font-size: 13px; font-family: monospace; display: flex; gap: 16px; }
        .meta span { background: #e2e8f0; padding: 4px 8px; border-radius: 4px; color: #334155; font-weight: bold; }
        
        .section-title { font-size: 18px; font-weight: 800; margin: 40px 0 16px 0; border-bottom: 1px solid #e2e8f0; padding-bottom: 8px; text-transform: uppercase; letter-spacing: 1px; color: #334155; }
        
        .metrics-grid { display: grid; grid-template-columns: repeat(4, 1fr); gap: 16px; margin-bottom: 30px; }
        .metric-box { padding: 16px; background: #fff; border: 1px solid #cbd5e1; border-radius: 8px; text-align: center; box-shadow: 0 2px 4px rgba(0,0,0,0.02); }
        .metric-value { font-size: 24px; font-weight: 800; font-family: monospace; color: #0f172a; }
        .metric-label { font-size: 10px; font-weight: 800; text-transform: uppercase; color: #64748b; letter-spacing: 1px; margin-top: 4px; }
        
        /* Heatmap */
        .heatmap-table { width: 100%; border-collapse: collapse; background: #fff; font-family: monospace; font-size: 12px; margin-bottom: 30px; border: 1px solid #cbd5e1; box-shadow: 0 2px 4px rgba(0,0,0,0.02); }
        .heatmap-table th { background: #f1f5f9; padding: 12px; text-align: left; border-bottom: 2px solid #cbd5e1; color: #334155; }
        .heatmap-table td { padding: 10px 12px; border-bottom: 1px solid #e2e8f0; font-weight: bold; }
        .heatmap-table .converged { color: #10b981; background: #ecfdf5; }
        .heatmap-table .divergent { color: #ef4444; background: #fef2f2; }
        .heatmap-table .stale { color: #f59e0b; background: #fffbeb; }
        .heatmap-table .empty { color: #cbd5e1; font-weight: normal; }

        /* Waterfall */
        .waterfall { display: flex; flex-direction: column; gap: 8px; background: #fff; padding: 20px; border: 1px solid #cbd5e1; border-radius: 8px; }
        .waterfall-row { display: flex; align-items: center; gap: 12px; font-family: monospace; font-size: 12px; font-weight: bold; }
        .w-label { width: 30px; color: #475569; }
        .w-track { flex: 1; background: #f1f5f9; height: 12px; border-radius: 6px; overflow: hidden; }
        .w-bar { height: 100%; background: #8b5cf6; }
        .w-time { width: 60px; text-align: right; color: #8b5cf6; }

        /* Timeline */
        .timeline { margin-top: 20px; border-left: 2px solid #cbd5e1; padding-left: 24px; }
        .event { margin-bottom: 32px; position: relative; }
        .event::before { content: ''; position: absolute; left: -31px; top: 4px; width: 12px; height: 12px; border-radius: 50%; background: #fff; border: 2px solid #94a3b8; }
        .event.PARTITION::before, .event.CRASH::before { border-color: #ef4444; background: #fef2f2; }
        .event.HEAL::before, .event.RECOVER::before { border-color: #10b981; background: #ecfdf5; }
        .event.WRITE::before { border-color: #3b82f6; background: #eff6ff; }
        .event.RESOLVE::before { border-color: #8b5cf6; background: #f5f3ff; }
        
        .time { font-family: monospace; font-size: 12px; font-weight: 700; color: #64748b; margin-bottom: 4px; }
        .action { font-size: 14px; font-weight: 800; margin: 0 0 8px 0; color: #0f172a; display: flex; align-items: center; gap: 8px; }
        .details { font-size: 13px; color: #475569; }
        .key-badge { padding: 2px 6px; background: #f1f5f9; border-radius: 4px; font-family: monospace; font-size: 10px; font-weight: bold; border: 1px solid #cbd5e1; }
        
        .event-card { display: flex; gap: 24px; align-items: flex-start; background: #fff; padding: 20px; border-radius: 8px; border: 1px solid #cbd5e1; box-shadow: 0 2px 4px rgba(0,0,0,0.02); }
        .event-text { flex: 1; }
        .event-diagram { flex-shrink: 0; }

        /* Edu Block */
        .edu-block { margin-top: 16px; background: #f8fafc; border: 1px solid #e2e8f0; border-left: 3px solid #8b5cf6; padding: 12px 16px; border-radius: 4px; font-size: 12px; color: #334155; }
        .edu-title { font-weight: 800; text-transform: uppercase; font-size: 10px; letter-spacing: 1px; color: #8b5cf6; margin-bottom: 8px; }
        .edu-grid { display: grid; grid-template-columns: auto 1fr; gap: 6px 16px; }
        .edu-label { font-weight: bold; color: #475569; }
        .edu-val { font-family: monospace; font-weight: bold; color: #0f172a; }

        /* Partition Window */
        .partition-window { background: #fff; border: 1px solid #cbd5e1; padding: 20px; border-radius: 8px; margin-bottom: 30px; }
        .pw-track { height: 24px; background: #ecfdf5; border-radius: 12px; position: relative; border: 1px solid #a7f3d0; margin-top: 12px; }
        .pw-fill { position: absolute; height: 100%; background: #fef2f2; border-left: 2px solid #ef4444; border-right: 2px solid #ef4444; }
        .pw-labels { display: flex; justify-content: space-between; font-family: monospace; font-size: 11px; font-weight: bold; color: #64748b; margin-top: 8px; }

        @media print {
          body { padding: 0; background: #fff; }
          .event-card, .metric-box, .waterfall, .heatmap-table, .partition-window { box-shadow: none; break-inside: avoid; }
        }
      </style>
    </head>
    <body>
      <div class="header">
        <h1>Incident Post-Mortem Report</h1>
        <div class="meta">
          <span>DATE: ${new Date().toLocaleDateString()}</span>
          <span>CLUSTER: ${snap.clusterId}</span>
          <span>DURATION: ${timeRange}ms</span>
        </div>
      </div>
      
      <div class="metrics-grid">
        <div class="metric-box">
          <div class="metric-value">${totalEvents}</div>
          <div class="metric-label">Total Events</div>
        </div>
        <div class="metric-box">
          <div class="metric-value">${divScore}</div>
          <div class="metric-label">Max Divergence Score</div>
        </div>
        <div class="metric-box">
          <div class="metric-value">${avgConv}</div>
          <div class="metric-label">Avg Convergence Time</div>
        </div>
        <div class="metric-box">
          <div class="metric-value">${partitionDuration}ms</div>
          <div class="metric-label">Total Partition Time</div>
        </div>
      </div>

      <h2 class="section-title">Network Partition Window</h2>
      <div class="partition-window">
        <div style="font-size: 13px; color: #475569; font-weight: 500;">Simulation timeline highlighting periods of network isolation.</div>
        <div class="pw-track">
          ${hasPartition ? `<div class="pw-fill" style="left: ${(partitionStart/timeRangeMs)*100}%; width: ${((partitionEnd-partitionStart)/timeRangeMs)*100}%;"></div>` : ''}
        </div>
        <div class="pw-labels">
          <span>0ms (Init)</span>
          ${hasPartition ? `<span style="color: #ef4444;">${partitionStart.toFixed(1)}ms (Split)</span><span style="color: #10b981;">${partitionEnd.toFixed(1)}ms (Heal)</span>` : ''}
          <span>${timeRange}ms (End)</span>
        </div>
      </div>

      <h2 class="section-title">Cluster State Heatmap</h2>
      <div style="margin-bottom: 12px; font-size: 13px; color: #475569;">Color Key: <span style="color: #10b981; font-weight: bold;">Green (Converged)</span> | <span style="color: #ef4444; font-weight: bold;">Red (Divergent)</span> | <span style="color: #f59e0b; font-weight: bold;">Orange (Stale)</span></div>
      ${heatmapHTML}

      <h2 class="section-title">Convergence Waterfall</h2>
      <div style="margin-bottom: 12px; font-size: 13px; color: #475569;">Time taken for each replica to synchronize after the network partition healed.</div>
      ${waterfallHTML}

      <h2 class="section-title">Execution Narrative (Causal Graph)</h2>
      <div class="timeline">
        ${globalEvents.slice().reverse().map(ev => {
          const time = (ev.Time / 1000000).toFixed(1);
          const action = getEventActionText(ev.EventType, ev.Source, ev.Origin);
          const keyBadge = ev.key !== "SYSTEM" ? `<span class="key-badge">${ev.key}</span>` : '';
          
          let details = ev.Value;
          let eduBlock = "";

          if (ev.key !== "SYSTEM") {
             if (ev.EventType === "WRITE") {
                 details = `Client accepted write for value "<span style="font-family: monospace; font-weight: bold; color: #3b82f6;">${ev.Value}</span>" directly to node <span style="font-family: monospace; font-weight: bold;">${ev.NodeID}</span>.`;
             } else if (ev.EventType === "RESOLVE") {
                 details = `Replica synchronized.`;
                 eduBlock = `
                    <div class="edu-block">
                      <div class="edu-title">Distributed Systems Concept</div>
                      <div class="edu-grid">
                        <div class="edu-label">Conflict Cause:</div><div class="edu-val" style="font-family: inherit; font-weight: normal;">Concurrent isolated writes during partition</div>
                        <div class="edu-label">Winning Node:</div><div class="edu-val">${ev.Source || ev.NodeID}</div>
                        <div class="edu-label">Winning Value:</div><div class="edu-val">"${ev.Value}"</div>
                        <div class="edu-label">Policy:</div><div class="edu-val" style="font-family: inherit; font-weight: normal;">Last-Write-Wins (LWW)</div>
                        <div class="edu-label">Reason:</div><div class="edu-val" style="font-family: inherit; font-weight: normal; color: #64748b;">Gossip metadata carried a strictly greater logical clock timestamp.</div>
                      </div>
                    </div>
                 `;
             } else {
                 details = `"${ev.Value}" on ${ev.NodeID}`;
             }
          }

          const eventsUpToThis = globalEvents.filter(e => e.Time <= ev.Time);
          const diagramHtml = generateMiniClusterSVGHTML(eventsUpToThis, ev, allKnownNodes);

          return `
            <div class="event ${ev.EventType}">
              <div class="time">${time}ms</div>
              <div class="event-card">
                <div class="event-text">
                  <div class="action">${action} ${keyBadge}</div>
                  <div class="details">${details}</div>
                  ${eduBlock}
                </div>
                <div class="event-diagram">
                  ${diagramHtml}
                </div>
              </div>
            </div>
          `;
        }).join('')}
      </div>

      <div style="margin-top: 80px; padding-top: 20px; border-top: 1px solid #e2e8f0; text-align: center; font-size: 11px; color: #94a3b8; font-family: monospace; letter-spacing: 1px;">
        GENERATED BY FAULTLAB DISTRIBUTED SYSTEMS OBSERVABILITY SUITE
      </div>
      
      <script>
        window.onload = function() { window.print(); }
      </script>
    </body>
    </html>
  `;

  const blob = new Blob([reportHTML], { type: 'text/html' });
  const url = URL.createObjectURL(blob);
  window.open(url, '_blank');
}

function generateMiniClusterSVGHTML(eventsUpToNow: any[], activeEvent: any, allKnownNodes: string[]) {
  let isPartitioned = false;
  const nodeValues: Record<string, string> = {};
  
  for (const e of eventsUpToNow) {
    if (e.EventType === "PARTITION") isPartitioned = true;
    if (e.EventType === "HEAL") isPartitioned = false;
    if ((e.EventType === "WRITE" || e.EventType === "RESOLVE" || e.EventType === "GOSSIP_RECEIVE") && e.NodeID && e.key !== "SYSTEM") {
      nodeValues[e.NodeID] = e.Value;
    }
  }

  const uniqueVals = Array.from(new Set(Object.values(nodeValues).filter(v => v)));
  const getColorForValue = (val: string) => {
    const colors = ["#3b82f6", "#8b5cf6", "#f59e0b", "#ec4899", "#10b981"];
    const idx = uniqueVals.indexOf(val);
    return colors[idx % colors.length] || "#64748b";
  };

  if (!allKnownNodes || allKnownNodes.length === 0) return "";

  const centerX = 100;
  const centerY = 60;
  const radius = 40;
  
  const nodes = allKnownNodes.map((id, i) => {
    const angle = (i / allKnownNodes.length) * 2 * Math.PI - Math.PI / 2; // start top
    return { id, x: centerX + radius * Math.cos(angle), y: centerY + radius * Math.sin(angle) };
  });

  let svgContent = `
    <svg width="200" height="120" style="background: #f8fafc; border-radius: 8px; border: 1px solid #e2e8f0;">
  `;

  if (!isPartitioned) {
    for (let i = 0; i < nodes.length; i++) {
      for (let j = i + 1; j < nodes.length; j++) {
        svgContent += `<line x1="${nodes[i].x}" y1="${nodes[i].y}" x2="${nodes[j].x}" y2="${nodes[j].y}" stroke="#cbd5e1" stroke-width="1" opacity="0.5" />`;
      }
    }
  } else {
    svgContent += `<line x1="${centerX}" y1="${centerY - radius - 15}" x2="${centerX}" y2="${centerY + radius + 15}" stroke="#ef4444" stroke-width="3" stroke-dasharray="4 4" />`;
    for (let i = 0; i < nodes.length; i++) {
      for (let j = i + 1; j < nodes.length; j++) {
        if ((nodes[i].x <= centerX && nodes[j].x <= centerX) || (nodes[i].x > centerX && nodes[j].x > centerX)) {
          svgContent += `<line x1="${nodes[i].x}" y1="${nodes[i].y}" x2="${nodes[j].x}" y2="${nodes[j].y}" stroke="#cbd5e1" stroke-width="1" opacity="0.5" />`;
        }
      }
    }
  }

  for (const n of nodes) {
    const val = nodeValues[n.id];
    const color = val ? getColorForValue(val) : "#94a3b8";
    const isTarget = activeEvent?.NodeID === n.id;
    
    svgContent += `<g transform="translate(${n.x}, ${n.y})">`;
    if (isTarget) {
      svgContent += `<circle r="16" fill="none" stroke="${color}" stroke-width="2" opacity="0.4" />`;
    }
    svgContent += `<circle r="12" fill="${color}" stroke="${isTarget ? '#1e293b' : '#fff'}" stroke-width="2" />`;
    svgContent += `<text y="2" text-anchor="middle" alignment-baseline="middle" fill="#fff" font-size="8" font-weight="bold" font-family="sans-serif">${n.id.replace("node", "N")}</text>`;
    if (val) {
      svgContent += `<text y="-16" text-anchor="middle" fill="${color}" font-size="9" font-weight="bold" font-family="sans-serif">${val}</text>`;
    }
    svgContent += `</g>`;
  }

  svgContent += `</svg>`;
  return svgContent;
}
