import React, { useState } from "react";
import { Plus, Server, Network, ChevronRight, Activity, ShieldCheck, HelpCircle } from "lucide-react";
import { useClusterStore } from "../store";

export function Sidebar() {
  const { clusters, selectedClusterId, setSelectedClusterId, handleCreateCluster, isSimulationRunning, isPaused, togglePause, isLiveMode } = useClusterStore();
  const [isCreatingCluster, setIsCreatingCluster] = useState(false);
  const [newClusterId, setNewClusterId] = useState("");
  const [newClusterProtocol, setNewClusterProtocol] = useState("gossip");

  const submitCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newClusterId.trim()) return;
    const success = await handleCreateCluster(newClusterId, newClusterProtocol);
    if (success) {
      setNewClusterId("");
      setIsCreatingCluster(false);
    }
  };

  return (
    <aside className="w-72 flex-shrink-0 border-r border-border bg-card flex flex-col fixed h-full z-10">
      {/* Logo */}
      <div className="p-5 border-b border-border/50">
        <div className="flex items-center gap-3">
          <div className="w-9 h-9 rounded-xl bg-gradient-to-br from-primary to-accent flex items-center justify-center shadow-lg shadow-primary/20">
            <Network className="w-5 h-5 text-primary-foreground" />
          </div>
          <div>
            <div className="flex items-center gap-1.5">
              <span className="font-bold text-lg tracking-tight">Faultlab</span>
              {isLiveMode ? (
                <div className="flex items-center gap-1 px-1.5 py-0.5 rounded-full bg-success/10 border border-success/20 animate-in fade-in zoom-in duration-500">
                  <span className="w-1.5 h-1.5 rounded-full bg-success animate-pulse" />
                  <span className="text-[8px] font-black tracking-tighter text-success uppercase">Live</span>
                </div>
              ) : (
                <div className="flex items-center gap-1 px-1.5 py-0.5 rounded-full bg-amber-500/10 border border-amber-500/20 animate-in fade-in zoom-in duration-500">
                  <span className="w-1.5 h-1.5 rounded-full bg-amber-500" />
                  <span className="text-[8px] font-black tracking-tighter text-amber-600 uppercase">Sim</span>
                </div>
              )}
            </div>
            <p className="text-[10px] text-muted-foreground font-medium">Distributed Systems Lab</p>
          </div>
        </div>
      </div>

      {/* Clusters Section */}
      <div className="flex-1 flex flex-col overflow-hidden">
        <div className="flex items-center justify-between px-4 py-3">
          <h2 className="text-xs uppercase font-bold text-muted-foreground tracking-widest">Clusters</h2>
          <button
            onClick={() => setIsCreatingCluster(!isCreatingCluster)}
            className="w-7 h-7 rounded-lg bg-primary/10 hover:bg-primary/20 flex items-center justify-center transition-colors group"
            title="Create new cluster"
          >
            <Plus className="w-4 h-4 text-primary group-hover:scale-110 transition-transform" />
          </button>
        </div>

        {/* Create Cluster Form */}
        {isCreatingCluster && (
          <div className="px-4 pb-3 animate-in fade-in slide-in-from-top-2 duration-200">
            <form onSubmit={submitCreate} className="p-3 rounded-xl border border-border bg-gradient-to-br from-muted/50 to-muted/30 flex flex-col gap-2.5 shadow-inner">
              <input
                value={newClusterId}
                onChange={(e) => setNewClusterId(e.target.value)}
                placeholder="Cluster ID"
                className="input-field font-mono text-xs"
                autoFocus
              />
              <select
                value={newClusterProtocol}
                onChange={(e) => setNewClusterProtocol(e.target.value)}
                className="input-field text-xs"
              >
                <option value="gossip">Gossip Protocol</option>
                <option value="raft">Raft Protocol</option>
                <option value="baseline">Baseline Protocol</option>
              </select>
              <button type="submit" className="btn-primary text-xs flex items-center justify-center gap-1.5">
                <Plus className="w-3.5 h-3.5" />
                Create Cluster
              </button>
            </form>
          </div>
        )}

        {/* Cluster List */}
        <div className="flex-1 overflow-y-auto px-3 pb-3 space-y-1 hide-scrollbar">
          {clusters.map((cluster) => {
            const isSelected = selectedClusterId === cluster.id;
            const nodeCount = cluster.nodes?.length || 0;
            
            return (
              <button
                key={cluster.id}
                onClick={() => setSelectedClusterId(cluster.id)}
                className={`w-full text-left flex items-center gap-3 px-3 py-2.5 rounded-xl transition-all duration-200 group ${
                  isSelected
                    ? "bg-gradient-to-r from-primary/15 to-accent/10 border border-primary/20 shadow-sm"
                    : "hover:bg-muted/50 border border-transparent"
                }`}
              >
                <div className={`w-8 h-8 rounded-lg flex items-center justify-center flex-shrink-0 transition-all ${
                  isSelected 
                    ? "bg-primary text-primary-foreground shadow-md shadow-primary/20" 
                    : "bg-muted text-muted-foreground group-hover:bg-primary/10 group-hover:text-primary"
                }`}>
                  <Server className="w-4 h-4" />
                </div>
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2">
                    <span className={`text-sm font-semibold truncate ${
                      isSelected ? "text-primary" : "text-foreground"
                    }`}>
                      {cluster.id}
                    </span>
                  </div>
                  <div className="flex items-center gap-2 mt-0.5">
                    <span className="text-[10px] text-muted-foreground font-mono">
                      {nodeCount} {nodeCount === 1 ? "node" : "nodes"}
                    </span>
                    <span className="text-[9px] uppercase font-bold tracking-wider text-muted-foreground/60">
                      •
                    </span>
                    <span className="text-[10px] uppercase font-mono tracking-wider text-muted-foreground/80 bg-muted px-1.5 py-0.5 rounded">
                      {cluster.protocol || "unknown"}
                    </span>
                  </div>
                </div>
                {isSelected && (
                  <ChevronRight className="w-4 h-4 text-primary/60 flex-shrink-0" />
                )}
              </button>
            );
          })}
          
          {clusters.length === 0 && !isCreatingCluster && (
            <div className="text-center py-8 px-4">
              <div className="w-12 h-12 rounded-full bg-muted/50 flex items-center justify-center mx-auto mb-3">
                <Network className="w-6 h-6 text-muted-foreground/50" />
              </div>
              <p className="text-sm font-medium text-muted-foreground">No clusters yet</p>
              <p className="text-xs text-muted-foreground/70 mt-1">Create your first cluster to begin</p>
            </div>
          )}
        </div>
      </div>

      {/* Bottom Actions */}
      <div className="p-4 border-t border-border/50 bg-gradient-to-t from-muted/30 to-transparent">
        <div className="space-y-3">
          <button 
            onClick={togglePause}
            className={`flex items-center justify-between w-full p-2.5 rounded-xl border transition-all group ${
              isPaused 
                ? "bg-warning/10 border-warning/30 text-warning" 
                : "bg-card border-border hover:border-primary/30"
            }`}
          >
            <div className="flex items-center gap-3">
              <div className={`w-7 h-7 rounded-lg flex items-center justify-center transition-all ${
                isPaused 
                  ? "bg-warning/20 text-warning" 
                  : "bg-primary/10 text-primary group-hover:bg-primary/20"
              }`}>
                <Network className={`w-3.5 h-3.5 ${isPaused ? "animate-pulse" : ""}`} />
              </div>
              <div className="text-left">
                <span className="text-xs font-medium">{isPaused ? "Paused" : "Live Simulation"}</span>
              </div>
            </div>
            <div className={`w-1.5 h-1.5 rounded-full ${isPaused ? "bg-warning" : "bg-success"} animate-pulse`} />
          </button>

          <div className="pt-1">
            <div className="p-3 rounded-xl bg-muted/30 border border-border/50 flex items-center justify-between group hover:border-primary/20 transition-all">
              <div className="flex items-center gap-3">
                <div className={`w-8 h-8 rounded-lg flex items-center justify-center transition-all ${isLiveMode ? 'bg-success/10 text-success' : 'bg-amber-500/10 text-amber-600'}`}>
                  {isLiveMode ? <Activity className="w-4 h-4" /> : <ShieldCheck className="w-4 h-4" />}
                </div>
                <div>
                  <p className="text-[10px] uppercase font-bold text-muted-foreground/60 tracking-wider leading-tight">System Status</p>
                  <p className="text-xs font-semibold leading-tight">{isLiveMode ? 'Connected to Controlplane' : 'Local Browser Simulation'}</p>
                </div>
              </div>
            </div>
          </div>
          
          <div className="text-[10px] text-center text-muted-foreground/60 pt-2 border-t border-border/20 mt-1">
            Faultlab v1.0
          </div>
        </div>
      </div>
    </aside>
  );
}
