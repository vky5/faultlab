import React, { useEffect, useState, useRef, useMemo, useCallback } from "react";
import { Activity } from "lucide-react";
import { AnimatePresence } from "framer-motion";
import {
  forceSimulation,
  forceManyBody,
  forceCenter,
  forceRadial,
  forceCollide,
  forceX,
  forceY,
  Simulation,
  SimulationNodeDatum,
} from "d3-force";
import { zoom, ZoomBehavior, zoomIdentity } from "d3-zoom";
import { select } from "d3-selection";
import { NodeInfo, useClusterStore } from "../store";
import { MetricsPanel } from "./MetricsPanel";
import { GraphCanvas, PositionedNode } from "./topology/GraphCanvas";
import { MessageInspector } from "./topology/MessageInspector";
import { ZoomControls } from "./topology/ZoomControls";
import { LegendPanel } from "./topology/LegendPanel";

interface SimNode extends SimulationNodeDatum {
  id: string;
  node: NodeInfo;
  x?: number;
  y?: number;
  fx?: number | null;
  fy?: number | null;
}

export function TopologyGraph({ nodes }: { nodes: NodeInfo[] }) {
  const containerRef = useRef<HTMLDivElement>(null);
  const [dimensions, setDimensions] = useState({ w: 0, h: 0 });
  const {
    raftState,
    messages,
    isSimulationRunning,
    isPaused,
    showControlPlane,
    toggleControlPlane,
    showMetrics,
    setShowMetrics,
    metrics,
  } = useClusterStore();
  const [isMetricsExpanded, setIsMetricsExpanded] = useState(false);
  const [hoveredMessageId, setHoveredMessageId] = useState<string | null>(null);
  const [simNodes, setSimNodes] = useState<SimNode[]>([]);
  const svgRef = useRef<SVGSVGElement>(null);
  const [transform, setTransform] = useState({ x: 0, y: 0, k: 1 });

  const selectedMessageId = useClusterStore((s) => s.selectedMessageId);
  const setSelectedMessageId = useClusterStore((s) => s.setSelectedMessageId);

  const selectedMessage = useMemo(
    () => messages.find((m) => m.id === selectedMessageId),
    [messages, selectedMessageId]
  );

  const effectiveW = dimensions.w || 1000;
  const effectiveH = dimensions.h || 800;
  const centerX = effectiveW / 2;
  const centerY = effectiveH / 2;

  const simulation = useRef<Simulation<SimNode, undefined> | null>(null);
  const zoomBehavior = useRef<ZoomBehavior<SVGSVGElement, unknown> | null>(null);
  const dragInfo = useRef<{
    id: string;
    startX: number;
    startY: number;
    nodeX: number;
    nodeY: number;
  } | null>(null);
  const isDraggingRef = useRef(false);

  useEffect(() => {
    if (!svgRef.current) return;
    const svgEl = svgRef.current;

    zoomBehavior.current = zoom<SVGSVGElement, unknown>()
      .filter((event) => {
        if (event.ctrlKey || event.button) return false;
        const target = event.target as Element | null;
        if (target && typeof target.closest === 'function' && target.closest(".node-draggable")) {
          return false;
        }
        if (isDraggingRef.current) return false;
        return event.type !== "dblclick";
      })
      .scaleExtent([0.1, 5])
      .on("zoom", (event) => {
        setTransform(event.transform);
      });

    select(svgEl).call(zoomBehavior.current);

    return () => {
      select(svgEl).on(".zoom", null);
    };
  }, []);

  const handleZoomIn = useCallback(() => {
    if (svgRef.current && zoomBehavior.current) {
      select(svgRef.current).call(zoomBehavior.current.scaleBy, 1.4);
    }
  }, []);

  const handleZoomOut = useCallback(() => {
    if (svgRef.current && zoomBehavior.current) {
      select(svgRef.current).call(zoomBehavior.current.scaleBy, 0.7);
    }
  }, []);

  const handleResetZoom = useCallback(() => {
    if (svgRef.current && zoomBehavior.current) {
      select(svgRef.current).call(zoomBehavior.current.transform, zoomIdentity);
    }
  }, []);

  useEffect(() => {
    simulation.current = forceSimulation<SimNode>()
      .force("charge", forceManyBody().strength(-300))
      .force("radial", forceRadial(220, 500, 400).strength(0.8))
      .force("collision", forceCollide<SimNode>().radius(85))
      .alphaDecay(0.02)
      .on("tick", () => {
        if (simulation.current) {
          setSimNodes([...simulation.current.nodes()]);
        }
      });

    return () => {
      simulation.current?.stop();
    };
  }, []);

  useEffect(() => {
    if (!simulation.current) return;

    const nodeData = [...nodes];
    if (showControlPlane) {
      nodeData.push({
        id: "CP",
        status: "active",
        isHub: true,
        address: "",
        port: 0,
      } as NodeInfo & { isHub: true });
    }

    const currentSimNodes = simulation.current.nodes();
    const nextNodes = nodeData.map((node) => {
      const existing = currentSimNodes.find((n) => n.id === node.id);
      if (existing) {
        existing.node = node;
        if (node.id === "CP") {
          existing.fx = centerX;
          existing.fy = centerY;
        }
        return existing;
      }

      const initX =
        node.id === "CP"
          ? centerX
          : centerX + (Math.random() - 0.5) * 200;
      const initY =
        node.id === "CP"
          ? centerY
          : centerY + (Math.random() - 0.5) * 200;

      return {
        id: node.id,
        node,
        x: initX,
        y: initY,
        vx: 0,
        vy: 0,
        fx: node.id === "CP" ? centerX : undefined,
        fy: node.id === "CP" ? centerY : undefined,
      } as SimNode;
    });

    simulation.current.nodes(nextNodes);
    simulation.current.alpha(0.3).restart();
    setSimNodes([...nextNodes]);
  }, [nodes, centerX, centerY, showControlPlane]);

  useEffect(() => {
    if (!simulation.current) return;
    simulation.current.force("x", forceX(centerX).strength(0.08));
    simulation.current.force("y", forceY(centerY).strength(0.08));
    simulation.current.force("radial", forceRadial(220, centerX, centerY).strength(0.8));
    simulation.current.alpha(0.1).restart();
  }, [centerX, centerY]);

  useEffect(() => {
    const container = containerRef.current;
    if (!container) return;
    
    const updateSize = () => {
      setDimensions({
        w: container.clientWidth,
        h: container.clientHeight,
      });
    };
    
    updateSize();
    
    const resizeObserver = new ResizeObserver(() => {
      updateSize();
    });
    
    resizeObserver.observe(container);
    
    return () => {
      resizeObserver.disconnect();
    };
  }, []);

  const prevIsActive = useRef(false);
  useEffect(() => {
    if (!metrics) return;

    const isActive =
      metrics.isActive ??
      (metrics.stoppedAt === "0001-01-01T00:00:00Z" || !metrics.stoppedAt);

    prevIsActive.current = isActive;
  }, [metrics]);

  const positionedNodes = simNodes.map((sn) => ({
    ...sn.node,
    x: sn.x || 0,
    y: sn.y || 0,
  }));

  const cpNode = {
    id: "CP",
    x: centerX,
    y: centerY,
    status: "active",
  };

  type EdgeDescriptor = {
    source: PositionedNode | { id: string; x: number; y: number; status: string };
    target: PositionedNode | { id: string; x: number; y: number; status: string };
    id: string;
  };

  const edges: EdgeDescriptor[] = [];
  for (let i = 0; i < positionedNodes.length; i++) {
    const source = positionedNodes[i];

    for (let j = i + 1; j < positionedNodes.length; j++) {
      const target = positionedNodes[j];

      const isPartitioned =
        source.fault?.partition?.includes(target.id) ||
        target.fault?.partition?.includes(source.id);

      if (!isPartitioned) {
        edges.push({ source, target, id: `${source.id}-${target.id}` });
      }
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

  const getSVGPoint = (clientX: number, clientY: number) => {
    if (!svgRef.current) return { x: 0, y: 0 };
    const svg = svgRef.current;
    const pt = svg.createSVGPoint();
    pt.x = clientX;
    pt.y = clientY;

    const rootPoint = pt.matrixTransform(svg.getScreenCTM()?.inverse());
    const scale = transform.k || 1;
    return {
      x: (rootPoint.x - transform.x) / scale,
      y: (rootPoint.y - transform.y) / scale,
    };
  };

  const handleDragStart = (id: string, clientX: number, clientY: number) => {
    const node = simNodes.find((n) => n.id === id);
    if (node) {
      const pt = getSVGPoint(clientX, clientY);
      dragInfo.current = {
        id,
        startX: pt.x,
        startY: pt.y,
        nodeX: node.fx !== undefined ? (node.fx as number) : node.x || 0,
        nodeY: node.fy !== undefined ? (node.fy as number) : node.y || 0,
      };
      isDraggingRef.current = true;
      node.fx = node.x;
      node.fy = node.y;
      simulation.current?.alphaTarget(0.3).restart();
    }
  };

  const handleDrag = (clientX: number, clientY: number) => {
    if (!dragInfo.current) return;
    const { id, startX, startY, nodeX, nodeY } = dragInfo.current;
    const node = simNodes.find((n) => n.id === id);
    if (node) {
      const pt = getSVGPoint(clientX, clientY);
      node.fx = nodeX + (pt.x - startX);
      node.fy = nodeY + (pt.y - startY);
    }
  };

  const handleDragEnd = () => {
    if (dragInfo.current) {
      dragInfo.current = null;
      isDraggingRef.current = false;
      simulation.current?.alphaTarget(0);
    }
  };

  return (
    <div
      ref={containerRef}
      className="w-full h-full relative z-10 overflow-hidden select-none"
    >
      <GraphCanvas
        svgRef={svgRef}
        transform={transform}
        centerX={centerX}
        centerY={centerY}
        cpNode={cpNode}
        positionedNodes={positionedNodes}
        edges={edges}
        messages={messages}
        hoveredMessageId={hoveredMessageId}
        setHoveredMessageId={setHoveredMessageId}
        selectedMessageId={selectedMessageId}
        setSelectedMessageId={setSelectedMessageId}
        raftState={raftState}
        isSimulationRunning={isSimulationRunning}
        isPaused={isPaused}
        showControlPlane={showControlPlane}
        handleDragStart={handleDragStart}
        handleDrag={handleDrag}
        handleDragEnd={handleDragEnd}
      />

      <MessageInspector
        selectedMessage={selectedMessage}
        setSelectedMessageId={setSelectedMessageId}
      />

      <ZoomControls
        handleZoomIn={handleZoomIn}
        handleZoomOut={handleZoomOut}
        handleResetZoom={handleResetZoom}
        toggleControlPlane={toggleControlPlane}
        showControlPlane={showControlPlane}
        showMetrics={showMetrics}
        setShowMetrics={setShowMetrics}
      />

      <LegendPanel messageCount={messages.length} isPaused={isPaused} />

      <AnimatePresence>
        {showMetrics && (
          <MetricsPanel
            isExpanded={isMetricsExpanded}
            onToggleExpand={() => setIsMetricsExpanded(!isMetricsExpanded)}
          />
        )}
      </AnimatePresence>
    </div>
  );
}
