import React, { useState } from "react";
import { Plus, Trash2, ShieldAlert, Zap, PowerOff, RotateCcw, Activity, Network, Clock, Droplets, X, Database } from "lucide-react";
import { useClusterStore, ClusterInfo } from "../store";
import { KVManager } from "./KVManager";

export function NodeManager({ selectedCluster }: { selectedCluster: ClusterInfo }) {
  const {
    handleAddNode,
    handleRemoveNode,
    handleCrashNode,
    handleRecoverNode,
    handleSetDropRate,
    handleSetDelay,
    handleSetPartition,
    getNodeStatus,
  } = useClusterStore();

  const [isInjectOpen, setIsInjectOpen] = useState(false);
  const [newNodeId, setNewNodeId] = useState("");
  const [newNodeHost, setNewNodeHost] = useState("localhost");
  const [newNodePort, setNewNodePort] = useState(8001);
  const [expandedNodeId, setExpandedNodeId] = useState<string | null>(null);
  const [faultDrafts, setFaultDrafts] = useState<Record<string, { dropRate: number; delayMs: number }>>({});

  const submitAddNode = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newNodeId.trim()) return;
    const success = await handleAddNode(selectedCluster.id, { node_id: newNodeId, address: newNodeHost, port: Number(newNodePort) });
    if (success) {
      setNewNodeId("");
      setNewNodePort((prev) => prev + 1);
    }
  };

  const updateDraft = (nodeId: string, key: "dropRate" | "delayMs", value: number) => {
    setFaultDrafts((prev) => ({
      ...prev,
      [nodeId]: {
        dropRate: key === "dropRate" ? value : (prev[nodeId]?.dropRate ?? 0),
        delayMs: key === "delayMs" ? value : (prev[nodeId]?.delayMs ?? 0),
      },
    }));
  };

  // Sort nodes by ID for stable ordering
  const sortedNodes = [...(selectedCluster.nodes || [])].sort((a, b) => 
    a.id.localeCompare(b.id, undefined, { numeric: true, sensitivity: 'base' })
  );

  return (
    <div className="w-96 flex flex-col gap-4 flex-shrink-0">
      {/* Inject Node Card - Collapsible */}
      {!isInjectOpen ? (
        <button
          onClick={() => setIsInjectOpen(true)}
          className="card bg-gradient-to-br from-primary/5 to-accent/5 border-primary/20 hover:border-primary/40 transition-all group"
        >
          <div className="flex items-center justify-center gap-3 py-6">
            <div className="w-12 h-12 rounded-xl bg-primary/10 flex items-center justify-center group-hover:scale-110 transition-transform">
              <Plus className="w-6 h-6 text-primary" />
            </div>
          </div>
          <p className="text-center text-xs text-muted-foreground font-medium">Add Node to Cluster</p>
        </button>
      ) : (
        <div className="card bg-gradient-to-br from-primary/5 to-accent/5 border-primary/20 animate-in fade-in slide-in-from-top-2 duration-200">
          <div className="flex items-center justify-between mb-4">
            <div className="flex items-center gap-2">
              <div className="w-8 h-8 rounded-lg bg-primary/10 flex items-center justify-center">
                <Zap className="w-4 h-4 text-primary" />
              </div>
              <div>
                <h3 className="text-sm font-semibold">Inject Node</h3>
                <p className="text-xs text-muted-foreground">Add a new node to the cluster</p>
              </div>
            </div>
            <button
              onClick={() => setIsInjectOpen(false)}
              className="p-1.5 rounded-lg hover:bg-muted/50 transition-colors text-muted-foreground"
            >
              <X className="w-4 h-4" />
            </button>
          </div>
          
          <form onSubmit={submitAddNode} className="flex flex-col gap-3">
            <div>
              <label className="text-xs text-muted-foreground font-medium mb-1.5 block">Node ID</label>
              <input
                value={newNodeId}
                onChange={(e) => setNewNodeId(e.target.value)}
                placeholder="node-1"
                className="input-field font-mono"
                autoFocus
              />
            </div>
            <div className="grid grid-cols-3 gap-2">
              <div className="col-span-2">
                <label className="text-xs text-muted-foreground font-medium mb-1.5 block">Address</label>
                <input
                  value={newNodeHost}
                  onChange={(e) => setNewNodeHost(e.target.value)}
                  className="input-field font-mono text-xs"
                />
              </div>
              <div>
                <label className="text-xs text-muted-foreground font-medium mb-1.5 block">Port</label>
                <input
                  type="number"
                  value={newNodePort}
                  onChange={(e) => setNewNodePort(Number(e.target.value))}
                  className="input-field font-mono text-xs"
                />
              </div>
            </div>
            <button type="submit" className="btn-primary mt-1 flex items-center justify-center gap-2">
              <Plus className="w-4 h-4" />
              Add Node
            </button>
          </form>
        </div>
      )}

      {/* Nodes Panel */}
      <div className="card flex-1 flex flex-col p-0 overflow-hidden min-h-0">
        <div className="px-4 py-3 border-b border-border bg-gradient-to-r from-muted/50 to-muted/30 flex items-center justify-between">
          <div className="flex items-center gap-2">
            <Network className="w-4 h-4 text-primary" />
            <span className="text-sm font-semibold">Active Nodes</span>
          </div>
          <span className="text-xs font-mono bg-primary/10 text-primary px-2 py-1 rounded-full font-semibold">
            {selectedCluster.nodes?.length || 0}
          </span>
        </div>
        
        <div className="flex-1 overflow-y-auto p-3 space-y-2 hide-scrollbar">
          {sortedNodes.map((node) => {
            const status = getNodeStatus(node);
            const isCrashed = status === "crashed";
            const fault = node.fault || { crashed: isCrashed, dropRate: 0, delayMs: 0, partition: [] };
            const draft = faultDrafts[node.id] || { dropRate: fault.dropRate || 0, delayMs: fault.delayMs || 0 };
            const partitionPeers = new Set(fault.partition || []);
            const isExpanded = expandedNodeId === node.id;

            return (
              <div
                key={node.id}
                className={`node-card group relative overflow-hidden ${
                  isCrashed ? "bg-destructive/5 border-destructive/30" : ""
                }`}
              >
                {/* Status indicator bar */}
                <div className={`absolute left-0 top-0 bottom-0 w-1 ${
                  isCrashed ? "bg-destructive" : "bg-success"
                }`} />
                
                <div className="flex items-start justify-between gap-2 pl-2">
                  <div className="flex items-center gap-3 flex-1 min-w-0">
                    <div className={`w-10 h-10 rounded-xl flex items-center justify-center flex-shrink-0 transition-all ${
                      isCrashed 
                        ? "bg-destructive/10 text-destructive" 
                        : "bg-gradient-to-br from-primary/10 to-accent/10 text-primary"
                    }`}>
                      <Activity className="w-5 h-5" />
                    </div>
                    <div className="flex-1 min-w-0">
                      <div className="flex items-center gap-2">
                        <span className={`text-sm font-semibold truncate ${
                          isCrashed ? "text-muted-foreground line-through" : "text-foreground"
                        }`}>
                          {node.id}
                        </span>
                        {isCrashed && (
                          <span className="badge bg-destructive/10 text-destructive border-destructive/20 text-[10px]">
                            CRASHED
                          </span>
                        )}
                      </div>
                      <div className="text-xs text-muted-foreground font-mono flex items-center gap-1 mt-0.5">
                        <span>{node.address}:{node.port}</span>
                      </div>
                    </div>
                  </div>
                  
                  <div className="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
                    <button
                      onClick={(e) => {
                        e.stopPropagation();
                        if (isCrashed) {
                          handleRecoverNode(selectedCluster.id, node.id);
                        } else {
                          handleCrashNode(selectedCluster.id, node.id);
                        }
                      }}
                      className={`p-2 rounded-lg transition-all ${
                        isCrashed
                          ? "text-emerald-600 hover:bg-emerald-500/10 hover:scale-105"
                          : "text-orange-500 hover:bg-orange-500/10 hover:scale-105"
                      }`}
                      title={isCrashed ? "Recover Node" : "Crash Node"}
                    >
                      {isCrashed ? <RotateCcw className="w-4 h-4" /> : <PowerOff className="w-4 h-4" />}
                    </button>
                    <button
                      onClick={() => handleRemoveNode(selectedCluster.id, node.id)}
                      className="p-2 rounded-lg text-muted-foreground hover:bg-destructive/10 hover:text-destructive transition-all hover:scale-105"
                      title="Remove Node"
                    >
                      <Trash2 className="w-4 h-4" />
                    </button>
                  </div>
                </div>

                {/* Expandable Fault Controls */}
                <div className="mt-3 pt-3 border-t border-border/50">
                  <button
                    onClick={() => setExpandedNodeId(isExpanded ? null : node.id)}
                    className="text-xs text-muted-foreground hover:text-primary transition-colors flex items-center gap-1"
                  >
                    <Activity className="w-3 h-3" />
                    {isExpanded ? "Hide" : "Configure"} Fault Injection
                  </button>
                  
                  {isExpanded && (
                    <div className="mt-3 space-y-3 animate-in fade-in slide-in-from-top-2 duration-200">
                      {/* Drop Rate */}
                      <div className="flex gap-2 items-end">
                        <div className="flex-1">
                          <label className="text-[10px] uppercase tracking-wide text-muted-foreground font-semibold flex items-center gap-1 mb-1">
                            <Droplets className="w-3 h-3" />
                            Drop Rate
                          </label>
                          <input
                            type="number"
                            min={0}
                            max={1}
                            step={0.05}
                            value={draft.dropRate}
                            onChange={(e) => updateDraft(node.id, "dropRate", Number(e.target.value))}
                            className="input-field text-xs"
                          />
                        </div>
                        <button
                          onClick={() => handleSetDropRate(selectedCluster.id, node.id, Math.max(0, Math.min(1, draft.dropRate)))}
                          className="btn-secondary text-xs px-3"
                        >
                          Apply
                        </button>
                      </div>

                      {/* Delay */}
                      <div className="flex gap-2 items-end">
                        <div className="flex-1">
                          <label className="text-[10px] uppercase tracking-wide text-muted-foreground font-semibold flex items-center gap-1 mb-1">
                            <Clock className="w-3 h-3" />
                            Delay (ms)
                          </label>
                          <input
                            type="number"
                            min={0}
                            step={50}
                            value={draft.delayMs}
                            onChange={(e) => updateDraft(node.id, "delayMs", Number(e.target.value))}
                            className="input-field text-xs"
                          />
                        </div>
                        <button
                          onClick={() => handleSetDelay(selectedCluster.id, node.id, Math.max(0, draft.delayMs))}
                          className="btn-secondary text-xs px-3"
                        >
                          Apply
                        </button>
                      </div>

                      {/* Partition */}
                      <div>
                        <label className="text-[10px] uppercase tracking-wide text-muted-foreground font-semibold block mb-2">
                          Network Partition
                        </label>
                        <div className="flex flex-wrap gap-1.5">
                          {sortedNodes.filter((peer) => peer.id !== node.id).map((peer) => {
                            const enabled = partitionPeers.has(peer.id);
                            return (
                              <button
                                key={peer.id}
                                onClick={() => handleSetPartition(selectedCluster.id, node.id, peer.id, !enabled)}
                                className={`px-2.5 py-1 text-[11px] rounded-full border font-medium transition-all ${
                                  enabled 
                                    ? "bg-destructive/10 border-destructive/40 text-destructive hover:bg-destructive/20" 
                                    : "border-border text-muted-foreground hover:bg-muted hover:border-primary/30"
                                }`}
                              >
                                {peer.id}
                              </button>
                            );
                          })}
                          {selectedCluster.nodes.filter((peer) => peer.id !== node.id).length === 0 && (
                            <span className="text-xs text-muted-foreground italic">No other nodes to partition</span>
                          )}
                        </div>
                      </div>
                      {/* KV State Management */}
                      <div className="pt-3 border-t border-border/30">
                        <KVManager clusterId={selectedCluster.id} nodeId={node.id} />
                      </div>
                    </div>
                  )}
                </div>
              </div>
            );
          })}
          
          {sortedNodes.length === 0 && (
            <div className="text-center py-12 flex flex-col items-center gap-3 text-muted-foreground">
              <div className="w-12 h-12 rounded-full bg-muted/50 flex items-center justify-center">
                <ShieldAlert className="w-6 h-6 opacity-50" />
              </div>
              <div>
                <p className="text-sm font-medium">No nodes yet</p>
                <p className="text-xs mt-1">Add your first node to get started</p>
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
