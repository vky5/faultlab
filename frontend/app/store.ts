import { create } from "zustand";

export type NodeInfo = {
  id: string;
  address: string;
  port: number;
  status?: "active" | "crashed";
  fault?: {
    crashed: boolean;
    dropRate: number;
    delayMs: number;
    partition?: string[];
  };
  activeProtocolKey?: string;
  activeProtocolEpoch?: number;
  capabilities?: {
    kvPut: boolean;
    kvGet: boolean;
    kvDelete: boolean;
  };
};

export type ClusterInfo = {
  id: string;
  protocol: string;
  nodes: NodeInfo[];
};

export type Role = "follower" | "candidate" | "leader";

export interface NodeRaftState {
  role: Role;
  term: number;
  timerProgress: number;
  votesReceived: Set<string>;
  votedFor: string | null;
  timeoutLimit: number;
}

export interface SimulatedMessage {
  id: string;
  sourceId: string;
  targetId: string;
  type: string;
  progress: number;
  term: number;
  voteGranted?: boolean;
  metadata?: string;
  sizeBytes?: number;
  timestampMs?: number;
}

export interface DivergencePoint {
  Time: number; 
  Divergence: number;
}

export interface TimelineEvent {
  Time: number;
  NodeID: string;
  Key: string;
  Value: string;
  Version: number;
  Origin: string;
  Source: string;
  EventType: string;
  Round: number;
  LWWTimestamp?: number;
}

export interface MetricsResult {
  ConvergenceTime?: number;
  FirstAgreementTime?: number;
  FinalConsistent: boolean;
  PeakDivergence: number;
  AreaUnderDivergence: number;
  DivergenceOverTime: DivergencePoint[];
  StaleDurationPerNode: Record<string, number>;
  Timeline: TimelineEvent[];
  ConvergenceCurve: DivergencePoint[];
}

export interface MetricsSnapshot {
  isActive: boolean;
  clusterId: string;
  startedAt: string;
  stoppedAt?: string;
  trackedKeys: string[];
  results: Record<string, MetricsResult>;
  clusterStats?: {
    totalRPCs: number;
    totalWrites: number;
    recentKeys: string[];
  };
}

const API_BASE = "http://localhost:8080/api";
const METRICS_HISTORY_STORAGE_KEY = "faultlab.metrics.history.v1";

function readStoredMetricsHistory(): Record<string, MetricsSnapshot[]> {
  if (typeof window === "undefined") return {};
  try {
    const raw = window.localStorage.getItem(METRICS_HISTORY_STORAGE_KEY);
    if (!raw) return {};
    const parsed = JSON.parse(raw) as Record<string, MetricsSnapshot[]>;
    return parsed && typeof parsed === "object" ? parsed : {};
  } catch {
    return {};
  }
}

function writeStoredMetricsHistory(historyByCluster: Record<string, MetricsSnapshot[]>) {
  if (typeof window === "undefined") return;
  try {
    window.localStorage.setItem(METRICS_HISTORY_STORAGE_KEY, JSON.stringify(historyByCluster));
  } catch {
    // Ignore quota/security failures; in-memory state continues to work.
  }
}

function readStoredHistoryForCluster(clusterId: string): MetricsSnapshot[] {
  const all = readStoredMetricsHistory();
  return all[clusterId] || [];
}

function persistHistoryForCluster(clusterId: string, history: MetricsSnapshot[]) {
  const all = readStoredMetricsHistory();
  all[clusterId] = history;
  writeStoredMetricsHistory(all);
}

function historySignature(snap: MetricsSnapshot): string {
  const resultEntries = Object.entries(snap.results || {})
    .sort(([a], [b]) => a.localeCompare(b))
    .map(([key, result]) => `${key}:${result.FinalConsistent ? "1" : "0"}:${result.PeakDivergence}:${result.AreaUnderDivergence}`)
    .join("|");

  const tracked = [...(snap.trackedKeys || [])].sort().join(",");
  const stoppedAt = snap.stoppedAt || "active";
  return `${snap.clusterId}|${snap.startedAt}|${stoppedAt}|${tracked}|${resultEntries}`;
}

