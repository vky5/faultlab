import { Activity } from "lucide-react";

interface MessageInspectorProps {
  selectedMessage: any;
  setSelectedMessageId: (id: string | null) => void;
}

export function MessageInspector({ selectedMessage, setSelectedMessageId }: MessageInspectorProps) {
  if (!selectedMessage) return null;

  return (
    <div className="absolute top-4 right-4 w-72 bg-white/95 dark:bg-slate-900/95 backdrop-blur-xl border border-slate-200 dark:border-slate-800 rounded-2xl shadow-2xl overflow-hidden z-50 pointer-events-auto animate-in fade-in slide-in-from-top-4 duration-300">
      <div className="px-4 py-3 border-b border-slate-100 dark:border-slate-800 flex items-center justify-between bg-slate-50 dark:bg-slate-800/50">
        <div className="flex items-center gap-2">
          <div
            className="w-2 h-2 rounded-full"
            style={{
              backgroundColor: selectedMessage.type.includes("GOSSIP")
                ? "#eab308"
                : selectedMessage.type.includes("CP")
                ? "#3b82f6"
                : "#f43f5e",
            }}
          />
          <span className="text-[10px] font-bold uppercase tracking-wider text-slate-500">
            Message Detail
          </span>
        </div>
        <button
          onClick={() => setSelectedMessageId(null)}
          className="p-1 hover:bg-slate-200 dark:hover:bg-slate-700 rounded-full transition-colors"
        >
          <div className="w-4 h-4 text-slate-400">✕</div>
        </button>
      </div>

      <div className="p-4 space-y-5">
        <div className="grid grid-cols-2 gap-3">
          <div className="p-2.5 bg-slate-50 dark:bg-slate-800/50 rounded-xl border border-slate-100 dark:border-slate-800 flex items-center gap-3">
            <div className="w-8 h-8 rounded-lg bg-indigo-500/10 flex items-center justify-center text-indigo-600 dark:text-indigo-400">
              <Activity className="w-4 h-4" />
            </div>
            <div>
              <div className="text-[8px] font-bold text-slate-400 uppercase tracking-tighter">
                Timing
              </div>
              <div className="text-[11px] font-mono font-bold text-slate-700 dark:text-slate-200">
                T + {selectedMessage.timestampMs || "0"}
              </div>
            </div>
          </div>

          <div className="p-2.5 bg-slate-50 dark:bg-slate-800/50 rounded-xl border border-slate-100 dark:border-slate-800 flex items-center gap-3">
            <div className="w-8 h-8 rounded-lg bg-emerald-500/10 flex items-center justify-center text-emerald-600 dark:text-emerald-400">
              <div className="text-[10px] font-bold underline decoration-2">KB</div>
            </div>
            <div>
              <div className="text-[8px] font-bold text-slate-400 uppercase tracking-tighter">
                Transit
              </div>
              <div className="text-[11px] font-mono font-bold text-slate-700 dark:text-slate-200">
                {selectedMessage.sizeBytes
                  ? selectedMessage.sizeBytes > 1024
                    ? (selectedMessage.sizeBytes / 1024).toFixed(1) + " KB"
                    : selectedMessage.sizeBytes + " B"
                  : "64 B"}
              </div>
            </div>
          </div>
        </div>

        <div>
          <div className="text-[9px] font-bold text-slate-400 uppercase mb-2 tracking-widest">
            Protocol Type
          </div>
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
                {selectedMessage.metadata.replace(/_/g, " ")}
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
    </div>
  );
}
