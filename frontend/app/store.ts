import { create } from 'zustand';

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
};

export type ClusterInfo = {
  id: string;
  protocol: string;
  nodes: NodeInfo[];
};

export type Role = 'follower' | 'candidate' | 'leader';

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

const API_BASE = "http://localhost:8080/api";

const ELECTION_TIMEOUT_MIN = 2000;
const ELECTION_TIMEOUT_MAX = 4000;
const HEARTBEAT_INTERVAL = 800;
const MESSAGE_SPEED = 0.001; // progress per ms

interface ClusterStore {
  clusters: ClusterInfo[];
  localStatuses: Record<string, "active" | "crashed">;
  selectedClusterId: string | null;
  showTimers: boolean;

  isSimulationRunning: boolean;
  isPaused: boolean;
  selectedMessageId: string | null;
  raftState: Record<string, NodeRaftState>;
  messages: SimulatedMessage[];

  fetchClusters: () => Promise<void>;
  setSelectedClusterId: (id: string | null) => void;
  handleCrashNode: (clusterId: string, nodeId: string) => Promise<void>;
  handleRecoverNode: (clusterId: string, nodeId: string) => Promise<void>;
  handleRemoveNode: (clusterId: string, nodeId: string) => Promise<void>;
  handleAddNode: (clusterId: string, payload: { node_id: string; address: string; port: number }) => Promise<boolean>;
  handleCreateCluster: (id: string, protocol: string) => Promise<boolean>;
  handleSetDropRate: (clusterId: string, nodeId: string, dropRate: number) => Promise<boolean>;
  handleSetDelay: (clusterId: string, nodeId: string, delayMs: number) => Promise<boolean>;
  handleSetPartition: (clusterId: string, nodeId: string, peerId: string, enabled: boolean) => Promise<boolean>;
  toggleTimers: () => void;
  getNodeStatus: (node: NodeInfo) => "active" | "crashed";

  toggleSimulation: () => void;
  togglePause: () => void;
  initRaftState: () => void;
  tickSimulation: (deltaMs: number) => void;
  setSelectedMessageId: (id: string | null) => void;

  connectLiveStream: (clusterId: string) => void;
  disconnectLiveStream: () => void;

  handleKVPut: (clusterId: string, nodeId: string, key: string, value: string) => Promise<boolean>;
  handleKVGet: (clusterId: string, nodeId: string, key: string) => Promise<string | null>;
}

let activeEventSource: EventSource | null = null;

const getRandomTimeout = () => ELECTION_TIMEOUT_MIN + Math.random() * (ELECTION_TIMEOUT_MAX - ELECTION_TIMEOUT_MIN);
const nextId = () => Math.random().toString(36).substring(2, 9);

