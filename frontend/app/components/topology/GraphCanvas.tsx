import React from "react";
import { Activity, Cpu, Crown, Server, Zap } from "lucide-react";
import { NodeInfo } from "../../store";

export type PositionedNode = NodeInfo & { x: number; y: number };

interface EdgeDescriptor {
  source: any;
  target: any;
  id: string;
}

interface GraphCanvasProps {
  svgRef: React.RefObject<SVGSVGElement | null>;
  transform: { x: number; y: number; k: number };
  centerX: number;
  centerY: number;
  cpNode: { id: string; x: number; y: number; status: string };
  positionedNodes: PositionedNode[];
  edges: EdgeDescriptor[];
  messages: any[];
  hoveredMessageId: string | null;
  setHoveredMessageId: (id: string | null) => void;
  selectedMessageId: string | null;
  setSelectedMessageId: (id: string | null) => void;
  raftState: Record<string, any>;
  isSimulationRunning: boolean;
  isPaused: boolean;
  showControlPlane: boolean;
  handleDragStart: (id: string, clientX: number, clientY: number) => void;
  handleDrag: (clientX: number, clientY: number) => void;
  handleDragEnd: () => void;
}

export function GraphCanvas({
  svgRef,
  transform,
  centerX,
  centerY,
  cpNode,
  positionedNodes,
  edges,
  messages,
  hoveredMessageId,
  setHoveredMessageId,
  selectedMessageId,
  setSelectedMessageId,
  raftState,
  isSimulationRunning,
  isPaused,
  showControlPlane,
  handleDragStart,
  handleDrag,
  handleDragEnd,
}: GraphCanvasProps) {
  return (
    <svg
      ref={svgRef}
      className="absolute inset-0 w-full h-full overflow-visible z-30 cursor-move"
    >
      <defs>
        <clipPath id="hexagonClip">
          <polygon points="32,0 60,16 60,48 32,64 4,48 4,16" />
        </clipPath>

        <linearGradient id="hubGradient" x1="0%" y1="0%" x2="100%" y2="100%">
          <stop offset="0%" stopColor="#4f46e5" />
          <stop offset="100%" stopColor="#1e1b4b" />
        </linearGradient>

        <filter id="neonGlow" x="-50%" y="-50%" width="200%" height="200%">
          <feGaussianBlur in="SourceGraphic" stdDeviation="4" result="blur" />
          <feComposite in="SourceGraphic" in2="blur" operator="over" />
        </filter>

        <marker
          id="arrowhead"
          markerWidth="6"
          markerHeight="6"
          refX="5"
          refY="3"
          orient="auto"
        >
          <polygon
            points="0 0, 6 3, 0 6"
            fill="currentColor"
            className="text-primary/40"
          />
        </marker>

        <linearGradient id="edgeGradient" x1="0%" y1="0%" x2="100%" y2="100%">
          <stop offset="0%" stopColor="var(--color-primary)" stopOpacity="0.2" />
          <stop offset="50%" stopColor="var(--color-primary)" stopOpacity="1" />
          <stop offset="100%" stopColor="var(--color-primary)" stopOpacity="0.2" />
        </linearGradient>
      </defs>

      <g transform={`translate(${transform.x},${transform.y}) scale(${transform.k})`}>
        <defs>
          <pattern id="grid" width="100" height="100" patternUnits="userSpaceOnUse">
            <path
              d="M 100 0 L 0 0 0 100"
              fill="none"
              stroke="currentColor"
              strokeWidth="0.5"
              className="text-slate-200 dark:text-slate-800"
              opacity="0.3"
            />
            <circle cx="0" cy="0" r="1.5" fill="currentColor" className="text-slate-300 dark:text-slate-700" />
          </pattern>
        </defs>

        <rect x="-5000" y="-5000" width="10000" height="10000" fill="url(#grid)" />

        {edges.map((edge) => {
          const isSourceCrashed = edge.source.status === "crashed";
          const isTargetCrashed = edge.target.status === "crashed";
          const isBroken = isSourceCrashed || isTargetCrashed;

          return (
            <g key={edge.id}>
              <line
                x1={edge.source.x}
                y1={edge.source.y}
                x2={edge.target.x}
                y2={edge.target.y}
                stroke="currentColor"
                className={`transition-all duration-300 ${
                  isBroken ? "text-border/20" : "text-primary/10"
                }`}
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
                  strokeDasharray="10, 20"
                  className="animate-pulse-slow"
                  style={{
                    filter: "blur(1px)",
                    animation: "pulseData 3s linear infinite",
                  }}
                />
              )}
            </g>
          );
        })}

        <style>{`
          @keyframes pulseData {
            from { stroke-dashoffset: 100; }
            to { stroke-dashoffset: 0; }
          }
        `}</style>

        {isSimulationRunning &&
          positionedNodes.map((n) => {
            if (n.status === "crashed") return null;
            const rState = raftState[n.id];
            if (!rState) return null;

            const circumference = 2 * Math.PI * 42;
            let progress = rState.timerProgress / rState.timeoutLimit;
            if (rState.role === "leader") progress = 1;
            const offset = circumference - progress * circumference;

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

        {isSimulationRunning &&
          messages.map((msg, idx) => {
            const src = msg.sourceId === "CP" ? cpNode : positionedNodes.find((n) => n.id === msg.sourceId);
            const tgt = msg.targetId === "CP" ? cpNode : positionedNodes.find((n) => n.id === msg.targetId);
            if (!src || !tgt || (src.id !== "CP" && (src as any).status === "crashed")) return null;
            if (!showControlPlane && (msg.sourceId === "CP" || msg.targetId === "CP")) return null;

            const t = msg.progress;
            const x = src.x + (tgt.x - src.x) * t;
            const y = src.y + (tgt.y - src.y) * t;
            const isHovered = hoveredMessageId === msg.id;

            let color = "#f43f5e";
            let orbSize = 6;
            if (msg.type === "vote_request") {
              color = "#3b82f6";
              orbSize = 7;
            } else if (msg.type === "GOSSIP_DIGEST") {
              color = "#eab308";
              orbSize = 5;
            } else if (msg.type === "GOSSIP_STATE") {
              color = "#10b981";
              orbSize = 7;
            } else if (msg.type.includes("CP")) {
              color = "#a855f7";
              orbSize = 8;
            }

            const isSelected = selectedMessageId === msg.id;

            return (
              <g
                key={`${msg.id}-${idx}`}
                onClick={(e) => {
                  e.stopPropagation();
                  setSelectedMessageId(msg.id);
                }}
                onMouseEnter={() => isPaused && setHoveredMessageId(msg.id)}
                onMouseLeave={() => setHoveredMessageId(null)}
                className="pointer-events-auto cursor-pointer"
              >
                <circle
                  cx={x}
                  cy={y}
                  r={orbSize + (isHovered || isSelected ? 18 : 14)}
                  fill={color}
                  opacity={isHovered || isSelected ? "0.4" : "0.2"}
                  style={{ filter: `blur(12px)` }}
                />
                <circle
                  cx={x}
                  cy={y}
                  r={orbSize + (isHovered || isSelected ? 3 : 1)}
                  fill="white"
                  opacity="0.3"
                  style={{ filter: "blur(2px)" }}
                />
                <circle
                  cx={x}
                  cy={y}
                  r={orbSize}
                  fill={color}
                  style={{
                    filter: `drop-shadow(0 0 10px ${color})`,
                    stroke: isSelected ? "white" : "none",
                    strokeWidth: 2,
                  }}
                />
              </g>
            );
          })}

        {positionedNodes
          .filter((n) => (n as any).isHub !== true)
          .map((n) => {
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
                height={64}
                className="overflow-visible"
              >
                <div
                  className="node-draggable w-16 h-16 flex flex-col items-center justify-center pointer-events-auto cursor-grab active:cursor-grabbing"
                  style={{ touchAction: "none" }}
                  onPointerDown={(e) => {
                    e.preventDefault();
                    e.stopPropagation();
                    e.nativeEvent.stopImmediatePropagation();
                    const target = e.currentTarget as HTMLElement;
                    target.setPointerCapture(e.pointerId);
                    handleDragStart(n.id, e.clientX, e.clientY);
                  }}
                  onPointerMove={(e) => {
                    const target = e.currentTarget as HTMLElement;
                    if (target.hasPointerCapture(e.pointerId)) {
                      e.preventDefault();
                      e.stopPropagation();
                      e.nativeEvent.stopImmediatePropagation();
                      handleDrag(e.clientX, e.clientY);
                    }
                  }}
                  onPointerUp={(e) => {
                    e.preventDefault();
                    e.stopPropagation();
                    e.nativeEvent.stopImmediatePropagation();
                    const target = e.currentTarget as HTMLElement;
                    if (target.hasPointerCapture(e.pointerId)) {
                      target.releasePointerCapture(e.pointerId);
                    }
                    handleDragEnd();
                  }}
                  onPointerCancel={(e) => {
                    e.preventDefault();
                    e.stopPropagation();
                    e.nativeEvent.stopImmediatePropagation();
                    const target = e.currentTarget as HTMLElement;
                    if (target.hasPointerCapture(e.pointerId)) {
                      target.releasePointerCapture(e.pointerId);
                    }
                    handleDragEnd();
                  }}
                >
                  <div
                    className={`w-full h-full rounded-2xl border-2 flex flex-col items-center justify-center transition-all duration-300 ${nodeBg} ${nodeBorder} shadow-lg`}
                  >
                    <Icon className={`w-6 h-6 mb-1 ${roleColor}`} />
                    <span className="text-[10px] font-bold uppercase tracking-wider text-slate-700 dark:text-slate-300">
                      {n.id}
                    </span>
                  </div>
                </div>
              </foreignObject>
            );
          })}

        {isSimulationRunning &&
          positionedNodes
            .filter((n) => (n as any).isHub !== true)
            .map((n) => {
              if (n.status === "crashed") return null;
              const rState = raftState[n.id];
              if (!rState) return null;

              const badgeBg =
                rState.role === "leader"
                  ? "#f59e0b"
                  : rState.role === "candidate"
                  ? "#3b82f6"
                  : "#10b981";
              const badgeBorder =
                rState.role === "leader"
                  ? "#fcd34d"
                  : rState.role === "candidate"
                  ? "#93c5fd"
                  : "#6ee7b7";
              const badgeLabel =
                rState.role === "leader"
                  ? "L"
                  : rState.role === "candidate"
                  ? "C"
                  : "F";

              return (
                <g
                  key={`badge-${n.id}`}
                  transform={`translate(${n.x + 20}, ${n.y - 20})`}
                  style={{ pointerEvents: "none" }}
                >
                  <circle r="10" fill={badgeBg} stroke={badgeBorder} strokeWidth="2" />
                  <text
                    textAnchor="middle"
                    dominantBaseline="central"
                    fontSize="9"
                    fontWeight="bold"
                    fill="white"
                  >
                    {badgeLabel}
                  </text>
                </g>
              );
            })}

        {showControlPlane && (
          <foreignObject
            x={centerX - 40}
            y={centerY - 32}
            width={80}
            height={100}
            className="pointer-events-none"
          >
            <div className="w-full h-full flex flex-col items-center justify-center">
              <div className="w-16 h-16 rounded-2xl border-2 border-primary/40 bg-white/20 dark:bg-slate-900/40 backdrop-blur-xl shadow-xl flex items-center justify-center overflow-hidden">
                <Server className="w-8 h-8 text-primary" />
              </div>
              <span className="mt-1 text-[10px] font-extrabold uppercase tracking-widest text-slate-800 dark:text-slate-200">
                HUB
              </span>
            </div>
          </foreignObject>
        )}
      </g>
    </svg>
  );
}
