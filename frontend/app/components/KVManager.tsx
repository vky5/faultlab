import React, { useState } from "react";
import { Database, Send, Search, Loader2 } from "lucide-react";
import { useClusterStore } from "../store";

interface KVManagerProps {
  clusterId: string;
  nodeId: string;
}

export function KVManager({ clusterId, nodeId }: KVManagerProps) {
  const { handleKVPut, handleKVGet } = useClusterStore();
  const [key, setKey] = useState("");
  const [value, setValue] = useState("");
  const [loading, setLoading] = useState(false);
  const [lastValue, setLastValue] = useState<string | null>(null);

  const onPut = async () => {
    if (!key || !value) return;
    setLoading(true);
    const success = await handleKVPut(clusterId, nodeId, key, value);
    setLoading(false);
    if (success) {
      setLastValue(value);
      // Optional: show toast
    }
  };

  const onGet = async () => {
    if (!key) return;
    setLoading(true);
    const val = await handleKVGet(clusterId, nodeId, key);
    setLoading(false);
    setLastValue(val);
  };

  return (
    <div className="p-4 rounded-xl bg-slate-50 dark:bg-slate-900/50 border border-slate-200 dark:border-slate-800 space-y-4">
      <div className="flex items-center gap-2 mb-2">
        <Database className="w-4 h-4 text-primary" />
        <h4 className="text-xs font-bold uppercase tracking-wider text-slate-500">KV State Storage</h4>
      </div>

      <div className="grid grid-cols-2 gap-3">
        <div className="space-y-1.5">
          <label className="text-[10px] font-medium text-slate-400 uppercase ml-1">Key Path</label>
          <div className="relative">
            <input
              type="text"
              value={key}
              onChange={(e) => setKey(e.target.value)}
              placeholder="e.g. cluster.config"
              className="w-full bg-white dark:bg-slate-950 border border-slate-200 dark:border-slate-800 rounded-lg px-3 py-2 text-sm focus:ring-2 focus:ring-primary/20 outline-none transition-all pr-10"
            />
            <Search className="absolute right-3 top-2.5 w-4 h-4 text-slate-400 pointer-events-none" />
          </div>
        </div>

        <div className="space-y-1.5">
          <label className="text-[10px] font-medium text-slate-400 uppercase ml-1">Payload Value</label>
          <input
            type="text"
            value={value}
            onChange={(e) => setValue(e.target.value)}
            placeholder="Data string..."
            className="w-full bg-white dark:bg-slate-950 border border-slate-200 dark:border-slate-800 rounded-lg px-3 py-2 text-sm focus:ring-2 focus:ring-primary/20 outline-none transition-all"
          />
        </div>
      </div>

      <div className="flex gap-2">
        <button
          onClick={onPut}
          disabled={loading || !key || !value}
          className="flex-1 bg-primary text-primary-foreground h-10 rounded-lg text-sm font-semibold flex items-center justify-center gap-2 hover:opacity-90 transition-opacity disabled:opacity-50"
        >
          {loading ? <Loader2 className="w-4 h-4 animate-spin" /> : <Send className="w-4 h-4" />}
          Broadcast Update
        </button>
        <button
          onClick={onGet}
          disabled={loading || !key}
          className="px-4 border border-slate-200 dark:border-slate-800 h-10 rounded-lg text-sm font-semibold hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors disabled:opacity-50"
        >
          Inspect
        </button>
      </div>

      {lastValue !== null && (
        <div className="mt-4 p-3 rounded-lg bg-white dark:bg-slate-950 border border-slate-200 dark:border-slate-800">
          <div className="text-[10px] font-medium text-slate-400 uppercase mb-1">Last Resolved State</div>
          <div className="font-mono text-sm text-primary break-all">
            {lastValue || <span className="text-slate-500 italic">Empty/Not set</span>}
          </div>
        </div>
      )}

      <p className="text-[10px] text-slate-500 flex items-center gap-1.5 px-1">
        <span className="w-1 h-1 rounded-full bg-amber-500" />
        Note: Updates will propagate via Gossip protocol to all active peers.
      </p>
    </div>
  );
}