export const useClusterStore = create<ClusterStore>((set, get) => ({
  clusters: [],
  localStatuses: {},
  selectedClusterId: null,
  showTimers: false,

  isSimulationRunning: true, // Start running by default
  isPaused: false,
  selectedMessageId: null,
  raftState: {},
  messages: [],

  fetchClusters: async () => {
    try {
      const res = await fetch(`${API_BASE}/clusters`);
      if (res.ok) {
        const data = await res.json();
        set((state) => {
          const firstId = !state.selectedClusterId && data && data.length > 0 ? data[0].id : state.selectedClusterId;
          
          if (firstId && !state.selectedClusterId) {
            // New selection on initial load, connect
            setTimeout(() => get().connectLiveStream(firstId), 0);
          }

          return {
            clusters: data || [],
            selectedClusterId: firstId
          };
        });
      }
    } catch (err) {
      console.error("Failed to fetch clusters:", err);
    }
  },

  setSelectedClusterId: (id) => {
    get().disconnectLiveStream();
    set({ selectedClusterId: id, messages: [] });
    if (id) {
      get().connectLiveStream(id);
    }
  },

  handleCrashNode: async (clusterId, nodeId) => {
    set((state) => ({
      localStatuses: { ...state.localStatuses, [nodeId]: "crashed" },
      // Remove any in-flight messages from this crashed node
      messages: state.messages.filter(msg => msg.sourceId !== nodeId && msg.targetId !== nodeId)
    }));
    try {
      const res = await fetch(`${API_BASE}/clusters/${clusterId}/nodes/${nodeId}/faults/crash`, {
        method: "POST",
      });
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
        role: 'follower',
        term: newRaft[nodeId]?.term || 0,
        timerProgress: 0,
        votesReceived: new Set(),
        votedFor: null,
        timeoutLimit: getRandomTimeout()
      };
      return {
        localStatuses: { ...state.localStatuses, [nodeId]: "active" },
        raftState: newRaft
      };
    });
    try {
      const res = await fetch(`${API_BASE}/clusters/${clusterId}/nodes/${nodeId}/faults/recover`, {
        method: "POST",
      });
      if (res.ok) {
        get().fetchClusters();
      }
    } catch (err) {
      console.error(err);
    }
  },

  handleRemoveNode: async (clusterId, nodeId) => {
    try {
      const res = await fetch(`${API_BASE}/clusters/${clusterId}/nodes/${nodeId}`, {
        method: "DELETE",
      });
      if (res.ok) {
        get().fetchClusters();
      }
    } catch (err) {
      console.error(err);
    }
  },

  handleAddNode: async (clusterId, payload) => {
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
    try {
      const res = await fetch(`${API_BASE}/clusters/${clusterId}/nodes/${nodeId}/faults/drop-rate`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ drop_rate: dropRate }),
      });
      if (!res.ok) return false;
      await get().fetchClusters();
      return true;
    } catch (err) {
      console.error(err);
      return false;
    }
  },

  handleSetDelay: async (clusterId, nodeId, delayMs) => {
    try {
      const res = await fetch(`${API_BASE}/clusters/${clusterId}/nodes/${nodeId}/faults/delay`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ delay_ms: delayMs }),
      });
      if (!res.ok) return false;
      await get().fetchClusters();
      return true;
    } catch (err) {
      console.error(err);
      return false;
    }
  },

  handleSetPartition: async (clusterId, nodeId, peerId, enabled) => {
    try {
      const res = await fetch(`${API_BASE}/clusters/${clusterId}/nodes/${nodeId}/faults/partition`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ peer_id: peerId, enabled }),
      });
      if (!res.ok) return false;
      await get().fetchClusters();
      return true;
    } catch (err) {
      console.error(err);
      return false;
    }
  },

  toggleTimers: () => set((state) => ({ showTimers: !state.showTimers })),

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
    const cluster = clusters.find(c => c.id === selectedClusterId);
    if (!cluster) return;

    const newRaftState: Record<string, NodeRaftState> = {};
    cluster.nodes.forEach(n => {
      newRaftState[n.id] = {
        role: 'follower',
        term: 0,
        timerProgress: 0,
        votesReceived: new Set(),
        votedFor: null,
        timeoutLimit: getRandomTimeout()
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
    activeEventSource = new EventSource(`${API_BASE}/clusters/${clusterId}/logs`);

    activeEventSource.onopen = () => {
      console.log("[SSE] Connection opened");
    };

    activeEventSource.onmessage = (event) => {
      try {
        // DISCARD messages if tab is hidden to prevent background backlog accumulation
        if (typeof document !== 'undefined' && document.visibilityState === 'hidden') {
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
            const msgType = parts[4]?.trim() || 'heartbeat';
            const metadata = parts[5]?.trim();
            const sizeBytes = parseInt(parts[6]?.trim() || '0', 10);
            
            // Check if source node is crashed - don't show messages from dead nodes
            const { localStatuses, clusters } = get();
            if (sourceId !== "CP") {
              const cluster = clusters.find(c => c.id === clusterId);
              const sourceNode = cluster?.nodes.find(n => n.id === sourceId);
              const sourceStatus = localStatuses[sourceId] || sourceNode?.status || "active";
              
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
              timestampMs: (log.timestamp || log.Timestamp || Date.now() / 1000) * 1000
            };

            set((state) => {
              // Safety limit: only keep the last 200 messages in the store
              // This handles cases where the tab is throttled but not fully hidden
              const MAX_MESSAGES = 200;
              const newMessages = [...state.messages, msg];
              return {
                messages: newMessages.length > MAX_MESSAGES ? newMessages.slice(-MAX_MESSAGES) : newMessages
              };
            });
          }
        }
      } catch (e) {
        console.error("[SSE] Failed to parse payload", e);
      }
    };

    activeEventSource.onerror = (e) => {
      console.error("[SSE] Stream error", e);
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

    const newMessages = [...state.messages];
    const inFlightMessages: SimulatedMessage[] = [];

    newMessages.forEach(msg => {
      msg.progress += deltaMs * MESSAGE_SPEED;
      if (msg.progress < 1) {
        inFlightMessages.push(msg);
      }
    });

    set({ messages: inFlightMessages });
  },

  handleKVPut: async (clusterId: string, nodeId: string, key: string, value: string) => {
    try {
      const res = await fetch(`${API_BASE}/clusters/${clusterId}/nodes/${nodeId}/kv`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ key, value }),
      });
      return res.ok;
    } catch (err) {
      console.error(err);
      return false;
    }
  },

  handleKVGet: async (clusterId: string, nodeId: string, key: string) => {
    try {
      const res = await fetch(`${API_BASE}/clusters/${clusterId}/nodes/${nodeId}/kv/${key}`);
      if (res.ok) {
        const data = await res.json();
        return data.value;
      }
      return null;
    } catch (err) {
      console.error(err);
      return null;
    }
  }
}));