function mergeHistorySnapshots(preferred: MetricsSnapshot[], incoming: MetricsSnapshot[]): MetricsSnapshot[] {
  const merged: MetricsSnapshot[] = [];
  const seen = new Set<string>();

  for (const snap of [...preferred, ...incoming]) {
    const sig = historySignature(snap);
    if (seen.has(sig)) continue;
    seen.add(sig);
    merged.push(snap);
  }

  return merged.slice(0, 100);
}

function hasConvergenceEdge(prev: MetricsSnapshot | null, next: MetricsSnapshot): boolean {
  if (!prev || !prev.results) {
    return false;
  }

  const prevResults = prev.results || {};
  const nextResults = next.results || {};
  const keys = new Set<string>([...Object.keys(prevResults), ...Object.keys(nextResults)]);

  for (const key of keys) {
    const prevFinal = prevResults[key]?.FinalConsistent;
    const nextFinal = nextResults[key]?.FinalConsistent;

    // Record an event only when a known key flips between converged and divergent.
    if (typeof prevFinal === "boolean" && typeof nextFinal === "boolean" && prevFinal !== nextFinal) {
      return true;
    }
  }

  return false;
}

function appendEventHistoryEntry(clusterId: string, currentHistory: MetricsSnapshot[], snap: MetricsSnapshot): MetricsSnapshot[] {
  // Keep newest first and bound memory growth.
  const entry: MetricsSnapshot = {
    ...snap,
    trackedKeys: [...(snap.trackedKeys || [])],
    results: { ...(snap.results || {}) },
    clusterStats: snap.clusterStats
      ? {
          totalRPCs: snap.clusterStats.totalRPCs,
          totalWrites: snap.clusterStats.totalWrites,
          recentKeys: [...(snap.clusterStats.recentKeys || [])],
        }
      : undefined,
  };

  const nextHistory = [entry, ...currentHistory].slice(0, 100);
  persistHistoryForCluster(clusterId, nextHistory);
  return nextHistory;
}

const ELECTION_TIMEOUT_MIN = 2000;
const ELECTION_TIMEOUT_MAX = 4000;
const HEARTBEAT_INTERVAL = 800;
const MESSAGE_SPEED = 0.001; // progress per ms

interface ClusterStore {
  clusters: ClusterInfo[];
  localStatuses: Record<string, "active" | "crashed">;
  selectedClusterId: string | null;
  showTimers: boolean;
  isLiveMode: boolean; // TRUE for API, FALSE for pure browser simulation
  kvStore: Record<string, any>; // clusterId -> nodeId -> key -> value

  isSimulationRunning: boolean;
  isPaused: boolean;
  setLiveMode: (live: boolean) => void;
  selectedMessageId: string | null;
  raftState: Record<string, NodeRaftState>;
  messages: SimulatedMessage[];
  lastGossipTime: number;

  fetchClusters: () => Promise<void>;
  setSelectedClusterId: (id: string | null) => void;
  handleCrashNode: (clusterId: string, nodeId: string) => Promise<void>;
  handleRecoverNode: (clusterId: string, nodeId: string) => Promise<void>;
  handleRemoveNode: (clusterId: string, nodeId: string) => Promise<void>;
  handleAddNode: (
    clusterId: string,
    payload: { node_id: string; address: string; port: number },
  ) => Promise<boolean>;
  handleCreateCluster: (id: string, protocol: string) => Promise<boolean>;
  handleSetDropRate: (
    clusterId: string,
    nodeId: string,
    dropRate: number,
  ) => Promise<boolean>;
  handleSetDelay: (
    clusterId: string,
    nodeId: string,
    delayMs: number,
  ) => Promise<boolean>;
  handleSetPartition: (
    clusterId: string,
    nodeId: string,
    peerId: string,
    enabled: boolean,
  ) => Promise<boolean>;
  toggleTimers: () => void;
  toggleControlPlane: () => void;
  getNodeStatus: (node: NodeInfo) => "active" | "crashed";

