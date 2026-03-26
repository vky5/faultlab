"use client";

import React, { useEffect } from "react";
import { Server, Network, TrendingUp } from "lucide-react";
import { useClusterStore } from "./store";
import { Sidebar } from "./components/Sidebar";
import { NodeManager } from "./components/NodeManager";
import { TopologyGraph } from "./components/TopologyGraph";
import { useRaftSimulation } from "./components/useRaftSimulation";

export default function DashboardPage() {
  const { clusters, selectedClusterId, fetchClusters, getNodeStatus } = useClusterStore();

  useRaftSimulation();

  useEffect(() => {
    fetchClusters();
    const interval = setInterval(fetchClusters, 2000);
    return () => clearInterval(interval);
  }, [fetchClusters]);

  const selectedCluster = clusters.find((c) => c.id === selectedClusterId);
  
  // Calculate cluster health stats
  const totalNodes = selectedCluster?.nodes?.length || 0;
  const crashedNodes = selectedCluster?.nodes?.filter(n => getNodeStatus(n) === "crashed").length || 0;
  const activeNodes = totalNodes - crashedNodes;
  const healthPercentage = totalNodes > 0 ? Math.round((activeNodes / totalNodes) * 100) : 100;

  return (
    <div className="flex h-screen bg-gradient-to-br from-background via-background to-muted/20 text-foreground antialiased selection:bg-primary/20 selection:text-primary-foreground">
      <Sidebar />

      <main className="flex-1 ml-72 overflow-auto">
        <div className="max-w-[1600px] mx-auto px-6 py-6 h-full flex flex-col gap-6">
          {selectedCluster ? (
            <>
              {/* Header */}
              <header className="flex flex-col gap-4">
                <div className="flex items-start justify-between">
                  <div className="flex items-center gap-4">
                    <div className="w-12 h-12 rounded-2xl bg-gradient-to-br from-primary/20 to-accent/20 border border-primary/30 flex items-center justify-center">
                      <Network className="w-6 h-6 text-primary" />
                    </div>
                    <div>
                      <div className="flex items-center gap-3">
                        <h1 className="text-3xl font-bold tracking-tight">{selectedCluster.id}</h1>
                        <span className="badge bg-primary/10 text-primary uppercase font-mono tracking-wider border border-primary/20 text-xs">
                          {selectedCluster.protocol || "protocol"}
                        </span>
                      </div>
                      <p className="text-sm text-muted-foreground mt-1">
                        Cluster topology and fault injection control
                      </p>
                    </div>
                  </div>
                  
                  <div className={`flex items-center gap-2 px-4 py-2 rounded-full border ${
                    healthPercentage === 100 
                      ? "bg-success/10 border-success/20 text-success" 
                      : healthPercentage > 50
                      ? "bg-warning/10 border-warning/20 text-warning"
                      : "bg-destructive/10 border-destructive/20 text-destructive"
                  }`}>
                    <div className={`w-2 h-2 rounded-full ${
                      healthPercentage === 100 ? "bg-success" : healthPercentage > 50 ? "bg-warning" : "bg-destructive"
                    } animate-pulse`} />
                    <span className="text-sm font-semibold">
                      {healthPercentage === 100 ? "Healthy" : healthPercentage > 50 ? "Degraded" : "Critical"}
                    </span>
                    <span className="text-xs opacity-70 ml-1">
                      ({activeNodes}/{totalNodes} active)
                    </span>
                  </div>
                </div>
                
                {/* Stats Bar */}
                <div className="grid grid-cols-3 gap-3">
                  <div className="card py-3 px-4 flex items-center gap-3">
                    <div className="w-10 h-10 rounded-xl bg-primary/10 flex items-center justify-center">
                      <Server className="w-5 h-5 text-primary" />
                    </div>
                    <div>
                      <div className="text-2xl font-bold text-foreground">{totalNodes}</div>
                      <div className="text-xs text-muted-foreground font-medium">Total Nodes</div>
                    </div>
                  </div>
                  <div className="card py-3 px-4 flex items-center gap-3">
                    <div className="w-10 h-10 rounded-xl bg-success/10 flex items-center justify-center">
                      <TrendingUp className="w-5 h-5 text-success" />
                    </div>
                    <div>
                      <div className="text-2xl font-bold text-success">{activeNodes}</div>
                      <div className="text-xs text-muted-foreground font-medium">Active</div>
                    </div>
                  </div>
                  <div className="card py-3 px-4 flex items-center gap-3">
                    <div className="w-10 h-10 rounded-xl bg-destructive/10 flex items-center justify-center">
                      <Server className="w-5 h-5 text-destructive" />
                    </div>
                    <div>
                      <div className="text-2xl font-bold text-destructive">{crashedNodes}</div>
                      <div className="text-xs text-muted-foreground font-medium">Crashed</div>
                    </div>
                  </div>
                </div>
              </header>

              {/* Main Content */}
              <div className="flex gap-6 flex-1 min-h-0">
                <NodeManager selectedCluster={selectedCluster} />

                <div className="flex-1 card flex flex-col p-0 overflow-hidden relative bg-gradient-to-br from-background to-muted/30">
                  {/* Grid pattern background */}
                  <div className="absolute inset-0 opacity-[0.03]" 
                    style={{
                      backgroundImage: `linear-gradient(to right, currentColor 1px, transparent 1px),
                                       linear-gradient(to bottom, currentColor 1px, transparent 1px)`,
                      backgroundSize: "32px 32px"
                    }} 
                  />
                  
                  {/* Radial gradient overlay */}
                  <div className="absolute inset-0 bg-gradient-to-br from-primary/5 via-transparent to-accent/5" />
                  
                  <TopologyGraph nodes={(selectedCluster.nodes || []).map(n => ({...n, status: getNodeStatus(n) as "active" | "crashed"}))} />
                </div>
              </div>
            </>
          ) : (
            /* Empty State */
            <div className="flex-1 flex flex-col items-center justify-center text-muted-foreground">
              <div className="w-24 h-24 rounded-3xl bg-gradient-to-br from-muted/50 to-muted/30 border border-border flex items-center justify-center mb-6">
                <Server className="w-12 h-12 opacity-30" />
              </div>
              <h2 className="text-2xl font-bold text-foreground mb-2">No Cluster Selected</h2>
              <p className="text-sm text-muted-foreground max-w-md text-center mb-6">
                Select an existing cluster from the sidebar or create a new one to start managing your distributed system topology.
              </p>
              <div className="flex items-center gap-2 text-xs text-muted-foreground/60">
                <div className="w-2 h-2 rounded-full bg-primary/50" />
                <span>Create a cluster to begin experimenting with fault injection</span>
              </div>
            </div>
          )}
        </div>
      </main>
    </div>
  );
}
