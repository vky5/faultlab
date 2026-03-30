import React, { useEffect, useState, useRef } from "react";
import { Activity, Cpu, Crown, Server, Zap } from "lucide-react";
import { NodeInfo, useClusterStore } from "../store";

export function TopologyGraph({ nodes }: { nodes: NodeInfo[] }) {
  const containerRef = useRef<HTMLDivElement>(null);
  const [dimensions, setDimensions] = useState({ w: 0, h: 0 });
  const { raftState, messages, isSimulationRunning, isPaused } = useClusterStore();
  const [hoveredMessageId, setHoveredMessageId] = useState<string | null>(null);
  
  // STABLE POSITION MAP - keyed by node ID
  const nodePositions = useRef<Map<string, { angle: number; x: number; y: number }>>(new Map());
  const layoutVersion = useRef(0); // Track layout changes

  const centerX = dimensions.w / 2;
  const centerY = dimensions.h / 2;
  const radius = Math.min(centerX, centerY) * 0.65;

  // RECALCULATE layout when node count changes
  useEffect(() => {
    const nodeCount = nodes.length;
    if (nodeCount === 0) return;
    
    // Create new position map with clean circular layout
    const newPositions = new Map<string, { angle: number; x: number; y: number }>();
    const angleStep = (2 * Math.PI) / nodeCount;
    
    nodes.forEach((node, index) => {
      const angle = angleStep * index - Math.PI / 2; // Start from top
      const x = centerX + radius * Math.cos(angle);
      const y = centerY + radius * Math.sin(angle);
      newPositions.set(node.id, { angle, x, y });
    });
    
    nodePositions.current = newPositions;
    layoutVersion.current += 1;
  }, [nodes.length, centerX, centerY, radius]); // Only recalculate when count or dimensions change

  // Update positions when container resizes (keep same layout)
  useEffect(() => {
    if (!containerRef.current) return;
    const updateSize = () => {
      const newCenterX = containerRef.current!.clientWidth / 2;
      const newCenterY = containerRef.current!.clientHeight / 2;
      const newRadius = Math.min(newCenterX, newCenterY) * 0.65;
      
      // Recalculate all positions with new dimensions (keep same angles)
      nodePositions.current.forEach((pos, nodeId) => {
        pos.x = newCenterX + newRadius * Math.cos(pos.angle);
        pos.y = newCenterY + newRadius * Math.sin(pos.angle);
      });
      
      setDimensions({ w: newCenterX * 2, h: newCenterY * 2 });
    };
    updateSize();
    window.addEventListener("resize", updateSize);
    return () => window.removeEventListener("resize", updateSize);
  }, []);

  // Build positioned nodes array with current positions
  const positionedNodes = nodes.map(node => {
    const pos = nodePositions.current.get(node.id);
    if (!pos) return null;
    return {
      ...node,
      x: pos.x,
      y: pos.y,
    };
  }).filter(Boolean) as Array<NodeInfo & { x: number; y: number }>;

  // Control Plane Node (Virtual node at center)
  const cpNode = {
    id: "CP",
    x: centerX,
    y: centerY,
    status: "active" as const
  };

  // Create edges - connect ALL nodes (complete graph)
  const edges: Array<{ source: any; target: any; id: string }> = [];
  for (let i = 0; i < positionedNodes.length; i++) {
    // Connect to other nodes
    for (let j = i + 1; j < positionedNodes.length; j++) {
      edges.push({
        source: positionedNodes[i],
        target: positionedNodes[j],
        id: `${positionedNodes[i].id}-${positionedNodes[j].id}`,
      });
    }
    // Connect to Control Plane
    edges.push({
      source: cpNode,
      target: positionedNodes[i],
      id: `CP-${positionedNodes[i].id}`,
    });
  }

  if (nodes.length === 0) {
    return (
      <div className="w-full h-full flex flex-col items-center justify-center text-muted-foreground relative z-10">
        <div className="w-16 h-16 rounded-full bg-muted/30 flex items-center justify-center mb-4">
          <Activity className="w-8 h-8 opacity-40" />
        </div>
        <span className="text-sm font-medium">Awaiting network topology</span>
      </div>
    );
  }

  return (
    <div ref={containerRef} className="w-full h-full relative z-10">
      <svg className="absolute inset-0 w-full h-full overflow-visible pointer-events-none z-30">
        <defs>
          {/* Hexagon Clip Path for CP (Reserved) */}
          <clipPath id="hexagonClip">
            <polygon points="32,0 60,16 60,48 32,64 4,48 4,16" />
          </clipPath>

          {/* Premium Gradient for Hubs */}
          <linearGradient id="hubGradient" x1="0%" y1="0%" x2="100%" y2="100%">
            <stop offset="0%" stopColor="#4f46e5" />
            <stop offset="100%" stopColor="#1e1b4b" />
          </linearGradient>

          {/* Neon Glow filters */}
          <filter id="neonGlow" x="-50%" y="-50%" width="200%" height="200%">
            <feGaussianBlur in="SourceGraphic" stdDeviation="4" result="blur" />
            <feComposite in="SourceGraphic" in2="blur" operator="over" />
          </filter>

          {/* Arrow markers */}
          <marker id="arrowhead" markerWidth="6" markerHeight="6" refX="5" refY="3" orient="auto">
            <polygon points="0 0, 6 3, 0 6" fill="currentColor" className="text-primary/40" />
          </marker>
        </defs>

        {/* Render Edges - STRAIGHT lines */}
        {edges.map((edge) => {
          const isSourceCrashed = edge.source.status === "crashed";
          const isTargetCrashed = edge.target.status === "crashed";
          const isBroken = isSourceCrashed || isTargetCrashed;

          return (
            <g key={edge.id}>
              {/* Straight connection line */}
              <line
                x1={edge.source.x}
                y1={edge.source.y}
                x2={edge.target.x}
                y2={edge.target.y}
                stroke="currentColor"
                className={`transition-all duration-300 ${
                  isBroken ? "text-border/30" : "text-primary/25"
                }`}
                strokeWidth={2}
                strokeDasharray={isBroken ? "4 4" : "none"}
              />
              
              {/* Animated pulse dots (only on active connections) */}
              {/* Background pulses removed to focus on protocol messages */}
            </g>
          );
        })}

        {/* Render Simulated Messages - ONLY from ALIVE nodes */}
        {isSimulationRunning && messages.map((msg, idx) => {
          const src = msg.sourceId === "CP" ? cpNode : positionedNodes.find(n => n.id === msg.sourceId);
          const tgt = msg.targetId === "CP" ? cpNode : positionedNodes.find(n => n.id === msg.targetId);
          
          // Skip if source or target not found, or if source is crashed
          if (!src || !tgt || (src.id !== "CP" && (src as any).status === "crashed")) return null;

          // Calculate position along straight line
          const t = msg.progress;
          const x = src.x + (tgt.x - src.x) * t;
          const y = src.y + (tgt.y - src.y) * t;

          // Calculate angle for rotation
          const dx = tgt.x - src.x;
          const dy = tgt.y - src.y;
          const angle = Math.atan2(dy, dx) * (180 / Math.PI);

          const isHovered = hoveredMessageId === msg.id;

          // Color based on message type
          let color = "#f43f5e"; // rose for heartbeat
          let glowColor = "#fb7185";
          let orbSize = 6;
          
          if (msg.type === "vote_request") {
            color = "#3b82f6";
            glowColor = "#60a5fa";
            orbSize = 7;
          } else if (msg.type === "vote_reply") {
            if (msg.voteGranted) {
              color = "#10b981";
              glowColor = "#34d399";
              orbSize = 6;
            } else {
              color = "#64748b";
              glowColor = "#94a3b8";
              orbSize = 5;
            }
          } else if (msg.type === "GOSSIP_DIGEST") {
            color = "#eab308"; // yellow-500
            glowColor = "#fde047"; // yellow-400
            orbSize = 5;
          } else if (msg.type === "GOSSIP_STATE") {
            color = "#10b981"; // emerald-500
            glowColor = "#34d399"; // emerald-400
            orbSize = 7;
          } else if (msg.type === "GOSSIP_SYNC_REQ") {
            color = "#3b82f6"; // blue-500
            glowColor = "#60a5fa"; // blue-400
            orbSize = 6;
          } else if (msg.type === "CP_REGISTRATION") {
            color = "#a855f7"; // purple-500
            glowColor = "#c084fc"; // purple-400
            orbSize = 8;
          } else if (msg.type === "CP_KV_PUT") {
            color = "#0ea5e9"; // sky-500
            glowColor = "#38bdf8"; // sky-400
            orbSize = 8;
          } else if (msg.type === "CP_KV_GET") {
            color = "#f97316"; // orange-500
            glowColor = "#fb923c"; // orange-400
            orbSize = 8;
          }

          const isSelected = useClusterStore.getState().selectedMessageId === msg.id;

          return (
            <g 
              key={`${msg.id}-${idx}`} 
              onClick={(e) => {
                e.stopPropagation();
                useClusterStore.getState().setSelectedMessageId(msg.id);
              }}
              onMouseEnter={() => isPaused && setHoveredMessageId(msg.id)}
              onMouseLeave={() => setHoveredMessageId(null)}
              className="pointer-events-auto cursor-pointer"
            >
              {/* NO TAIL: Removed per user request */}
              
              {/* Outer neon glow */}
              <circle
                cx={x}
                cy={y}
                r={orbSize + (isHovered || isSelected ? 18 : 14)}
                fill={color}
                opacity={isHovered || isSelected ? "0.4" : (t > 0.92 ? 0 : "0.2")}
                style={{ filter: `blur(12px)`, transition: "opacity 0.2s" }}
              />
              
              {/* Static selection halo (fixed 'flying orb' bug) */}
              {isSelected && (
                <circle
                  cx={x}
                  cy={y}
                  r={orbSize + 10}
                  fill="none"
                  stroke={color}
                  strokeWidth="2"
                  opacity="0.6"
                />
              )}

              {/* Core neon orb */}
              <circle
                cx={x}
                cy={y}
                r={orbSize + (isHovered || isSelected ? 3 : 1)}
                fill="white"
                opacity={t > 0.92 ? 0 : "0.3"}
                style={{ filter: "blur(2px)", transition: "opacity 0.2s" }}
              />

              <circle
                cx={x}
                cy={y}
                r={orbSize}
                fill={color}
                className="shadow-2xl transition-opacity duration-200"
                opacity={t > 0.92 ? 0 : 1}
                style={{ 
                  filter: "drop-shadow(0 0 10px " + color + ")",
                  stroke: isSelected ? "white" : "none",
                  strokeWidth: 2
                }}
              />

              {/* Hover Label */}
              {isHovered && isPaused && (
                <g>
                  <rect
                    x={x + 15}
                    y={y - 12}
                    width={msg.type.length * 7 + 25}
                    height={28}
                    rx={8}
                    fill="rgba(15, 23, 42, 0.95)"
                    className="backdrop-blur-md shadow-2xl border border-white/20"
                  />
                  <text
                    x={x + 25}
                    y={y + 6}
                    fill="white"
                    fontSize="11"
                    fontWeight="700"
                    className="font-mono tracking-tight"
                  >
                    {msg.type}
                  </text>
                  <circle cx={x + 20} cy={y + 2} r="2" fill={color} />
                </g>
              )}
            </g>
          );
        })}

        {/* Render Raft Timers */}
        {isSimulationRunning && positionedNodes.map((n) => {
          if (n.status === "crashed") return null;
          const rState = raftState[n.id];
          if (!rState) return null;

          const circumference = 2 * Math.PI * 42;
          let progress = rState.timerProgress / rState.timeoutLimit;
          if (rState.role === "leader") progress = 1;
          const offset = circumference - (progress * circumference);

          let color = "#10b981";
          if (rState.role === "candidate") {
            color = "#3b82f6";
          }
          if (rState.role === "leader") {
            color = "#f59e0b";
          }

          return (
            <g key={`timer-${n.id}`}>
              <circle
                cx={n.x}
                cy={n.y}
                r="42"
                fill="none"
                stroke={color}
                strokeWidth="3"
                className="transform -rotate-90"
                style={{ 
                  transformOrigin: `${n.x}px ${n.y}px`,
                  strokeDasharray: circumference,
                  strokeDashoffset: rState.role === "leader" ? 0 : offset,
                  strokeLinecap: "round",
                  filter: `drop-shadow(0 0 8px ${color})`,
                  opacity: 0.7,
                }}
              />
            </g>
          );
        })}
        {/* Control Plane Hub (Center) - Match Standard Node Design Exactly */}
        <foreignObject x={centerX - 40} y={centerY - 40} width={80} height={100} className="pointer-events-auto">
          <div className="w-full h-full flex flex-col items-center justify-center transition-all duration-500">
            {/* Base box - EXACT same style as nodes but slightly larger icon */}
            <div className="w-[66px] h-[66px] rounded-2xl border-2 border-slate-300 dark:border-slate-600 bg-white dark:bg-slate-800 shadow-[0_0_15px_rgba(30,58,138,0.1)] flex flex-col items-center justify-center overflow-hidden">
              <Server className="w-8 h-8 text-primary/80 dark:text-primary mb-0.5" />
            </div>

            {/* Simple text label - same as nodes */}
            <div className="mt-1.5 flex flex-col items-center">
              <span className="text-[10px] font-extrabold uppercase tracking-widest text-slate-900 dark:text-slate-100">
                CONTROL PLANE
              </span>
              <span className="text-[8px] font-bold text-slate-400 dark:text-slate-500 uppercase flex items-center gap-1">
                <div className="w-1 h-1 rounded-full bg-success" /> Hub
              </span>
            </div>
          </div>
        </foreignObject>
      </svg>

      {/* Render Nodes */}
      {positionedNodes.map((n) => {
        const rState = raftState[n.id];
        const isCrashed = n.status === "crashed";

        let nodeBg = "bg-white dark:bg-slate-800";
        let nodeBorder = "border-slate-200 dark:border-slate-700";
        let nodeGlow = "";
        let Icon = Cpu;
        let roleLabel = "";
        let roleColor = "text-slate-600 dark:text-slate-400";

        if (isSimulationRunning && !isCrashed && rState) {
          if (rState.role === "leader") {
            nodeBg = "bg-amber-50 dark:bg-amber-900/20";
            nodeBorder = "border-amber-400";
            nodeGlow = "shadow-[0_0_20px_rgba(245,158,11,0.4)]";
            Icon = Crown;
            roleLabel = `L`;
            roleColor = "text-amber-600 dark:text-amber-400";
          } else if (rState.role === "candidate") {
            nodeBg = "bg-blue-50 dark:bg-blue-900/20";
            nodeBorder = "border-blue-400";
            nodeGlow = "shadow-[0_0_15px_rgba(59,130,246,0.3)]";
            Icon = Zap;
            roleLabel = "C";
            roleColor = "text-blue-600 dark:text-blue-400";
          } else {
            nodeBg = "bg-emerald-50 dark:bg-emerald-900/20";
            nodeBorder = "border-emerald-400";
            nodeGlow = "shadow-[0_0_10px_rgba(16,185,129,0.2)]";
            roleLabel = "F";
            roleColor = "text-emerald-600 dark:text-emerald-400";
          }
        }

        if (isCrashed) {
          nodeBg = "bg-red-50 dark:bg-red-900/20";
          nodeBorder = "border-red-400";
          nodeGlow = "";
          roleLabel = "X";
          roleColor = "text-red-600 dark:text-red-400";
        }

        return (
          <div
            key={n.id}
            className="absolute transform -translate-x-1/2 -translate-y-1/2 z-20 flex flex-col items-center transition-all duration-500"
            style={{ left: n.x, top: n.y }}
          >
            <div className={`w-16 h-16 rounded-2xl border-2 flex flex-col items-center justify-center transition-all duration-300 ${nodeBg} ${nodeBorder} ${nodeGlow}`}>
              <Icon className={`w-6 h-6 mb-1 ${roleColor}`} />
              <span className="text-[10px] font-bold uppercase tracking-wider text-slate-700 dark:text-slate-300">
                {n.id}
              </span>
            </div>
            
            {isSimulationRunning && roleLabel && (
              <div className={`absolute -top-2 -right-2 w-6 h-6 rounded-full flex items-center justify-center text-[10px] font-bold border-2 shadow-lg ${
                isCrashed 
                  ? "bg-red-500 border-red-300 text-white" 
                  : rState?.role === "leader"
                  ? "bg-amber-500 border-amber-300 text-white"
                  : rState?.role === "candidate"
                  ? "bg-blue-500 border-blue-300 text-white"
                  : "bg-emerald-500 border-emerald-300 text-white"
              }`}>
                {roleLabel}
              </div>
            )}
            
            {!isCrashed && (
              <div className="absolute -top-1.5 flex gap-1">
                <div className="w-1.5 h-1.5 rounded-full bg-primary/60" />
                <div className="w-1.5 h-1.5 rounded-full bg-primary/40" />
                <div className="w-1.5 h-1.5 rounded-full bg-primary/60" />
              </div>
            )}
          </div>
        );
      })}
      
      {/* Message Inspector Panel */}
      {(() => {
        const selectedMessage = messages.find(m => m.id === useClusterStore.getState().selectedMessageId);
        if (!selectedMessage) return null;
        
        return (
          <div className="absolute top-4 right-4 w-72 bg-white/95 dark:bg-slate-900/95 backdrop-blur-xl border border-slate-200 dark:border-slate-800 rounded-2xl shadow-2xl overflow-hidden z-50 animate-in fade-in slide-in-from-top-4 duration-300">
            <div className="px-4 py-3 border-b border-slate-100 dark:border-slate-800 flex items-center justify-between bg-slate-50 dark:bg-slate-800/50">
              <div className="flex items-center gap-2">
                <div 
                  className="w-2 h-2 rounded-full" 
                  style={{ backgroundColor: 
                    selectedMessage.type.includes("GOSSIP") ? "#eab308" : 
                    selectedMessage.type.includes("CP") ? "#3b82f6" : "#f43f5e" 
                  }} 
                />
                <span className="text-[10px] font-bold uppercase tracking-wider text-slate-500">Message Detail</span>
              </div>
              <button 
                onClick={() => useClusterStore.getState().setSelectedMessageId(null)}
                className="p-1 hover:bg-slate-200 dark:hover:bg-slate-700 rounded-full transition-colors"
              >
                <div className="w-4 h-4 text-slate-400">✕</div>
              </button>
            </div>
            
            <div className="p-4 space-y-5">
              {/* Telemetry Row */}
              <div className="grid grid-cols-2 gap-3">
                <div className="p-2.5 bg-slate-50 dark:bg-slate-800/50 rounded-xl border border-slate-100 dark:border-slate-800 flex items-center gap-3">
                  <div className="w-8 h-8 rounded-lg bg-indigo-500/10 flex items-center justify-center text-indigo-600 dark:text-indigo-400">
                    <Activity className="w-4 h-4" />
                  </div>
                  <div>
                    <div className="text-[8px] font-bold text-slate-400 uppercase tracking-tighter">Timing</div>
                    <div className="text-[11px] font-mono font-bold text-slate-700 dark:text-slate-200">
                      T + {selectedMessage.timestampMs ? Math.max(0, Math.floor(Date.now() - selectedMessage.timestampMs)) : "0"}ms
                    </div>
                  </div>
                </div>
                
                <div className="p-2.5 bg-slate-50 dark:bg-slate-800/50 rounded-xl border border-slate-100 dark:border-slate-800 flex items-center gap-3">
                  <div className="w-8 h-8 rounded-lg bg-emerald-500/10 flex items-center justify-center text-emerald-600 dark:text-emerald-400">
                    <div className="text-[10px] font-bold underline decoration-2">KB</div>
                  </div>
                  <div>
                    <div className="text-[8px] font-bold text-slate-400 uppercase tracking-tighter">Transit</div>
                    <div className="text-[11px] font-mono font-bold text-slate-700 dark:text-slate-200">
                      {selectedMessage.sizeBytes ? (
                        selectedMessage.sizeBytes > 1024 
                          ? (selectedMessage.sizeBytes / 1024).toFixed(1) + " KB" 
                          : selectedMessage.sizeBytes + " B"
                      ) : "64 B"}
                    </div>
                  </div>
                </div>
              </div>

              <div>
                <div className="text-[9px] font-bold text-slate-400 uppercase mb-2 tracking-widest">Protocol Type</div>
                <div className="text-sm font-black text-primary font-mono bg-primary/5 px-3 py-2 rounded-lg border border-primary/10 inline-block shadow-sm">
                  {selectedMessage.type}
                </div>
              </div>
              
              <div className="flex items-center gap-4 py-2 border-y border-slate-100 dark:border-slate-800">
                <div className="flex-1">
                  <div className="text-[8px] font-bold text-slate-400 uppercase mb-1">Source</div>
                  <div className="text-xs font-black text-slate-700 dark:text-slate-200">{selectedMessage.sourceId}</div>
                </div>
                <div className="w-8 h-8 rounded-full bg-slate-100 dark:bg-slate-800 flex items-center justify-center">
                  <div className="w-4 h-0.5 bg-slate-300 dark:bg-slate-600 rounded-full" />
                </div>
                <div className="flex-1 text-right">
                  <div className="text-[8px] font-bold text-slate-400 uppercase mb-1">Recipient</div>
                  <div className="text-xs font-black text-slate-700 dark:text-slate-200">{selectedMessage.targetId}</div>
                </div>
              </div>

              {selectedMessage.metadata ? (
                <div>
                  <div className="text-[9px] font-bold text-slate-400 uppercase mb-2 tracking-widest flex justify-between">
                    <span>Payload Information</span>
                    <span className="text-emerald-500 font-mono opacity-50">TRACE://DATA</span>
                  </div>
                  <div className="text-[11px] font-mono p-4 bg-slate-900 text-emerald-400 rounded-xl overflow-x-auto border border-white/5 shadow-2xl relative leading-relaxed">
                    <div className="absolute top-2 right-2 opacity-10 text-[7px] uppercase font-bold text-emerald-500 select-none">
                      Secured Intercept
                    </div>
                    <span className="text-emerald-500/40 mr-2 select-none">❯</span>
                    <span className="break-all whitespace-pre-wrap">
                      {selectedMessage.metadata.replace(/_/g, ' ')}
                    </span>
                  </div>
                </div>
              ) : (
                <div className="text-[10px] text-slate-500 italic p-3 bg-slate-50 dark:bg-slate-800/50 rounded-xl border border-dashed border-slate-200 dark:border-slate-700 text-center">
                  Base heartbeat – No extra data payload.
                </div>
              )}
            </div>
            
            <div className="px-4 py-2 bg-slate-50/50 dark:bg-slate-800/30 border-t border-slate-100 dark:border-slate-800 flex items-center gap-2 text-[10px] text-slate-500 italic">
              <Activity className="w-3 h-3 text-primary/60" />
              <span>Real-time simulation interceptor active</span>
            </div>
          </div>
        );
      })()}

      {/* Legend and Stats */}
      <div className="absolute bottom-4 right-4 bg-white/95 dark:bg-slate-800/95 backdrop-blur-sm border border-slate-200 dark:border-slate-700 rounded-xl p-4 shadow-xl pointer-events-auto">
        <div className="text-[10px] font-bold text-slate-500 uppercase tracking-wider mb-3">Protocol Messages</div>
        <div className="grid grid-cols-2 gap-x-6 gap-y-2">
          <div className="flex items-center gap-2">
            <div className="w-2.5 h-2.5 rounded-full bg-yellow-500" style={{ filter: "drop-shadow(0 0 6px #eab308)" }} />
            <span className="text-[11px] text-slate-600 dark:text-slate-400">Gossip Digest</span>
          </div>
          <div className="flex items-center gap-2">
            <div className="w-2.5 h-2.5 rounded-full bg-emerald-500" style={{ filter: "drop-shadow(0 0 6px #10b981)" }} />
            <span className="text-[11px] text-slate-600 dark:text-slate-400">Gossip State</span>
          </div>
          <div className="flex items-center gap-2">
            <div className="w-2.5 h-2.5 rounded-full bg-blue-500" style={{ filter: "drop-shadow(0 0 6px #3b82f6)" }} />
            <span className="text-[11px] text-slate-600 dark:text-slate-400">Sync Request</span>
          </div>
          <div className="flex items-center gap-2">
            <div className="w-2.5 h-2.5 rounded-full bg-purple-500" style={{ filter: "drop-shadow(0 0 6px #a855f7)" }} />
            <span className="text-[11px] text-slate-600 dark:text-slate-400">Node Reg</span>
          </div>
          <div className="flex items-center gap-2">
            <div className="w-2.5 h-2.5 rounded-full bg-sky-500" style={{ filter: "drop-shadow(0 0 6px #0ea5e9)" }} />
            <span className="text-[11px] text-slate-600 dark:text-slate-400">KV Put (CP)</span>
          </div>
          <div className="flex items-center gap-2">
            <div className="w-2.5 h-2.5 rounded-full bg-orange-500" style={{ filter: "drop-shadow(0 0 6px #f97316)" }} />
            <span className="text-[11px] text-slate-600 dark:text-slate-400">KV Get (CP)</span>
          </div>
        </div>
        <div className="mt-3 pt-3 border-t border-slate-200 dark:border-slate-700 flex items-center justify-between">
          <div className="text-[10px] text-slate-500 font-medium">
            In-flight: <span className="font-mono font-bold text-primary">{messages.length}</span>
          </div>
          <div className={`text-[10px] px-2 py-0.5 rounded-full font-bold uppercase tracking-tighter ${isPaused ? "bg-warning/20 text-warning" : "bg-success/20 text-success"}`}>
            {isPaused ? "Paused" : "Live"}
          </div>
        </div>
      </div>
      
    </div>
  );
}
