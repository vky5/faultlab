import React, { useEffect, useState, useRef, useMemo } from "react";
import { Activity, Cpu, Crown, Server, Zap } from "lucide-react";
import { motion, AnimatePresence, useAnimation } from "framer-motion";
import * as d3Force from "d3-force";
import * as d3Zoom from "d3-zoom";
import * as d3Select from "d3-selection";
import { NodeInfo, useClusterStore } from "../store";

const d3 = { ...d3Force, ...d3Zoom, ...d3Select };

interface SimNode extends d3.SimulationNodeDatum {
  id: string;
  node: NodeInfo;
}

interface SimEdge extends d3.SimulationLinkDatum<SimNode> {
  id: string;
}

export function TopologyGraph({ nodes }: { nodes: NodeInfo[] }) {
  const containerRef = useRef<HTMLDivElement>(null);
  const [dimensions, setDimensions] = useState({ w: 0, h: 0 });
  const { raftState, messages, isSimulationRunning, isPaused, showControlPlane, toggleControlPlane } = useClusterStore();
  const [hoveredMessageId, setHoveredMessageId] = useState<string | null>(null);
  const [simNodes, setSimNodes] = useState<SimNode[]>([]);
  const svgRef = useRef<SVGSVGElement>(null);
  const [transform, setTransform] = useState({ x: 0, y: 0, k: 1 });

  const centerX = dimensions.w / 2;
  const centerY = dimensions.h / 2;

  const simulation = useRef<d3.Simulation<SimNode, undefined> | null>(null);
  const zoomBehavior = useRef<d3.ZoomBehavior<SVGSVGElement, unknown> | null>(null);

  // Initialize Zoom behavior
  useEffect(() => {
    if (!svgRef.current) return;
    zoomBehavior.current = d3.zoom<SVGSVGElement, unknown>()
      .scaleExtent([0.1, 5])
      .on("zoom", (event) => {
        setTransform(event.transform);
      });
    
    d3.select(svgRef.current).call(zoomBehavior.current);
  }, []);

  const handleZoomIn = () => {
    if (svgRef.current && zoomBehavior.current) {
      d3.select(svgRef.current).transition().duration(300).call(zoomBehavior.current.scaleBy, 1.4);
    }
  };

  const handleZoomOut = () => {
    if (svgRef.current && zoomBehavior.current) {
      d3.select(svgRef.current).transition().duration(300).call(zoomBehavior.current.scaleBy, 0.7);
    }
  };

  const handleResetZoom = () => {
    if (svgRef.current && zoomBehavior.current) {
      d3.select(svgRef.current).transition().duration(500).call(zoomBehavior.current.transform, d3.zoomIdentity);
    }
  };

  // Initialize simulation
  useEffect(() => {
    simulation.current = d3.forceSimulation<SimNode>()
      .force("charge", d3.forceManyBody().strength(-400))
      .force("center", d3.forceCenter(centerX, centerY))
      .force("x", d3.forceX(centerX).strength(0.08))
      .force("y", d3.forceY(centerY).strength(0.08))
      .force("collision", d3.forceCollide<SimNode>().radius(100))
      .alphaDecay(0.0228) // Slightly slower decay for smoother settling
      .on("tick", () => {
        // Use functional update to avoid capturing stale state, but keep it stable
        setSimNodes(prev => [...prev]); 
      });

    return () => {
      simulation.current?.stop();
    };
  }, []);

  // Sync prop nodes with simulation nodes
  useEffect(() => {
    if (!simulation.current) return;

    setSimNodes(prev => {
      let changed = false;
      const newSimNodes = nodes.map(node => {
        const existing = prev.find(p => p.id === node.id);
        if (existing) {
          // Update the underlying node info but preserve the simulation object (x, y, vx, vy)
          if (existing.node !== node) {
            existing.node = node;
            changed = true;
          }
          return existing;
        }
        changed = true;
        return {
          id: node.id,
          node: node,
          x: centerX + (Math.random() - 0.5) * 50,
          y: centerY + (Math.random() - 0.5) * 50,
        };
      });

      // Also check if any nodes were removed
      if (prev.length !== nodes.length) changed = true;
      
      if (changed) {
        simulation.current?.nodes(newSimNodes);
        simulation.current?.alpha(0.3).restart();
        return [...newSimNodes];
      }
      return prev;
    });
  }, [nodes, centerX, centerY]);

  // Update center forces when dimensions change
  useEffect(() => {
    if (!simulation.current) return;
    simulation.current.force("center", d3.forceCenter(centerX, centerY));
    simulation.current.force("x", d3.forceX(centerX).strength(0.08));
    simulation.current.force("y", d3.forceY(centerY).strength(0.08));
    simulation.current.alpha(0.1).restart(); // Lower alpha for dimension changes
  }, [centerX, centerY]);

  // Resize handler
  useEffect(() => {
    if (!containerRef.current) return;
    const updateSize = () => {
      setDimensions({
        w: containerRef.current!.clientWidth,
        h: containerRef.current!.clientHeight
      });
    };
    updateSize();
    window.addEventListener("resize", updateSize);
    return () => window.removeEventListener("resize", updateSize);
  }, []);

  // Build positioned nodes for rendering
  const positionedNodes = simNodes.map(sn => ({
    ...sn.node,
    x: sn.x || 0,
    y: sn.y || 0,
  }));

  // Control Plane Node (Virtual node at center)
  const cpNode = {
    id: "CP",
    x: centerX,
    y: centerY,
    status: "active" as const
  };

  // Create edges - connect nodes while respecting network partitions
  const edges: Array<{ source: any; target: any; id: string }> = [];
  for (let i = 0; i < positionedNodes.length; i++) {
    const source = positionedNodes[i];

    // Connect to other nodes
    for (let j = i + 1; j < positionedNodes.length; j++) {
      const target = positionedNodes[j];
      
      // Only add edge if there is NO partition between source and target
      const isPartitioned = source.fault?.partition?.includes(target.id) || 
                          target.fault?.partition?.includes(source.id);
      
      if (!isPartitioned) {
        edges.push({
          source: source,
          target: target,
          id: `${source.id}-${target.id}`,
        });
      }
    }

    // Connect to Control Plane (Management links only if visible)
    if (showControlPlane) {
      edges.push({
        source: cpNode,
        target: source,
        id: `CP-${source.id}`,
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

  // Handle node dragging
  const handleDragStart = (id: string, x: number, y: number) => {
    const node = simNodes.find(n => n.id === id);
    if (node) {
      node.fx = x;
      node.fy = y;
      simulation.current?.alphaTarget(0.3).restart();
    }
  };

  const handleDrag = (id: string, dx: number, dy: number) => {
    const node = simNodes.find(n => n.id === id);
    if (node) {
      // Divide by transform.k to maintain cursor-to-node alignment when zoomed
      node.fx = (node.fx || 0) + dx / transform.k;
      node.fy = (node.fy || 0) + dy / transform.k;
    }
  };

  const handleDragEnd = (id: string) => {
    const node = simNodes.find(n => n.id === id);
    if (node) {
      node.fx = null;
      node.fy = null;
      simulation.current?.alphaTarget(0);
    }
  };

  return (
    <div ref={containerRef} className="w-full h-full relative z-10 overflow-hidden select-none">
      <svg 
        ref={svgRef}
        className="absolute inset-0 w-full h-full overflow-visible z-30 cursor-move"
      >
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

          {/* Premium Edge Gradient */}
          <linearGradient id="edgeGradient" x1="0%" y1="0%" x2="100%" y2="100%">
            <stop offset="0%" stopColor="var(--color-primary)" stopOpacity="0.2" />
            <stop offset="50%" stopColor="var(--color-primary)" stopOpacity="1" />
            <stop offset="100%" stopColor="var(--color-primary)" stopOpacity="0.2" />
          </linearGradient>
        </defs>

        {/* Unified Transform Layer */}
        <g transform={`translate(${transform.x},${transform.y}) scale(${transform.k})`}>
          {/* Render Edges */}
          {edges.map((edge) => {
          const isSourceCrashed = edge.source.status === "crashed";
          const isTargetCrashed = edge.target.status === "crashed";
          const isBroken = isSourceCrashed || isTargetCrashed;

          return (
            <g key={edge.id}>
              {/* Energy strand base */}
              <line
                x1={edge.source.x}
                y1={edge.source.y}
                x2={edge.target.x}
                y2={edge.target.y}
                stroke="currentColor"
                className={`transition-all duration-300 ${isBroken ? "text-border/20" : "text-primary/10"}`}
                strokeWidth={isBroken ? 1 : 2}
                strokeDasharray={isBroken ? "4 4" : "none"}
              />
              
              {!isBroken && (
                <line
                  x1={edge.source.x}
                  y1={edge.source.y}
                  x2={edge.target.x}
                  y2={edge.target.y}
                  stroke="url(#edgeGradient)"
                  strokeWidth={2}
                  opacity={0.3}
                  style={{ filter: "blur(2px)" }}
                />
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
          if (rState.role === "candidate") color = "#3b82f6";
          if (rState.role === "leader") color = "#f59e0b";

          return (
            <g key={`timer-${n.id}`}>
              <circle
                cx={n.x}
                cy={n.y}
                r="42"
                fill="none"
                stroke={color}
                strokeWidth="2"
                className="transform -rotate-90 origin-center"
                style={{ 
                  transformOrigin: `${n.x}px ${n.y}px`,
                  strokeDasharray: circumference,
                  strokeDashoffset: rState.role === "leader" ? 0 : offset,
                  strokeLinecap: "round",
                  filter: `drop-shadow(0 0 5px ${color})`,
                  opacity: 0.4,
                }}
              />
            </g>
          );
        })}

        {/* Render Simulated Messages */}
        {isSimulationRunning && messages.map((msg, idx) => {
          const src = msg.sourceId === "CP" ? cpNode : positionedNodes.find(n => n.id === msg.sourceId);
          const tgt = msg.targetId === "CP" ? cpNode : positionedNodes.find(n => n.id === msg.targetId);
          if (!src || !tgt || (src.id !== "CP" && (src as any).status === "crashed")) return null;

          // Hide messages involving CP if CP is hidden
          if (!showControlPlane && (msg.sourceId === "CP" || msg.targetId === "CP")) return null;

          const t = msg.progress;
          const x = src.x + (tgt.x - src.x) * t;
          const y = src.y + (tgt.y - src.y) * t;
          const isHovered = hoveredMessageId === msg.id;

          let color = "#f43f5e";
          let orbSize = 6;
          if (msg.type === "vote_request") { color = "#3b82f6"; orbSize = 7; }
          else if (msg.type === "GOSSIP_DIGEST") { color = "#eab308"; orbSize = 5; }
          else if (msg.type === "GOSSIP_STATE") { color = "#10b981"; orbSize = 7; }
          else if (msg.type.includes("CP")) { color = "#a855f7"; orbSize = 8; }

          const isSelected = useClusterStore.getState().selectedMessageId === msg.id;

          return (
            <g 
              key={`${msg.id}-${idx}`} 
              onClick={(e) => { e.stopPropagation(); useClusterStore.getState().setSelectedMessageId(msg.id); }}
              onMouseEnter={() => isPaused && setHoveredMessageId(msg.id)}
              onMouseLeave={() => setHoveredMessageId(null)}
              className="pointer-events-auto cursor-pointer"
            >
              <circle cx={x} cy={y} r={orbSize + (isHovered || isSelected ? 18 : 14)} fill={color} opacity={isHovered || isSelected ? "0.4" : "0.2"} style={{ filter: `blur(12px)` }} />
              <circle cx={x} cy={y} r={orbSize + (isHovered || isSelected ? 3 : 1)} fill="white" opacity="0.3" style={{ filter: "blur(2px)" }} />
              <circle cx={x} cy={y} r={orbSize} fill={color} style={{ filter: `drop-shadow(0 0 10px ${color})`, stroke: isSelected ? "white" : "none", strokeWidth: 2 }} />
            </g>
          );
        })}

        {/* Render Nodes (Nested in SVG for perfect coordinate sync) */}
        {positionedNodes.map((n) => {
          const rState = raftState[n.id];
          const isCrashed = n.status === "crashed";
          let nodeBg = "bg-white/80 dark:bg-slate-800/80 backdrop-blur-md";
          let nodeBorder = "border-slate-200 dark:border-slate-700";
          let Icon = Cpu;
          let roleColor = "text-slate-600 dark:text-slate-400";

          if (isSimulationRunning && !isCrashed && rState) {
            if (rState.role === "leader") {
              nodeBg = "bg-amber-50/90 dark:bg-amber-900/40 backdrop-blur-lg";
              nodeBorder = "border-amber-400";
              Icon = Crown;
              roleColor = "text-amber-600 dark:text-amber-400";
            } else if (rState.role === "candidate") {
              nodeBg = "bg-blue-50/90 dark:bg-blue-900/40 backdrop-blur-lg";
              nodeBorder = "border-blue-400";
              Icon = Zap;
              roleColor = "text-blue-600 dark:text-blue-400";
            } else {
              nodeBg = "bg-emerald-50/90 dark:bg-emerald-900/40 backdrop-blur-lg";
              nodeBorder = "border-emerald-400";
              roleColor = "text-emerald-600 dark:text-emerald-400";
            }
          }

          if (isCrashed) {
            nodeBg = "bg-red-50/90 dark:bg-red-900/40 backdrop-blur-md";
            nodeBorder = "border-red-400";
            roleColor = "text-red-600 dark:text-red-400";
          }

          return (
            <foreignObject
              key={n.id}
              x={n.x - 32}
              y={n.y - 32}
              width={64}
              height={80}
              className="overflow-visible"
            >
              <div
                className="w-16 h-16 flex flex-col items-center justify-center pointer-events-auto cursor-grab active:cursor-grabbing"
                onPointerDown={(e) => {
                  (e.target as HTMLElement).setPointerCapture(e.pointerId);
                  handleDragStart(n.id, n.x, n.y);
                }}
                onPointerMove={(e) => {
                  if ((e.target as HTMLElement).hasPointerCapture(e.pointerId)) {
                    handleDrag(n.id, e.movementX, e.movementY);
                  }
                }}
                onPointerUp={(e) => {
                  (e.target as HTMLElement).releasePointerCapture(e.pointerId);
                  handleDragEnd(n.id);
                }}
              >
                <div className={`w-full h-full rounded-2xl border-2 flex flex-col items-center justify-center transition-all duration-300 ${nodeBg} ${nodeBorder} shadow-lg`}>
                  <Icon className={`w-6 h-6 mb-1 ${roleColor}`} />
                  <span className="text-[10px] font-bold uppercase tracking-wider text-slate-700 dark:text-slate-300">
                    {n.id}
                  </span>
                </div>
                
                {isSimulationRunning && !isCrashed && rState && (
                  <div className={`absolute -top-2 -right-2 w-6 h-6 rounded-full flex items-center justify-center text-[10px] font-bold border-2 shadow-lg ${
                    rState.role === "leader" ? "bg-amber-500 border-amber-300 text-white" :
                    rState.role === "candidate" ? "bg-blue-500 border-blue-300 text-white" :
                    "bg-emerald-500 border-emerald-300 text-white"
                  }`}>
                    {rState.role === "leader" ? "L" : rState.role === "candidate" ? "C" : "F"}
                  </div>
                )}
              </div>
            </foreignObject>
          );
        })}

        {/* Control Plane Hub */}
        {showControlPlane && (
          <foreignObject x={centerX - 40} y={centerY - 32} width={80} height={100} className="pointer-events-none">
            <div className="w-full h-full flex flex-col items-center justify-center">
              <div className="w-[64px] h-[64px] rounded-2xl border-2 border-primary/40 bg-white/20 dark:bg-slate-900/40 backdrop-blur-xl shadow-xl flex items-center justify-center overflow-hidden">
                <Server className="w-8 h-8 text-primary" />
              </div>
              <span className="mt-1 text-[10px] font-extrabold uppercase tracking-widest text-slate-800 dark:text-slate-200">HUB</span>
            </div>
          </foreignObject>
        )}
        </g>
      </svg>

      {/* Overlays (Legend & Inspector) */}
      <AnimatePresence>
        {(() => {
          const selectedMessage = messages.find(m => m.id === useClusterStore.getState().selectedMessageId);
          if (!selectedMessage) return null;
          
          return (
            <motion.div 
              initial={{ opacity: 0, scale: 0.95, y: 10 }}
              animate={{ opacity: 1, scale: 1, y: 0 }}
              exit={{ opacity: 0, scale: 0.95, y: 10 }}
              className="absolute top-4 right-4 w-72 bg-white/95 dark:bg-slate-900/95 backdrop-blur-xl border border-slate-200 dark:border-slate-800 rounded-2xl shadow-2xl overflow-hidden z-50 pointer-events-auto animate-in fade-in slide-in-from-top-4 duration-300"
            >
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
            </motion.div>
          );
        })()}
      </AnimatePresence>

      {/* Zoom Controls */}
      <div className="absolute bottom-4 left-4 flex flex-col gap-2 z-50">
        <div className="flex flex-col bg-white/95 dark:bg-slate-800/95 backdrop-blur-sm border border-slate-200 dark:border-slate-700 rounded-xl overflow-hidden shadow-xl">
          <button 
            onClick={handleZoomIn}
            className="p-2.5 hover:bg-slate-100 dark:hover:bg-slate-700 transition-colors border-b border-slate-200 dark:border-slate-700"
            title="Zoom In"
          >
            <div className="w-5 h-5 flex items-center justify-center font-bold text-slate-600 dark:text-slate-300">＋</div>
          </button>
          <button 
            onClick={handleZoomOut}
            className="p-2.5 hover:bg-slate-100 dark:hover:bg-slate-700 transition-colors border-b border-slate-200 dark:border-slate-700"
            title="Zoom Out"
          >
            <div className="w-5 h-5 flex items-center justify-center font-bold text-slate-600 dark:text-slate-300">－</div>
          </button>
          <button 
            onClick={handleResetZoom}
            className="p-2.5 hover:bg-slate-100 dark:hover:bg-slate-700 transition-colors border-b border-slate-200 dark:border-slate-700"
            title="Reset View"
          >
            <Activity className="w-4 h-4 text-slate-600 dark:text-slate-300" />
          </button>
          <button 
            onClick={toggleControlPlane}
            className={`p-2.5 hover:bg-slate-100 dark:hover:bg-slate-700 transition-colors ${showControlPlane ? "text-primary" : "text-slate-400"}`}
            title={showControlPlane ? "Hide Hub" : "Show Hub"}
          >
            <Crown className="w-4 h-4" />
          </button>
        </div>
      </div>

      {/* Legend and Stats */}
      <div className="absolute bottom-4 right-4 bg-white/95 dark:bg-slate-800/95 backdrop-blur-sm border border-slate-200 dark:border-slate-700 rounded-xl p-4 shadow-xl pointer-events-auto z-50">
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