  toggleSimulation: () => void;
  togglePause: () => void;
  initRaftState: () => void;
  tickSimulation: (deltaMs: number) => void;
  setSelectedMessageId: (id: string | null) => void;

  connectLiveStream: (clusterId: string) => void;
  disconnectLiveStream: () => void;

  handleKVPut: (
    clusterId: string,
    nodeId: string,
    key: string,
    value: string,
  ) => Promise<boolean>;
  handleKVGet: (
    clusterId: string,
    nodeId: string,
    key: string,
  ) => Promise<string | null>;
  handleSwapProtocol: (clusterId: string, protocol: string) => Promise<boolean>;
  initSimulationData: () => void;

  showMetrics: boolean;
  setShowMetrics: (show: boolean) => void;
  metrics: MetricsSnapshot | null;
  metricsHistory: MetricsSnapshot[];
  handleStartMetrics: (clusterId: string, intervalMs: number) => Promise<void>;
  handleStopMetrics: (clusterId: string) => Promise<void>;
  handleWatchKey: (clusterId: string, key: string) => Promise<void>;
  fetchMetricsSnapshot: (clusterId: string) => Promise<void>;
  handleFetchMetricsHistory: (clusterId: string) => Promise<void>;
}

let activeEventSource: EventSource | null = null;

const getRandomTimeout = () =>
  ELECTION_TIMEOUT_MIN +
  Math.random() * (ELECTION_TIMEOUT_MAX - ELECTION_TIMEOUT_MIN);
const nextId = () => Math.random().toString(36).substring(2, 9);

