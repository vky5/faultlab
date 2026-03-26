import React, { useEffect, useState, useRef } from "react";
import { Activity, Cpu, Crown, Zap } from "lucide-react";
import { NodeInfo, useClusterStore } from "../store";

export function TopologyGraph({ nodes }: { nodes: NodeInfo[] }) {
  const containerRef = useRef<HTMLDivElement>(null);
  const [dimensions, setDimensions] = useState({ w: 0, h: 0 });
  const { raftState, messages, isSimulationRunning } = useClusterStore();
  
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

  // Create edges - connect ALL nodes (complete graph)
  const edges: Array<{ source: any; target: any; id: string }> = [];
  for (let i = 0; i < positionedNodes.length; i++) {
    for (let j = i + 1; j < positionedNodes.length; j++) {
      edges.push({
        source: positionedNodes[i],
        target: positionedNodes[j],
        id: `${positionedNodes[i].id}-${positionedNodes[j].id}`,
      });
    }
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
      <svg className="absolute inset-0 w-full h-full overflow-visible pointer-events-none">
        <defs>
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
              {!isBroken && (
                <>
                  {/* Pulse traveling from source to target */}
                  <circle r="3" fill="var(--primary)" opacity="0.5">
                    <animateMotion
                      dur="2s"
                      repeatCount="indefinite"
                      path={`M ${edge.source.x} ${edge.source.y} L ${edge.target.x} ${edge.target.y}`}
                    />
                  </circle>
                  {/* Pulse traveling from target to source (offset timing) */}
                  <circle r="3" fill="var(--primary)" opacity="0.5">
                    <animateMotion
                      dur="2s"
                      repeatCount="indefinite"
                      path={`M ${edge.target.x} ${edge.target.y} L ${edge.source.x} ${edge.source.y}`}
                      begin="1s"
                    />
                  </circle>
                </>
              )}
            </g>
          );
        })}

        {/* Render Simulated Messages - ONLY from ALIVE nodes */}
        {isSimulationRunning && messages.map((msg, idx) => {
          const src = positionedNodes.find(n => n.id === msg.sourceId);
          const tgt = positionedNodes.find(n => n.id === msg.targetId);
          
          // Skip if source or target not found, or if source is crashed
          if (!src || !tgt || src.status === "crashed") return null;

          // Calculate position along straight line
          const t = msg.progress;
          const x = src.x + (tgt.x - src.x) * t;
          const y = src.y + (tgt.y - src.y) * t;

          // Calculate angle for rotation
          const dx = tgt.x - src.x;
          const dy = tgt.y - src.y;
          const angle = Math.atan2(dy, dx) * (180 / Math.PI);

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
          }

          return (
            <g key={`${msg.id}-${idx}`} style={{ transform: `rotate(${angle}deg)`, transformOrigin: `${x}px ${y}px` }}>
              {/* Trailing glow tail */}
              {t > 0.1 && t < 0.95 && (
                <ellipse
                  cx={x - Math.cos(angle * Math.PI / 180) * 10}
                  cy={y - Math.sin(angle * Math.PI / 180) * 10}
                  rx={orbSize * 1.5}
                  ry={orbSize * 0.8}
                  fill={glowColor}
                  opacity={0.3 * (1 - t)}
                  style={{ filter: `blur(4px)` }}
                />
              )}
              
              {/* Outer glow ring */}
              <circle
                cx={x}
                cy={y}
                r={orbSize + 12}
                fill={glowColor}
                opacity="0.15"
                style={{ filter: `blur(8px)` }}
              />
              
              {/* Middle glow ring */}
              <circle
                cx={x}
                cy={y}
                r={orbSize + 7}
                fill={glowColor}
                opacity="0.3"
                style={{ filter: `blur(5px)` }}
              />
              
              {/* Inner glow */}
              <circle
                cx={x}
                cy={y}
                r={orbSize + 3}
                fill={glowColor}
                opacity="0.5"
                style={{ filter: `blur(3px)` }}
              />
              
              {/* Core colored orb */}
              <circle
                cx={x}
                cy={y}
                r={orbSize}
                fill={color}
                style={{
                  filter: `drop-shadow(0 0 8px ${glowColor}) drop-shadow(0 0 16px ${glowColor})`,
                }}
              />
              
              {/* Bright white core */}
              <circle
                cx={x}
                cy={y}
                r={orbSize * 0.45}
                fill="white"
                opacity="0.95"
              />
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
      
      {/* Legend and Stats */}
      {isSimulationRunning && (
        <div className="absolute bottom-4 right-4 bg-white/95 dark:bg-slate-800/95 backdrop-blur-sm border border-slate-200 dark:border-slate-700 rounded-xl p-4 shadow-xl">
          <div className="text-[10px] font-bold text-slate-500 uppercase tracking-wider mb-3">Messages</div>
          <div className="space-y-2">
            <div className="flex items-center gap-2">
              <div className="w-2.5 h-2.5 rounded-full bg-rose-500" style={{ filter: "drop-shadow(0 0 6px #f43f5e)" }} />
              <span className="text-xs text-slate-600 dark:text-slate-400">Heartbeat</span>
            </div>
            <div className="flex items-center gap-2">
              <div className="w-2.5 h-2.5 rounded-full bg-blue-500" style={{ filter: "drop-shadow(0 0 6px #3b82f6)" }} />
              <span className="text-xs text-slate-600 dark:text-slate-400">Vote Request</span>
            </div>
            <div className="flex items-center gap-2">
              <div className="w-2.5 h-2.5 rounded-full bg-emerald-500" style={{ filter: "drop-shadow(0 0 6px #10b981)" }} />
              <span className="text-xs text-slate-600 dark:text-slate-400">Vote Granted</span>
            </div>
          </div>
          <div className="mt-3 pt-3 border-t border-slate-200 dark:border-slate-700">
            <div className="text-xs text-slate-500">
              Active: <span className="font-mono font-bold text-primary">{messages.length}</span>
            </div>
          </div>
        </div>
      )}
      
      {!isSimulationRunning && nodes.length > 0 && (
        <div className="absolute bottom-4 right-4 bg-white/95 dark:bg-slate-800/95 backdrop-blur-sm border border-slate-200 dark:border-slate-700 rounded-xl p-4 shadow-xl max-w-xs">
          <div className="flex items-start gap-3">
            <div className="w-8 h-8 rounded-lg bg-primary/10 flex items-center justify-center flex-shrink-0">
              <Activity className="w-4 h-4 text-primary" />
            </div>
            <div>
              <div className="text-sm font-semibold text-slate-700 dark:text-slate-300">Enable Simulation</div>
              <p className="text-xs text-slate-500 mt-1">
                Toggle "Run Raft Simulation" in the sidebar to see live message flow between nodes.
              </p>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