export const useClusterStore = create<ClusterStore>((set, get) => ({
  clusters: [],
  localStatuses: {},
  selectedClusterId: null,
  showTimers: false,
  showControlPlane: true,

  isSimulationRunning: true, // Start running by default
  isPaused: false,
  selectedMessageId: null,
  raftState: {},
  messages: [],
  kvStore: {},
  lastGossipTime: 0,
  isLiveMode: true,
  showMetrics: false,
  metrics: null,
  metricsHistory: [],

  setLiveMode: (live) => set({ isLiveMode: live }),
  setShowMetrics: (show) => set({ showMetrics: show }),

  initSimulationData: () => {
    if (get().clusters.length === 0) {
      const currentSelected = get().selectedClusterId;
      set({
        clusters: [
          {
            id: "sim-cluster",
            protocol: "gossip",
            nodes: [
              {
                id: "node-1",
                address: "local",
                port: 8001,
                status: "active",
                activeProtocolKey: "gossip",
                capabilities: { kvPut: true, kvGet: true, kvDelete: true },
              },
              {
                id: "node-2",
                address: "local",
                port: 8002,
                status: "active",
                activeProtocolKey: "gossip",
                capabilities: { kvPut: true, kvGet: true, kvDelete: true },
              },
              {
                id: "node-3",
                address: "local",
                port: 8003,
                status: "active",
                activeProtocolKey: "gossip",
                capabilities: { kvPut: true, kvGet: true, kvDelete: true },
              },
            ],
          },
        ].sort((a: any, b: any) => a.id.localeCompare(b.id)),
        selectedClusterId: currentSelected || "sim-cluster",
      });
      get().initRaftState();
    }
  },

  fetchClusters: async () => {
    const { isLiveMode } = get();
    if (!isLiveMode) {
      get().initSimulationData();
      return;
    }

    try {
      const res = await fetch(`${API_BASE}/clusters`);
      if (!res.ok) {
        throw new Error(`HTTP error! status: ${res.status}`);
      }

      const data = await res.json();
      set((state) => {
        const isInitialLoad = !state.selectedClusterId;
        const firstId =
          isInitialLoad && data && data.length > 0
            ? data[0].id
            : state.selectedClusterId;

        if (isInitialLoad && firstId) {
          // Only auto-connect on the very first load when no selection exists
          setTimeout(() => get().connectLiveStream(firstId), 0);
        }

        const sortedData = (data || []).sort((a: any, b: any) => a.id.localeCompare(b.id));

        return {
          clusters: sortedData,
          selectedClusterId: firstId,
          isLiveMode: true,
        };
      });
    } catch (err) {
      // SILENTLY fail to simulation mode on network errors or backend down
      set({ isLiveMode: false });
      get().initSimulationData();
    }
  },

  setSelectedClusterId: (id) => {
    get().disconnectLiveStream();
    const cachedHistory = id ? readStoredHistoryForCluster(id) : [];
    set({ selectedClusterId: id, messages: [], metrics: null, metricsHistory: cachedHistory });
    if (id) {
      get().connectLiveStream(id);
      get().handleFetchMetricsHistory(id);
    }
  },

  handleCrashNode: async (clusterId, nodeId) => {
    set((state) => ({
      localStatuses: { ...state.localStatuses, [nodeId]: "crashed" },
      // Remove any in-flight messages from this crashed node
      messages: state.messages.filter(
        (msg) => msg.sourceId !== nodeId && msg.targetId !== nodeId,
      ),
    }));

    if (!get().isLiveMode) return;

    try {
      const res = await fetch(
        `${API_BASE}/clusters/${clusterId}/nodes/${nodeId}/faults/crash`,
        {
          method: "POST",
        },
      );
      if (res.ok) {
        get().fetchClusters();
      }
    } catch (err) {
      console.error(err);
    }
  },

  handleRecoverNode: async (clusterId, nodeId) => {
    set((state) => {
      const newRaft = { ...state.raftState };
      newRaft[nodeId] = {
        role: "follower",
        term: newRaft[nodeId]?.term || 0,
        timerProgress: 0,
        votesReceived: new Set(),
        votedFor: null,
        timeoutLimit: getRandomTimeout(),
      };
      return {
        localStatuses: { ...state.localStatuses, [nodeId]: "active" },
        raftState: newRaft,
      };
    });

    if (!get().isLiveMode) return;

    try {
      const res = await fetch(
        `${API_BASE}/clusters/${clusterId}/nodes/${nodeId}/faults/recover`,
        {
          method: "POST",
        },
      );
      if (res.ok) {
        get().fetchClusters();
      }
    } catch (err) {
      console.error(err);
    }
  },

  handleRemoveNode: async (clusterId, nodeId) => {
    if (!get().isLiveMode) {
      set((state) => ({
        clusters: state.clusters.map((c) =>
          c.id === clusterId
            ? { ...c, nodes: c.nodes.filter((n) => n.id !== nodeId) }
            : c,
        ),
      }));
      return;
    }
    try {
      const res = await fetch(
        `${API_BASE}/clusters/${clusterId}/nodes/${nodeId}`,
        {
          method: "DELETE",
        },
      );
      if (res.ok) {
        get().fetchClusters();
      }
    } catch (err) {
      console.error(err);
    }
  },

  handleAddNode: async (clusterId, payload) => {
    if (!get().isLiveMode) {
      set((state) => ({
        clusters: state.clusters.map((c) =>
          c.id === clusterId
            ? {
                ...c,
                nodes: [
                  ...c.nodes,
                  {
                    id: payload.node_id,
                    address: payload.address,
                    port: payload.port,
                    status: "active",
                    activeProtocolKey: c.protocol,
                    capabilities: { kvPut: true, kvGet: true, kvDelete: true },
                  },
                ],
              }
            : c,
        ),
      }));
      get().initRaftState();
      return true;
    }
    try {
      const res = await fetch(`${API_BASE}/clusters/${clusterId}/nodes`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload),
      });
      if (res.ok) {
        get().fetchClusters();
        get().initRaftState();
        return true;
      }
      return false;
    } catch (err) {
      console.error(err);
      return false;
    }
  },

  handleCreateCluster: async (id, protocol) => {
    if (!get().isLiveMode) {
      set((state) => ({
        clusters: [...state.clusters, { id, protocol, nodes: [] }],
        selectedClusterId: id,
      }));
      get().initRaftState();
      return true;
    }
    try {
      const res = await fetch(`${API_BASE}/clusters`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ id, protocol }),
      });
      if (res.ok) {
        get().setSelectedClusterId(id);
        get().fetchClusters();
        get().initRaftState();
        return true;
      }
      return false;
    } catch (err) {
      console.error(err);
      return false;
    }
  },

  handleSetDropRate: async (clusterId, nodeId, dropRate) => {
    if (!get().isLiveMode) {
      set((state) => ({
        clusters: state.clusters.map((c) =>
          c.id === clusterId
            ? {
                ...c,
                nodes: c.nodes.map((n) =>
                  n.id === nodeId
                    ? { ...n, fault: { ...n.fault!, dropRate } }
                    : n,
                ),
              }
            : c,
        ),
      }));
      return true;
    }
    try {
      const res = await fetch(
        `${API_BASE}/clusters/${clusterId}/nodes/${nodeId}/faults/drop-rate`,
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ drop_rate: dropRate }),
        },
      );
      if (!res.ok) return false;
      await get().fetchClusters();
      return true;
    } catch (err) {
      console.error(err);
      return false;
    }
  },

  handleSetDelay: async (clusterId, nodeId, delayMs) => {
    if (!get().isLiveMode) {
      set((state) => ({
        clusters: state.clusters.map((c) =>
          c.id === clusterId
            ? {
                ...c,
                nodes: c.nodes.map((n) =>
                  n.id === nodeId
                    ? { ...n, fault: { ...n.fault!, delayMs } }
                    : n,
                ),
              }
            : c,
        ),
      }));
      return true;
    }
    try {
      const res = await fetch(
        `${API_BASE}/clusters/${clusterId}/nodes/${nodeId}/faults/delay`,
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ delay_ms: delayMs }),
        },
      );
      if (!res.ok) return false;
      await get().fetchClusters();
      return true;
    } catch (err) {
      console.error(err);
      return false;
    }
  },

  handleSetPartition: async (clusterId, nodeId, peerId, enabled) => {
    if (!get().isLiveMode) {
      set((state) => ({
        clusters: state.clusters.map((c) =>
          c.id === clusterId
            ? {
                ...c,
                nodes: c.nodes.map((n) => {
                  // Symmetric update in simulation mode
                  if (n.id === nodeId) {
                    const p = n.fault?.partition || [];
                    const next = enabled
                      ? Array.from(new Set([...p, peerId]))
                      : p.filter((id) => id !== peerId);
                    return {
                      ...n,
                      fault: {
                        ...(n.fault || {
                          crashed: false,
                          dropRate: 0,
                          delayMs: 0,
                        }),
                        partition: next,
                      },
                    };
                  }
                  if (n.id === peerId) {
                    const p = n.fault?.partition || [];
                    const next = enabled
                      ? Array.from(new Set([...p, nodeId]))
                      : p.filter((id) => id !== nodeId);
                    return {
                      ...n,
                      fault: {
                        ...(n.fault || {
                          crashed: false,
                          dropRate: 0,
                          delayMs: 0,
                        }),
                        partition: next,
                      },
                    };
                  }
                  return n;
                }),
              }
            : c,
        ),
      }));
      return true;
    }

    try {
      const res = await fetch(
        `${API_BASE}/clusters/${clusterId}/nodes/${nodeId}/faults/partition`,
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ peer_id: peerId, enabled }),
        },
      );
      if (!res.ok) return false;
      await get().fetchClusters();
      return true;
    } catch (err) {
      console.error(err);
      return false;
    }
  },

  toggleTimers: () => set((state) => ({ showTimers: !state.showTimers })),
  toggleControlPlane: () =>
    set((state) => ({ showControlPlane: !state.showControlPlane })),

  toggleSimulation: () => {
    const running = !get().isSimulationRunning;
    const { selectedClusterId } = get();

    if (running && selectedClusterId) {
      get().connectLiveStream(selectedClusterId);
    } else {
      get().disconnectLiveStream();
    }

    set({ isSimulationRunning: running, messages: [] });
  },

  togglePause: () => set((state) => ({ isPaused: !state.isPaused })),

  setSelectedMessageId: (id) => set({ selectedMessageId: id }),

  initRaftState: () => {
    const { clusters, selectedClusterId } = get();
    const cluster = clusters.find((c) => c.id === selectedClusterId);
    if (!cluster) return;

    const newRaftState: Record<string, NodeRaftState> = {};
    cluster.nodes.forEach((n) => {
      newRaftState[n.id] = {
        role: "follower",
        term: 0,
        timerProgress: 0,
        votesReceived: new Set(),
        votedFor: null,
        timeoutLimit: getRandomTimeout(),
      };
    });
    set({ raftState: newRaftState });
  },

  getNodeStatus: (node) => {
    const { localStatuses } = get();
    return localStatuses[node.id] || node.status || "active";
  },

  connectLiveStream: (clusterId: string) => {
    get().disconnectLiveStream();

    console.log("[SSE] Connecting to logs stream for cluster:", clusterId);
    activeEventSource = new EventSource(
      `${API_BASE}/clusters/${clusterId}/logs`,
    );

    activeEventSource.onopen = () => {
      console.log("[SSE] Connection opened");
    };

    activeEventSource.onmessage = (event) => {
      try {
        // DISCARD messages if tab is hidden to prevent background backlog accumulation
        if (
          typeof document !== "undefined" &&
          document.visibilityState === "hidden"
        ) {
          return;
        }

        const log = JSON.parse(event.data);
        const rawMsg = log.message || log.Message || "";

        if (rawMsg.includes("TRACE:SEND:")) {
          if (get().isPaused) return; // Skip if paused

          const parts = rawMsg.split(":");
          if (parts.length >= 4) {
            const sourceId = parts[2]?.trim();
            const targetId = parts[3]?.trim();
            const msgType = parts[4]?.trim() || "heartbeat";
            const metadata = parts[5]?.trim();
            const sizeBytes = parseInt(parts[6]?.trim() || "0", 10);

            // Check if source node is crashed - don't show messages from dead nodes
            const { localStatuses, clusters } = get();
            if (sourceId !== "CP") {
              const cluster = clusters.find((c) => c.id === clusterId);
              const sourceNode = cluster?.nodes?.find((n) => n.id === sourceId);
              const sourceStatus =
                localStatuses[sourceId] || sourceNode?.status || "active";

              if (sourceStatus === "crashed") {
                return;
              }
            }

            const msg: SimulatedMessage = {
              id: nextId(),
              sourceId,
              targetId,
              type: msgType,
              term: 0,
              progress: 0,
              metadata,
              sizeBytes,
              timestampMs:
                (log.timestamp || log.Timestamp || Date.now() / 1000) * 1000,
            };

            set((state) => {
              // Safety limit: only keep the last 200 messages in the store
              // This handles cases where the tab is throttled but not fully hidden
              const MAX_MESSAGES = 200;
              const newMessages = [...state.messages, msg];
              return {
                messages:
                  newMessages.length > MAX_MESSAGES
                    ? newMessages.slice(-MAX_MESSAGES)
                    : newMessages,
              };
            });
          }
        }
      } catch (e) {
        console.error("[SSE] Failed to parse payload", e);
      }
    };

    activeEventSource.onerror = (e) => {
      // EventSource errors are often just connection drops/reloads
      if (activeEventSource?.readyState !== EventSource.CONNECTING) {
        console.error("[SSE] Stream error", e);
      }
      get().disconnectLiveStream();
    };
  },

  disconnectLiveStream: () => {
    if (activeEventSource) {
      activeEventSource.close();
      activeEventSource = null;
    }
  },

  tickSimulation: (deltaMs: number) => {
    const state = get();
    if (!state.isSimulationRunning || state.isPaused) return;

    // 1. Advance in-flight messages
    const newMessages = [...state.messages];
    const inFlightMessages: SimulatedMessage[] = [];

    newMessages.forEach((msg) => {
      msg.progress += deltaMs * MESSAGE_SPEED;
      if (msg.progress < 1) {
        inFlightMessages.push(msg);
      }
    });

    // 2. Background Gossip heartbeats (every 3 seconds)
    let nextGossipTime = state.lastGossipTime;
    if (!state.isLiveMode) {
      const cluster = state.clusters.find(
        (c) => c.id === state.selectedClusterId,
      );
      if (cluster && cluster.protocol === "gossip") {
        const GOSSIP_TICK_INTERVAL = 3000;
        const now = Date.now();

        if (now - state.lastGossipTime > GOSSIP_TICK_INTERVAL) {
          nextGossipTime = now;
          const activeNodes = cluster.nodes.filter(
            (n) => n.status !== "crashed",
          );

          activeNodes.forEach((node) => {
            const peers = activeNodes.filter((n) => n.id !== node.id);
            if (peers.length > 0) {
              const target = peers[Math.floor(Math.random() * peers.length)];
              const digestMsg: SimulatedMessage = {
                id: nextId(),
                sourceId: node.id,
                targetId: target.id,
                type: "GOSSIP_DIGEST",
                progress: 0,
                term: 0,
                metadata: `DIGEST (Periodic)`,
                timestampMs: now,
              };
              inFlightMessages.push(digestMsg);
            }
          });
        }
      }
    }

    set({ messages: inFlightMessages, lastGossipTime: nextGossipTime });
  },

  handleKVPut: async (
    clusterId: string,
    nodeId: string,
    key: string,
    value: string,
  ) => {
    if (!get().isLiveMode) {
      // Simulation Logic
      set((state) => {
        const clusterKV: any = state.kvStore[clusterId] || {};
        const nodeKV: any = clusterKV[nodeId] || {};
        const updatedNodeKV = { ...nodeKV, [key]: value };
        const updatedClusterKV = { ...clusterKV, [nodeId]: updatedNodeKV };

        return {
          kvStore: {
            ...state.kvStore,
            [clusterId]: updatedClusterKV,
          },
        };
      });

      // 1. CP -> Node message
      const cpMsg: SimulatedMessage = {
        id: nextId(),
        sourceId: "CP",
        targetId: nodeId,
        type: "CP_KV_PUT",
        progress: 0,
        term: 0,
        metadata: `PUT ${key}=${value}`,
        timestampMs: Date.now(),
      };

      set((state) => ({ messages: [...state.messages, cpMsg] }));

      // 2. Gossip propagation if applicable
      const cluster = get().clusters.find((c) => c.id === clusterId);
      if (cluster && cluster.protocol === "gossip") {
        const peers = cluster.nodes.filter(
          (n) => n.id !== nodeId && n.status !== "crashed",
        );
        if (peers.length > 0) {
          // Send to 1-2 random peers
          const gossipTargets = peers
            .sort(() => 0.5 - Math.random())
            .slice(0, 2);

          setTimeout(() => {
            gossipTargets.forEach((target) => {
              const digestMsg: SimulatedMessage = {
                id: nextId(),
                sourceId: nodeId,
                targetId: target.id,
                type: "GOSSIP_DIGEST",
                progress: 0,
                term: 0,
                metadata: `DIGEST [${key}]`,
                timestampMs: Date.now(),
              };
              set((state) => ({ messages: [...state.messages, digestMsg] }));

              // Simulate returning sync/state after some delay
              setTimeout(() => {
                const stateMsg: SimulatedMessage = {
                  id: nextId(),
                  sourceId: nodeId,
                  targetId: target.id,
                  type: "GOSSIP_STATE",
                  progress: 0,
                  term: 0,
                  metadata: `STATE [${key}=${value}]`,
                  timestampMs: Date.now(),
                };
                set((state) => ({ messages: [...state.messages, stateMsg] }));
              }, 1200);
            });
          }, 800);
        }
      }

      return true;
    }

    try {
      const res = await fetch(
        `${API_BASE}/clusters/${clusterId}/nodes/${nodeId}/kv`,
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ key, value }),
        },
      );
      return res.ok;
    } catch (err) {
      console.error(err);
      return false;
    }
  },

  handleKVGet: async (clusterId: string, nodeId: string, key: string) => {
    if (!get().isLiveMode) {
      // 1. CP -> Node message
      const cpMsg: SimulatedMessage = {
        id: nextId(),
        sourceId: "CP",
        targetId: nodeId,
        type: "CP_KV_GET",
        progress: 0,
        term: 0,
        metadata: `GET ${key}`,
        timestampMs: Date.now(),
      };
      set((state) => ({ messages: [...state.messages, cpMsg] }));

      // 2. Local retrieval
      const clusterKV = get().kvStore[clusterId];
      if (clusterKV && clusterKV[nodeId]) {
        return clusterKV[nodeId][key] || null;
      }
      return null;
    }

    try {
      const res = await fetch(
        `${API_BASE}/clusters/${clusterId}/nodes/${nodeId}/kv/${key}`,
      );
      if (res.ok) {
        const data = await res.json();
        return data.value;
      }
      return null;
    } catch (err) {
      console.error(err);
      return null;
    }
  },
  handleSwapProtocol: async (clusterId: string, protocol: string) => {
    if (!get().isLiveMode) {
      set((state) => ({
        clusters: state.clusters.map((c) =>
          c.id === clusterId
            ? {
                ...c,
                protocol,
                nodes: c.nodes.map((n) => ({
                  ...n,
                  activeProtocolKey: protocol,
                })),
              }
            : c,
        ),
      }));
      return true;
    }
    try {
      const res = await fetch(`${API_BASE}/clusters/${clusterId}/protocol`, {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ protocol }),
      });
      if (res.ok) {
        get().fetchClusters();
        return true;
      }
      return false;
    } catch (err) {
      console.error("Failed to swap protocol:", err);
      return false;
    }
  },

  handleStartMetrics: async (clusterId, intervalMs) => {
    try {
      const res = await fetch(`${API_BASE}/clusters/${clusterId}/metrics/start`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ interval_ms: intervalMs }),
      });
      if (res.ok) {
        const data = await res.json();
        set({ metrics: data });
        await get().handleFetchMetricsHistory(clusterId);
      }
    } catch (err) {
      console.error(err);
    }
  },

  handleStopMetrics: async (clusterId) => {
    try {
      const res = await fetch(`${API_BASE}/clusters/${clusterId}/metrics/stop`, {
        method: "POST",
      });
      if (res.ok) {
        const data = await res.json();
        set({ metrics: data });
        // Refresh history from backend
        get().handleFetchMetricsHistory(clusterId);
      }
    } catch (err) {
      console.error(err);
    }
  },

  handleWatchKey: async (clusterId, key) => {
    try {
      const res = await fetch(`${API_BASE}/clusters/${clusterId}/metrics/watch`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ key }),
      });
      if (res.ok) {
        get().fetchMetricsSnapshot(clusterId);
      }
    } catch (err) {
      console.error(err);
    }
  },

  fetchMetricsSnapshot: async (clusterId) => {
    try {
      const res = await fetch(`${API_BASE}/clusters/${clusterId}/metrics`);
      if (res.ok) {
        const data: MetricsSnapshot = await res.json();
        const prev = get().metrics;
        const edgeTriggered = hasConvergenceEdge(prev, data);

        if (edgeTriggered) {
          const updatedHistory = appendEventHistoryEntry(clusterId, get().metricsHistory, data);
          set({ metrics: data, metricsHistory: updatedHistory });
        } else {
          set({ metrics: data });
          await get().handleFetchMetricsHistory(clusterId);
        }
      }
    } catch (err) {
      console.error(err);
    }
  },

  handleFetchMetricsHistory: async (clusterId) => {
    try {
      const res = await fetch(`${API_BASE}/clusters/${clusterId}/metrics/history`);
      if (res.ok) {
        const data = await res.json();
        // Backend returns oldest first, we want newest first in UI
        const normalized = [...(data || [])].reverse() as MetricsSnapshot[];
        const localCached = readStoredHistoryForCluster(clusterId);
        const current = get().metricsHistory;
        const merged = mergeHistorySnapshots([...current, ...localCached], normalized);

        set({ metricsHistory: merged });
        persistHistoryForCluster(clusterId, merged);
      }
    } catch (err) {
      console.error(err);
    }
  },
}));
